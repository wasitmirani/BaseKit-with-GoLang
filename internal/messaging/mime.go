package messaging

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"mime"
	"mime/quotedprintable"
	"net/mail"
	"sort"
	"strings"
	"time"
)

// Envelope is who the SMTP/SES transaction is from and to.
type Envelope struct {
	From      string
	To        string
	MessageID string
}

// noBreaks strips CR and LF so values cannot inject headers.
func noBreaks(s string) string {
	return strings.NewReplacer("\r", " ", "\n", " ").Replace(s)
}

func parseAddress(label, what string) (*mail.Address, error) {
	a, err := mail.ParseAddress(noBreaks(label))
	if err != nil {
		return nil, Fail("INVALID_ADDRESS", fmt.Sprintf("Invalid %s address %q", what, label), true)
	}
	return a, nil
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// BuildMIME renders a multipart/alternative message (text and HTML parts,
// quoted-printable, UTF-8). Returns the bytes and the envelope.
func BuildMIME(r EmailRequest, now time.Time) ([]byte, Envelope, error) {
	from, err := parseAddress(r.From, "sender")
	if err != nil {
		return nil, Envelope{}, err
	}
	to, err := parseAddress(r.To, "recipient")
	if err != nil {
		return nil, Envelope{}, err
	}
	domain := "localhost"
	if at := strings.LastIndex(from.Address, "@"); at >= 0 {
		domain = from.Address[at+1:]
	}
	messageID := fmt.Sprintf("<%s@%s>", randomHex(16), domain)
	boundary := "lf-" + randomHex(12)

	var b bytes.Buffer
	h := func(k, v string) { fmt.Fprintf(&b, "%s: %s\r\n", k, noBreaks(v)) }
	h("From", from.String())
	h("To", to.String())
	if r.ReplyTo != "" {
		if rt, err := mail.ParseAddress(noBreaks(r.ReplyTo)); err == nil {
			h("Reply-To", rt.String())
		}
	}
	h("Subject", mime.QEncoding.Encode("utf-8", noBreaks(r.Subject)))
	h("Date", now.UTC().Format(time.RFC1123Z))
	h("Message-ID", messageID)
	h("MIME-Version", "1.0")
	// Caller headers (List-Unsubscribe, the product's message-id header), in a stable order.
	keys := make([]string, 0, len(r.Headers))
	for k := range r.Headers {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	reserved := map[string]bool{"from": true, "to": true, "subject": true, "date": true, "message-id": true, "mime-version": true, "content-type": true, "reply-to": true}
	for _, k := range keys {
		name := noBreaks(strings.TrimSpace(k))
		if name == "" || strings.ContainsAny(name, ": ") || reserved[strings.ToLower(name)] {
			continue
		}
		h(name, r.Headers[k])
	}
	h("Content-Type", fmt.Sprintf(`multipart/alternative; boundary="%s"`, boundary))
	b.WriteString("\r\n")

	part := func(contentType, body string) error {
		fmt.Fprintf(&b, "--%s\r\nContent-Type: %s; charset=utf-8\r\nContent-Transfer-Encoding: quoted-printable\r\n\r\n", boundary, contentType)
		w := quotedprintable.NewWriter(&b)
		if _, err := w.Write([]byte(strings.ReplaceAll(body, "\r\n", "\n"))); err != nil {
			return err
		}
		if err := w.Close(); err != nil {
			return err
		}
		b.WriteString("\r\n")
		return nil
	}
	if err := part("text/plain", r.Text); err != nil {
		return nil, Envelope{}, err
	}
	if err := part("text/html", r.HTML); err != nil {
		return nil, Envelope{}, err
	}
	fmt.Fprintf(&b, "--%s--\r\n", boundary)
	return b.Bytes(), Envelope{From: from.Address, To: to.Address, MessageID: messageID}, nil
}
