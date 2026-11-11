package messaging

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Credentials is the decrypted credential bag of a channel account (string values only).
type Credentials map[string]string

// Account is a connected channel account with readable credentials.
type Account struct {
	ID          string
	Provider    string // provider key: smtp, ses, gmail, outlook, twilio, meta_whatsapp, telegram
	Label       string // what the UI shows, e.g. "Amazon SES"
	Credentials Credentials
}

// providerKey maps the stored provider (label or key) to its key, like providerOf in NestJS.
func providerKey(stored string) string {
	switch strings.ToLower(strings.TrimSpace(stored)) {
	case "smtp":
		return "smtp"
	case "ses", "amazon ses":
		return "ses"
	case "gmail":
		return "gmail"
	case "outlook":
		return "outlook"
	case "twilio":
		return "twilio"
	case "meta_whatsapp", "meta cloud api":
		return "meta_whatsapp"
	case "telegram":
		return "telegram"
	}
	return ""
}

// Sender calls providers. Safe for concurrent use.
type Sender struct {
	cfg    *Config
	client *http.Client
	tokens *TokenSource
	now    func() time.Time
}

func NewSender(cfg *Config) *Sender {
	client := &http.Client{Timeout: 20 * time.Second}
	return &Sender{cfg: cfg, client: client, tokens: NewTokenSource(cfg, client), now: time.Now}
}

func decode[T any](raw json.RawMessage) (T, error) {
	var v T
	if err := json.Unmarshal(raw, &v); err != nil {
		return v, Fail("BAD_JOB", "Unreadable job payload: "+err.Error(), true)
	}
	return v, nil
}

// Send delivers one job through the account. Errors are *Failure.
func (s *Sender) Send(ctx context.Context, job Job, acc *Account) (Result, error) {
	switch job.Kind {
	case KindEmail:
		r, err := decode[EmailRequest](job.Payload)
		if err != nil {
			return Result{}, err
		}
		return s.sendEmail(ctx, acc, r)
	case KindSMS:
		r, err := decode[SMSRequest](job.Payload)
		if err != nil {
			return Result{}, err
		}
		if acc.Provider != "twilio" {
			return Result{}, Fail("WRONG_PROVIDER", acc.Label+" cannot send SMS.", true)
		}
		return sendTwilio(ctx, s.client, s.cfg, acc.Credentials, r)
	case KindWhatsAppTemplate:
		r, err := decode[WhatsAppTemplateRequest](job.Payload)
		if err != nil {
			return Result{}, err
		}
		return sendWhatsAppTemplate(ctx, s.client, s.cfg, acc.Credentials, r)
	case KindWhatsAppText:
		r, err := decode[WhatsAppTextRequest](job.Payload)
		if err != nil {
			return Result{}, err
		}
		return sendWhatsAppText(ctx, s.client, s.cfg, acc.Credentials, r)
	case KindTelegram:
		r, err := decode[TelegramRequest](job.Payload)
		if err != nil {
			return Result{}, err
		}
		return sendTelegram(ctx, s.client, s.cfg, acc.Credentials, r)
	}
	return Result{}, Fail("BAD_JOB", "Unknown job kind "+job.Kind, true)
}

func (s *Sender) sendEmail(ctx context.Context, acc *Account, r EmailRequest) (Result, error) {
	c := acc.Credentials
	// Mailbox providers send only as the connected address (NestJS already set From to it).
	raw, envl, err := BuildMIME(r, s.now())
	if err != nil {
		return Result{}, err
	}
	switch acc.Provider {
	case "ses":
		id, err := sendSES(ctx, s.client, s.cfg, c, raw)
		if err != nil {
			return Result{}, err
		}
		return Result{ProviderMessageID: id}, nil
	case "smtp":
		port, _ := strconv.Atoi(c["port"])
		if port == 0 {
			port = 587
		}
		implicit := port == 465
		if c["secure"] != "" {
			implicit = c["secure"] == "true"
		}
		srv := smtpServer{host: c["host"], port: port, implicit: implicit, starttls: port == 587}
		if c["username"] != "" {
			srv.auth = chooseAuth{user: c["username"], pass: c["password"]}
		}
		if err := sendSMTP(ctx, srv, envl, raw); err != nil {
			return Result{}, err
		}
		return Result{ProviderMessageID: envl.MessageID}, nil
	case "gmail", "outlook":
		token, err := s.tokens.Token(ctx, acc.ID, acc.Provider, c["refreshToken"])
		if err != nil {
			return Result{}, err
		}
		srv := smtpServer{host: "smtp.gmail.com", port: 465, implicit: true, auth: xoauth2{user: c["email"], token: token}}
		if acc.Provider == "outlook" {
			srv = smtpServer{host: "smtp.office365.com", port: 587, starttls: true, auth: xoauth2{user: c["email"], token: token}}
		}
		if err := sendSMTP(ctx, srv, envl, raw); err != nil {
			if f, ok := err.(*Failure); ok && f.Code == "AUTH_FAILED" {
				s.tokens.Forget(acc.ID) // the next attempt fetches a new token
			}
			return Result{}, err
		}
		return Result{ProviderMessageID: envl.MessageID}, nil
	}
	return Result{}, Fail("WRONG_PROVIDER", acc.Label+" cannot send email.", true)
}
