// Package messaging is the delivery engine: it sends what NestJS decided to send.
//
// NestJS renders each message (content, consent, plan limits) and writes one row
// to delivery_jobs with a SendRequest in `payload`. This package claims those
// jobs, calls the provider (SMTP, Amazon SES, Gmail, Outlook, Twilio, Meta
// WhatsApp, Telegram), retries transient errors and records the outcome, which
// NestJS applies to the message log, inbox and campaign stats.
//
// The payload shapes mirror `SendRequest` in
// backend-nestjs/src/modules/delivery/delivery-jobs.ts: change both together.
package messaging

import (
	"encoding/json"
	"time"
)

// Job kinds (delivery_jobs.kind).
const (
	KindEmail            = "email"
	KindSMS              = "sms"
	KindWhatsAppTemplate = "whatsapp_template"
	KindWhatsAppText     = "whatsapp_text"
	KindTelegram         = "telegram"
)

// Job is one claimed delivery_jobs row.
type Job struct {
	ID         string
	TenantID   string
	MessageID  string
	CampaignID *string
	AccountID  string
	Channel    string
	Kind       string
	Payload    json.RawMessage
	// Attempts includes the current one.
	Attempts int
}

// EmailRequest is a fully rendered email.
type EmailRequest struct {
	From    string            `json:"from"`
	ReplyTo string            `json:"replyTo,omitempty"`
	To      string            `json:"to"`
	Subject string            `json:"subject"`
	HTML    string            `json:"html"`
	Text    string            `json:"text"`
	Headers map[string]string `json:"headers"`
}

// SMSRequest is a Twilio text message.
type SMSRequest struct {
	To             string `json:"to"`
	Body           string `json:"body"`
	StatusCallback string `json:"statusCallback,omitempty"`
}

// WhatsAppTemplateRequest is an approved WhatsApp template with its parameters.
type WhatsAppTemplateRequest struct {
	To          string   `json:"to"`
	Name        string   `json:"name"`
	Language    string   `json:"language"`
	BodyParams  []string `json:"bodyParams"`
	HeaderParam *struct {
		Type  string `json:"type"`
		Value string `json:"value"`
	} `json:"headerParam"`
}

// WhatsAppTextRequest is free-form text inside the 24-hour window (inbox replies).
type WhatsAppTextRequest struct {
	To   string `json:"to"`
	Body string `json:"body"`
}

// TelegramRequest is a Bot API text message.
type TelegramRequest struct {
	ChatID string `json:"chatId"`
	Body   string `json:"body"`
}

// Result is what a provider accepted (delivery_jobs.result on success).
type Result struct {
	ProviderMessageID string `json:"providerMessageId"`
	Segments          int    `json:"segments,omitempty"`
}

// Failure is a provider or engine refusal (delivery_jobs.result on failure).
type Failure struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Permanent bool   `json:"-"`
}

func (f *Failure) Error() string { return f.Code + ": " + f.Message }

// Fail builds a Failure.
func Fail(code, message string, permanent bool) *Failure {
	return &Failure{Code: code, Message: message, Permanent: permanent}
}

// HistoryEntry is appended to delivery_jobs.history and copied into the message's events.
type HistoryEntry struct {
	Status string    `json:"status"`
	At     time.Time `json:"at"`
	Detail string    `json:"detail"`
}
