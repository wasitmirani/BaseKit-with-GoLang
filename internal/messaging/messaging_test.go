package messaging

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/httptest"
	"net/mail"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// Sealed by backend-nestjs/src/common/crypto/secret-box.ts (seal()).
func TestSecretBoxOpensNestValues(t *testing.T) {
	box, _ := NewSecretBox("test-key-0123456789-0123456789-abcdef", "", true)
	var c Credentials
	if err := box.Open("v1:15_hgjBqtqYvksfq:saTk1FYn_76MI4pygaMz0Q:-M0Gu93SllOF0Plr26oSm5vZGY7ax6IDmuiDBDjP-WJMebVfRKEN_FeoiQe_dmnSAQ", &c); err != nil {
		t.Fatal(err)
	}
	if c["host"] != "smtp.acme.test" || c["password"] != "p@ss:wörd" {
		t.Fatalf("got %v", c)
	}

	// Development key derived from JWT_SECRET, like NestJS without SECRETS_ENCRYPTION_KEY.
	dev, _ := NewSecretBox("", "dev-jwt", false)
	var d Credentials
	if err := dev.Open("v1:nO5LyJzwcvchU6Vw:rZ8Jv2p6yduaWwgmSAcWMA:MNzlaa8yzXbzoyxYdNEIqS0OxzoA_g", &d); err != nil || d["botToken"] != "123:ABC" {
		t.Fatalf("dev key: %v %v", d, err)
	}

	// Wrong key and tampering are refused.
	other, _ := NewSecretBox("another-key-0123456789-0123456789-xyz", "", true)
	if err := other.Open("v1:15_hgjBqtqYvksfq:saTk1FYn_76MI4pygaMz0Q:-M0Gu93SllOF0Plr26oSm5vZGY7ax6IDmuiDBDjP-WJMebVfRKEN_FeoiQe_dmnSAQ", &c); err == nil {
		t.Fatal("opened with the wrong key")
	}
	if _, err := NewSecretBox("short", "x", true); err == nil {
		t.Fatal("production must require SECRETS_ENCRYPTION_KEY")
	}
}

// AWS's published SigV4 example (IAM ListUsers, "Create a signed AWS API request").
func TestSignV4MatchesAWSExample(t *testing.T) {
	req, _ := http.NewRequest(http.MethodGet, "https://iam.amazonaws.com/?Action=ListUsers&Version=2010-05-08", nil)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=utf-8")
	now, _ := time.Parse("20060102T150405Z", "20150830T123600Z")
	signV4(req, nil, "AKIDEXAMPLE", "wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY", "us-east-1", "iam", now)
	want := "AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20150830/us-east-1/iam/aws4_request, SignedHeaders=content-type;host;x-amz-date, Signature=5d672d79c15b13162d9279b0855cfba6789a8edb4c82c400e06b5924a6f2b5d7"
	if got := req.Header.Get("Authorization"); got != want {
		t.Fatalf("signature\n got %s\nwant %s", got, want)
	}
}

func TestBuildMIME(t *testing.T) {
	raw, env, err := BuildMIME(EmailRequest{
		From:    `"Acme Team" <hello@acme.test>`,
		ReplyTo: "support@acme.test",
		To:      "ada@example.test",
		Subject: "Ünïcode\r\nBcc: victim@evil.test",
		HTML:    "<p>Hi Ada</p>",
		Text:    "Hi Ada",
		Headers: map[string]string{"List-Unsubscribe": "<https://x/unsubscribe/1>", "X-Evil\r\nBcc": "x", "From": "spoof@evil.test"},
	}, time.Date(2026, 11, 11, 9, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if env.From != "hello@acme.test" || env.To != "ada@example.test" || !strings.HasSuffix(env.MessageID, "@acme.test>") {
		t.Fatalf("envelope %+v", env)
	}
	msg, err := mail.ReadMessage(strings.NewReader(string(raw)))
	if err != nil {
		t.Fatal(err)
	}
	if msg.Header.Get("Bcc") != "" || len(msg.Header["From"]) != 1 {
		t.Fatalf("header injection: %v", msg.Header)
	}
	subject, _ := new(mime.WordDecoder).DecodeHeader(msg.Header.Get("Subject"))
	if subject != "Ünïcode  Bcc: victim@evil.test" {
		t.Fatalf("subject %q", subject)
	}
	if msg.Header.Get("List-Unsubscribe") != "<https://x/unsubscribe/1>" || msg.Header.Get("Reply-To") != "<support@acme.test>" {
		t.Fatalf("headers %v", msg.Header)
	}
	_, params, _ := mime.ParseMediaType(msg.Header.Get("Content-Type"))
	parts := multipart.NewReader(msg.Body, params["boundary"])
	var types []string
	for {
		p, err := parts.NextPart() // decodes quoted-printable
		if err != nil {
			break
		}
		body, _ := io.ReadAll(p)
		types = append(types, p.Header.Get("Content-Type")+"="+string(body))
	}
	if strings.Join(types, "|") != "text/plain; charset=utf-8=Hi Ada|text/html; charset=utf-8=<p>Hi Ada</p>" {
		t.Fatalf("parts %v", types)
	}
	if _, _, err := BuildMIME(EmailRequest{From: "not an address", To: "a@b.c"}, time.Now()); err == nil {
		t.Fatal("expected invalid sender")
	}
}

type call struct {
	path, auth, body string
}

func fakeAPI(t *testing.T, handle func(w http.ResponseWriter, c call)) (*Config, *[]call) {
	var mu sync.Mutex
	calls := &[]call{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		c := call{path: r.URL.Path, auth: r.Header.Get("Authorization"), body: string(b)}
		mu.Lock()
		*calls = append(*calls, c)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		handle(w, c)
	}))
	t.Cleanup(srv.Close)
	return &Config{TwilioBase: srv.URL, MetaBase: srv.URL, TelegramBase: srv.URL, SESBase: srv.URL}, calls
}

func job(kind string, payload any) Job {
	b, _ := json.Marshal(payload)
	return Job{ID: "j1", Kind: kind, Payload: b, Attempts: 1}
}

func TestTwilio(t *testing.T) {
	cfg, calls := fakeAPI(t, func(w http.ResponseWriter, c call) {
		if strings.Contains(c.body, "To=%2B15550000000") {
			w.WriteHeader(400)
			w.Write([]byte(`{"code":21211,"message":"The 'To' number is not a valid phone number."}`))
			return
		}
		if strings.Contains(c.body, "To=%2B15559999999") {
			w.WriteHeader(503)
			w.Write([]byte(`{"message":"busy"}`))
			return
		}
		w.WriteHeader(201)
		w.Write([]byte(`{"sid":"SM1","num_segments":"2","status":"queued"}`))
	})
	s := NewSender(cfg)
	acc := &Account{ID: "a", Provider: "twilio", Label: "Twilio", Credentials: Credentials{"accountSid": "AC1", "authToken": "tok", "from": "+15550001111"}}
	r, err := s.Send(context.Background(), job(KindSMS, SMSRequest{To: "+14155550100", Body: "Hi", StatusCallback: "https://api/hooks/twilio/k/status"}), acc)
	if err != nil || r.ProviderMessageID != "SM1" || r.Segments != 2 {
		t.Fatalf("%+v %v", r, err)
	}
	c := (*calls)[0]
	form, _ := url.ParseQuery(c.body)
	if c.path != "/2010-04-01/Accounts/AC1/Messages.json" || form.Get("From") != "+15550001111" || form.Get("StatusCallback") == "" ||
		c.auth != "Basic "+base64.StdEncoding.EncodeToString([]byte("AC1:tok")) {
		t.Fatalf("call %+v", c)
	}
	_, err = s.Send(context.Background(), job(KindSMS, SMSRequest{To: "+15550000000", Body: "x"}), acc)
	if f, _ := err.(*Failure); f == nil || f.Code != "TWILIO_21211" || !f.Permanent {
		t.Fatalf("invalid number: %v", err)
	}
	_, err = s.Send(context.Background(), job(KindSMS, SMSRequest{To: "+15559999999", Body: "x"}), acc)
	if f, _ := err.(*Failure); f == nil || f.Permanent {
		t.Fatalf("503 must be retryable: %v", err)
	}
}

func TestWhatsAppAndTelegram(t *testing.T) {
	cfg, calls := fakeAPI(t, func(w http.ResponseWriter, c call) {
		switch {
		case strings.HasSuffix(c.path, "/messages") && strings.Contains(c.body, `"text"`) && strings.Contains(c.body, "window"):
			w.WriteHeader(400)
			w.Write([]byte(`{"error":{"code":131047,"message":"Re-engagement message"}}`))
		case strings.HasSuffix(c.path, "/messages"):
			w.Write([]byte(`{"messages":[{"id":"wamid.1"}]}`))
		case strings.HasSuffix(c.path, "/sendMessage") && strings.Contains(c.body, "blocked"):
			w.WriteHeader(403)
			w.Write([]byte(`{"ok":false,"error_code":403,"description":"Forbidden: bot was blocked by the user"}`))
		default:
			w.Write([]byte(`{"ok":true,"result":{"message_id":77}}`))
		}
	})
	s := NewSender(cfg)
	wa := &Account{ID: "w", Provider: "meta_whatsapp", Credentials: Credentials{"phoneNumberId": "PN1", "accessToken": "EAA"}}
	tpl := WhatsAppTemplateRequest{To: "+1 (415) 555-0100", Name: "order_update", Language: "en", BodyParams: []string{"Ada", ""}}
	tpl.HeaderParam = &struct {
		Type  string `json:"type"`
		Value string `json:"value"`
	}{"image", "https://img/x.png"}
	r, err := s.Send(context.Background(), job(KindWhatsAppTemplate, tpl), wa)
	if err != nil || r.ProviderMessageID != "wamid.1" {
		t.Fatalf("%v %v", r, err)
	}
	var sent map[string]any
	_ = json.Unmarshal([]byte((*calls)[0].body), &sent)
	if (*calls)[0].path != "/v21.0/PN1/messages" || sent["to"] != "14155550100" || (*calls)[0].auth != "Bearer EAA" {
		t.Fatalf("whatsapp call %+v", (*calls)[0])
	}
	if !strings.Contains((*calls)[0].body, `"text":"-"`) || !strings.Contains((*calls)[0].body, `"link":"https://img/x.png"`) {
		t.Fatalf("template params %s", (*calls)[0].body)
	}
	_, err = s.Send(context.Background(), job(KindWhatsAppText, WhatsAppTextRequest{To: "1", Body: "outside window"}), wa)
	if f, _ := err.(*Failure); f == nil || f.Code != "WINDOW_CLOSED" || !f.Permanent {
		t.Fatalf("window: %v", err)
	}

	tg := &Account{ID: "t", Provider: "telegram", Credentials: Credentials{"botToken": "123:ABC"}}
	r, err = s.Send(context.Background(), job(KindTelegram, TelegramRequest{ChatID: "42", Body: "hi"}), tg)
	if err != nil || r.ProviderMessageID != "77" {
		t.Fatalf("%v %v", r, err)
	}
	_, err = s.Send(context.Background(), job(KindTelegram, TelegramRequest{ChatID: "42", Body: "blocked"}), tg)
	if f, _ := err.(*Failure); f == nil || f.Code != "BLOCKED" || !f.Permanent {
		t.Fatalf("blocked: %v", err)
	}
}

func TestSES(t *testing.T) {
	cfg, calls := fakeAPI(t, func(w http.ResponseWriter, c call) {
		if strings.Contains(c.body, "ConfigurationSetName") {
			w.Header().Set("X-Amzn-Errortype", "MessageRejected:http://internal.amazon.com/")
			w.WriteHeader(400)
			w.Write([]byte(`{"message":"Email address is not verified."}`))
			return
		}
		w.Write([]byte(`{"MessageId":"0100018f-ses"}`))
	})
	s := NewSender(cfg)
	acc := &Account{ID: "s", Provider: "ses", Credentials: Credentials{"region": "eu-west-1", "accessKeyId": "AKID", "secretAccessKey": "secret"}}
	req := EmailRequest{From: "hello@acme.test", To: "ada@example.test", Subject: "Hi", HTML: "<p>x</p>", Text: "x"}
	r, err := s.Send(context.Background(), job(KindEmail, req), acc)
	if err != nil || r.ProviderMessageID != "0100018f-ses" {
		t.Fatalf("%v %v", r, err)
	}
	c := (*calls)[0]
	if c.path != "/v2/email/outbound-emails" || !strings.HasPrefix(c.auth, "AWS4-HMAC-SHA256 Credential=AKID/") || !strings.Contains(c.auth, "/eu-west-1/ses/aws4_request") {
		t.Fatalf("ses call %+v", c)
	}
	var body struct{ Content struct{ Raw struct{ Data string } } }
	_ = json.Unmarshal([]byte(c.body), &body)
	raw, _ := base64.StdEncoding.DecodeString(body.Content.Raw.Data)
	if !strings.Contains(string(raw), "To: <ada@example.test>") {
		t.Fatalf("raw mime %s", raw)
	}
	acc.Credentials["configurationSet"] = "tracking"
	_, err = s.Send(context.Background(), job(KindEmail, req), acc)
	if f, _ := err.(*Failure); f == nil || f.Code != "SENDER_NOT_VERIFIED" || !f.Permanent {
		t.Fatalf("rejected: %v", err)
	}
}

// fakeSMTP accepts AUTH PLAIN mailer/s3cret and answers 550 for rcpt "bounce@...".
func fakeSMTP(t *testing.T) (port int, received *[]string) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	var mu sync.Mutex
	got := &[]string{}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				r := bufio.NewReader(c)
				say := func(s string) { c.Write([]byte(s + "\r\n")) }
				say("220 fake ESMTP")
				for {
					line, err := r.ReadString('\n')
					if err != nil {
						return
					}
					cmd := strings.ToUpper(strings.TrimSpace(line))
					switch {
					case strings.HasPrefix(cmd, "EHLO"):
						say("250-fake")
						say("250 AUTH PLAIN LOGIN")
					case strings.HasPrefix(cmd, "AUTH PLAIN"):
						dec, _ := base64.StdEncoding.DecodeString(strings.TrimSpace(line[len("AUTH PLAIN "):]))
						if string(dec) == "\x00mailer\x00s3cret" {
							say("235 ok")
						} else {
							say("535 5.7.8 bad credentials")
						}
					case strings.HasPrefix(cmd, "MAIL"):
						say("250 ok")
					case strings.HasPrefix(cmd, "RCPT"):
						if strings.Contains(cmd, "BOUNCE@") {
							say("550 5.1.1 no such user")
						} else {
							say("250 ok")
						}
					case cmd == "DATA":
						say("354 go")
						var b strings.Builder
						for {
							l, err := r.ReadString('\n')
							if err != nil || l == ".\r\n" {
								break
							}
							b.WriteString(l)
						}
						mu.Lock()
						*got = append(*got, b.String())
						mu.Unlock()
						say("250 queued")
					case cmd == "QUIT":
						say("221 bye")
						return
					default:
						say("250 ok")
					}
				}
			}(conn)
		}
	}()
	return ln.Addr().(*net.TCPAddr).Port, got
}

func TestSMTP(t *testing.T) {
	port, got := fakeSMTP(t)
	s := NewSender(&Config{})
	acc := &Account{ID: "m", Provider: "smtp", Credentials: Credentials{"host": "127.0.0.1", "port": strconv.Itoa(port), "username": "mailer", "password": "s3cret"}}
	req := EmailRequest{From: "Acme <hello@acme.test>", To: "ada@example.test", Subject: "Hello", HTML: "<p>Hi</p>", Text: "Hi", Headers: map[string]string{"X-LinqFlow-Message-Id": "m1"}}
	r, err := s.Send(context.Background(), job(KindEmail, req), acc)
	if err != nil || !strings.HasSuffix(r.ProviderMessageID, "@acme.test>") {
		t.Fatalf("%v %v", r, err)
	}
	if len(*got) != 1 || !strings.Contains((*got)[0], "X-LinqFlow-Message-Id: m1") {
		t.Fatalf("received %v", *got)
	}

	req.To = "bounce@example.test"
	_, err = s.Send(context.Background(), job(KindEmail, req), acc)
	if f, _ := err.(*Failure); f == nil || f.Code != "REJECTED" || !f.Permanent {
		t.Fatalf("550: %v", err)
	}
	acc.Credentials["password"] = "wrong"
	req.To = "ada@example.test"
	_, err = s.Send(context.Background(), job(KindEmail, req), acc)
	if f, _ := err.(*Failure); f == nil || f.Code != "AUTH_FAILED" {
		t.Fatalf("535: %v", err)
	}
	acc.Credentials["port"] = "1" // nothing listens: retryable
	_, err = s.Send(context.Background(), job(KindEmail, req), acc)
	if f, _ := err.(*Failure); f == nil || f.Code != "TEMPORARY" || f.Permanent {
		t.Fatalf("refused: %v", err)
	}
}

func TestOAuthTokenRefreshAndCache(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		_ = r.ParseForm()
		if r.Form.Get("refresh_token") == "revoked" {
			w.WriteHeader(400)
			w.Write([]byte(`{"error":"invalid_grant","error_description":"Token has been expired or revoked."}`))
			return
		}
		w.Write([]byte(`{"access_token":"ya29.token","expires_in":3600}`))
	}))
	defer srv.Close()
	ts := NewTokenSource(&Config{GoogleTokenURL: srv.URL, GoogleClientID: "id", GoogleClientSecret: "secret"}, srv.Client())
	for i := 0; i < 2; i++ {
		tok, err := ts.Token(context.Background(), "acc", "gmail", "refresh")
		if err != nil || tok != "ya29.token" {
			t.Fatal(tok, err)
		}
	}
	if hits != 1 {
		t.Fatalf("token not cached: %d calls", hits)
	}
	_, err := ts.Token(context.Background(), "acc2", "gmail", "revoked")
	if f, _ := err.(*Failure); f == nil || !f.Permanent {
		t.Fatalf("revoked: %v", err)
	}
	_, err = NewTokenSource(&Config{}, srv.Client()).Token(context.Background(), "a", "outlook", "r")
	if f, _ := err.(*Failure); f == nil || f.Code != "NOT_CONFIGURED" {
		t.Fatalf("unconfigured: %v", err)
	}
}

func TestLimiter(t *testing.T) {
	l := NewLimiter()
	now := time.Now()
	for i := 0; i < 2; i++ {
		if d := l.Reserve("a", 2, now); d != 0 {
			t.Fatalf("burst %d waited %v", i, d)
		}
	}
	if d := l.Reserve("a", 2, now); d != 500*time.Millisecond {
		t.Fatalf("third send waits %v", d)
	}
	if d := l.Reserve("b", 2, now); d != 0 {
		t.Fatal("accounts must not share a bucket")
	}
	if d := l.Reserve("a", 2, now.Add(2*time.Second)); d != 0 {
		t.Fatalf("refill: %v", d)
	}
	if RateFor(&Account{Provider: "ses", Credentials: Credentials{"rateLimitPerSecond": "40"}}) != 40 || RateFor(&Account{Provider: "gmail"}) != 1 {
		t.Fatal("rates")
	}
}
