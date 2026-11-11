package messaging

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/smtp"
	"net/textproto"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const smtpTimeout = 30 * time.Second

// smtpServer is where and how to connect.
type smtpServer struct {
	host     string
	port     int
	implicit bool // TLS from the first byte (465)
	starttls bool // require STARTTLS
	auth     smtp.Auth
}

// sendSMTP delivers one message over a fresh connection.
func sendSMTP(ctx context.Context, s smtpServer, env Envelope, msg []byte) error {
	addr := net.JoinHostPort(s.host, strconv.Itoa(s.port))
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	var conn net.Conn
	var err error
	if s.implicit {
		conn, err = tls.DialWithDialer(dialer, "tcp", addr, &tls.Config{ServerName: s.host, MinVersion: tls.VersionTLS12})
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", addr)
	}
	if err != nil {
		return Fail("TEMPORARY", "Could not connect to "+addr+": "+err.Error(), false)
	}
	deadline := time.Now().Add(smtpTimeout)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	_ = conn.SetDeadline(deadline)
	c, err := smtp.NewClient(conn, s.host)
	if err != nil {
		conn.Close()
		return classifySMTP(err)
	}
	defer c.Close()
	if !s.implicit {
		if ok, _ := c.Extension("STARTTLS"); ok {
			if err := c.StartTLS(&tls.Config{ServerName: s.host, MinVersion: tls.VersionTLS12}); err != nil {
				return classifySMTP(err)
			}
		} else if s.starttls {
			return Fail("TLS_REQUIRED", "The server does not offer STARTTLS on port "+strconv.Itoa(s.port), false)
		}
	}
	if s.auth != nil {
		if err := c.Auth(s.auth); err != nil {
			return classifySMTP(err)
		}
	}
	if err := c.Mail(env.From); err != nil {
		return classifySMTP(err)
	}
	if err := c.Rcpt(env.To); err != nil {
		return classifySMTP(err)
	}
	w, err := c.Data()
	if err != nil {
		return classifySMTP(err)
	}
	if _, err := w.Write(msg); err != nil {
		return classifySMTP(err)
	}
	if err := w.Close(); err != nil {
		return classifySMTP(err)
	}
	_ = c.Quit()
	return nil
}

// classifySMTP mirrors describeSendError in NestJS: 535 auth (retry, the token may
// refresh), 55x rejected (permanent), network trouble (retry).
func classifySMTP(err error) error {
	var f *Failure
	if errors.As(err, &f) {
		return f
	}
	msg := strings.Join(strings.Fields(err.Error()), " ")
	if len(msg) > 300 {
		msg = msg[:300]
	}
	var tp *textproto.Error
	if errors.As(err, &tp) {
		switch {
		case tp.Code == 535 || tp.Code == 534 || tp.Code == 530:
			return Fail("AUTH_FAILED", msg, false)
		case tp.Code >= 550 && tp.Code < 560:
			return Fail("REJECTED", msg, true)
		case tp.Code >= 400 && tp.Code < 500:
			return Fail("TEMPORARY", msg, false)
		}
		return Fail("SEND_FAILED", msg, false)
	}
	if strings.Contains(strings.ToLower(msg), "auth") {
		return Fail("AUTH_FAILED", msg, false)
	}
	return Fail("TEMPORARY", msg, false)
}

// ---------- AUTH mechanisms ----------

// plainAuth is AUTH PLAIN without net/smtp's "TLS or localhost only" rule, which
// would refuse servers reached over STARTTLS-less private networks; NestJS
// (nodemailer) allows the same.
type plainAuth struct{ user, pass string }

func (a plainAuth) Start(*smtp.ServerInfo) (string, []byte, error) {
	return "PLAIN", []byte("\x00" + a.user + "\x00" + a.pass), nil
}
func (a plainAuth) Next([]byte, bool) ([]byte, error) { return nil, nil }

// loginAuth is AUTH LOGIN, for servers that do not offer PLAIN.
type loginAuth struct{ user, pass string }

func (a loginAuth) Start(*smtp.ServerInfo) (string, []byte, error) { return "LOGIN", nil, nil }
func (a loginAuth) Next(from []byte, more bool) ([]byte, error) {
	if !more {
		return nil, nil
	}
	if strings.Contains(strings.ToLower(string(from)), "user") {
		return []byte(a.user), nil
	}
	return []byte(a.pass), nil
}

// chooseAuth picks PLAIN or LOGIN by what the server advertises, after the greeting.
type chooseAuth struct{ user, pass string }

func (a chooseAuth) Start(info *smtp.ServerInfo) (string, []byte, error) {
	for _, m := range info.Auth {
		if strings.EqualFold(m, "PLAIN") {
			return plainAuth(a).Start(info)
		}
	}
	for _, m := range info.Auth {
		if strings.EqualFold(m, "LOGIN") {
			return loginAuth(a).Start(info)
		}
	}
	return plainAuth(a).Start(info)
}
func (a chooseAuth) Next(from []byte, more bool) ([]byte, error) {
	if !more {
		return nil, nil
	}
	return loginAuth(a).Next(from, more)
}

// xoauth2 is SASL XOAUTH2 (Gmail, Outlook).
type xoauth2 struct{ user, token string }

func (a xoauth2) Start(*smtp.ServerInfo) (string, []byte, error) {
	return "XOAUTH2", []byte("user=" + a.user + "\x01auth=Bearer " + a.token + "\x01\x01"), nil
}

// On failure the server sends a JSON challenge; an empty answer ends the exchange.
func (a xoauth2) Next(_ []byte, more bool) ([]byte, error) {
	if more {
		return []byte{}, nil
	}
	return nil, nil
}

// ---------- OAuth access tokens (Gmail, Outlook) ----------

type cachedToken struct {
	token   string
	expires time.Time
}

// TokenSource exchanges stored refresh tokens for access tokens and caches them.
type TokenSource struct {
	cfg    *Config
	client *http.Client
	mu     sync.Mutex
	cache  map[string]cachedToken
}

func NewTokenSource(cfg *Config, client *http.Client) *TokenSource {
	return &TokenSource{cfg: cfg, client: client, cache: map[string]cachedToken{}}
}

// Forget drops a cached token (after the server refused it).
func (t *TokenSource) Forget(accountID string) {
	t.mu.Lock()
	delete(t.cache, accountID)
	t.mu.Unlock()
}

func (t *TokenSource) Token(ctx context.Context, accountID, provider, refreshToken string) (string, error) {
	t.mu.Lock()
	if c, ok := t.cache[accountID]; ok && time.Until(c.expires) > time.Minute {
		t.mu.Unlock()
		return c.token, nil
	}
	t.mu.Unlock()

	tokenURL, id, secret := t.cfg.GoogleTokenURL, t.cfg.GoogleClientID, t.cfg.GoogleClientSecret
	name, prefix := "Gmail", "GOOGLE"
	if provider == "outlook" {
		tokenURL, id, secret = t.cfg.MicrosoftTokenURL, t.cfg.MicrosoftClientID, t.cfg.MicrosoftClientSecret
		name, prefix = "Outlook", "MICROSOFT"
	}
	if id == "" || secret == "" {
		return "", Fail("NOT_CONFIGURED", fmt.Sprintf("%s is not configured on the messaging engine (%s_OAUTH_CLIENT_ID / %s_OAUTH_CLIENT_SECRET).", name, prefix, prefix), false)
	}
	form := url.Values{
		"client_id":     {id},
		"client_secret": {secret},
		"refresh_token": {refreshToken},
		"grant_type":    {"refresh_token"},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := t.client.Do(req)
	if err != nil {
		return "", Fail("TEMPORARY", name+" sign-in: "+err.Error(), false)
	}
	defer res.Body.Close()
	var body struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		Error       string `json:"error"`
		Description string `json:"error_description"`
	}
	_ = json.NewDecoder(res.Body).Decode(&body)
	if res.StatusCode != http.StatusOK || body.AccessToken == "" {
		reason := body.Description
		if reason == "" {
			reason = body.Error
		}
		if body.Error == "invalid_grant" {
			// Revoked or expired consent: reconnecting the account is the only fix.
			return "", Fail("AUTH_FAILED", name+" access was revoked; reconnect the account in Channels → Email. ("+reason+")", true)
		}
		return "", Fail("AUTH_FAILED", name+" sign-in failed: "+reason, res.StatusCode >= 400 && res.StatusCode < 500 && res.StatusCode != 429)
	}
	expires := time.Now().Add(time.Duration(max(body.ExpiresIn, 60)) * time.Second)
	t.mu.Lock()
	t.cache[accountID] = cachedToken{token: body.AccessToken, expires: expires}
	t.mu.Unlock()
	return body.AccessToken, nil
}
