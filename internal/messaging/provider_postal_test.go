package messaging

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPostal(t *testing.T) {
	var gotKey, gotPath string
	var got struct {
		MailFrom string   `json:"mail_from"`
		RcptTo   []string `json:"rcpt_to"`
		Data     string   `json:"data"`
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotKey, gotPath = r.Header.Get("X-Server-API-Key"), r.URL.Path
		_ = json.Unmarshal(b, &got)
		w.Header().Set("Content-Type", "application/json")
		switch {
		case gotKey == "bad":
			w.Write([]byte(`{"status":"error","data":{"code":"InvalidServerAPIKey","message":"The API key provided was not valid"}}`))
		case strings.Contains(strings.Join(got.RcptTo, ","), "busy"):
			w.WriteHeader(503)
		case got.MailFrom == "nobody@unverified.test":
			w.Write([]byte(`{"status":"error","data":{"code":"UnauthenticatedFromAddress","message":"The From address is not authorised"}}`))
		default:
			w.Write([]byte(`{"status":"success","data":{"message_id":"abc123@postal.acme.test","messages":{"ada@example.test":{"id":7,"token":"t"}}}}`))
		}
	}))
	t.Cleanup(srv.Close)

	s := NewSender(&Config{})
	acc := &Account{ID: "p", Provider: providerKey("Postal"), Label: "Postal", Credentials: Credentials{"serverUrl": srv.URL + "/", "apiKey": "key-1"}}
	req := EmailRequest{From: "hello@acme.test", To: "ada@example.test", Subject: "Hi", HTML: "<p>x</p>", Text: "x"}

	r, err := s.Send(context.Background(), job(KindEmail, req), acc)
	if err != nil || r.ProviderMessageID != "<abc123@postal.acme.test>" {
		t.Fatalf("send: %v %v", r, err)
	}
	if gotPath != "/api/v1/send/raw" || gotKey != "key-1" || got.MailFrom != "hello@acme.test" || len(got.RcptTo) != 1 || got.RcptTo[0] != "ada@example.test" {
		t.Fatalf("request: %s %s %+v", gotPath, gotKey, got)
	}
	raw, _ := base64.StdEncoding.DecodeString(got.Data)
	if !strings.Contains(string(raw), "Subject: Hi") || !strings.Contains(string(raw), "Message-ID: <") {
		t.Fatalf("raw mime %s", raw)
	}

	fail := func(mutate func(), code string, permanent bool) {
		t.Helper()
		creds := Credentials{"serverUrl": srv.URL, "apiKey": "key-1"}
		acc.Credentials = creds
		r := req
		mutateReq = &r
		mutate()
		_, err := s.Send(context.Background(), job(KindEmail, r), acc)
		f, _ := err.(*Failure)
		if f == nil || f.Code != code || f.Permanent != permanent {
			t.Fatalf("want %s permanent=%v, got %v", code, permanent, err)
		}
	}
	fail(func() { acc.Credentials["apiKey"] = "bad" }, "AUTH_FAILED", false)
	fail(func() { mutateReq.To = "busy@example.test" }, "TEMPORARY", false)
	fail(func() { mutateReq.From = "nobody@unverified.test" }, "SENDER_NOT_VERIFIED", true)
	fail(func() { acc.Credentials["serverUrl"] = "ftp://postal.test" }, "NOT_CONFIGURED", true)
}

var mutateReq *EmailRequest
