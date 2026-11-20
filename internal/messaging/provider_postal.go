package messaging

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
)

// Postal (https://postalserver.io), the open-source self-hosted mail server.
// Raw MIME through its HTTP API, so our Message-ID and headers are kept and
// Postal's webhooks match by Message-ID. Same client as
// backend-nestjs/src/modules/delivery/postal.ts.

var postalAuthCodes = map[string]bool{
	"InvalidServerAPIKey": true,
	"AccessDenied":        true,
	"ServerSuspended":     true,
}

func postalBase(c Credentials, production bool) (string, error) {
	u, err := url.Parse(strings.TrimSpace(c["serverUrl"]))
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") {
		return "", Fail("NOT_CONFIGURED", "Postal: the server address is not an http(s) URL.", true)
	}
	if u.Scheme == "http" && production && os.Getenv("POSTAL_ALLOW_HTTP") != "true" {
		return "", Fail("NOT_CONFIGURED", "Postal: use an https:// server address.", true)
	}
	return u.Scheme + "://" + u.Host + strings.TrimRight(u.Path, "/"), nil
}

// sendPostal posts raw MIME to /api/v1/send/raw and returns the Message-ID as `<id>`.
func sendPostal(ctx context.Context, client *http.Client, cfg *Config, c Credentials, envl Envelope, raw []byte) (string, error) {
	base, err := postalBase(c, cfg.Production)
	if err != nil {
		return "", err
	}
	body, _ := json.Marshal(map[string]any{
		"mail_from": envl.From,
		"rcpt_to":   []string{envl.To},
		"data":      base64.StdEncoding.EncodeToString(raw),
		"bounce":    false,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/api/v1/send/raw", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Server-API-Key", c["apiKey"])
	res, err := client.Do(req)
	if err != nil {
		return "", Fail("TEMPORARY", "Postal is unreachable: "+err.Error(), false)
	}
	defer res.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode == 429 || res.StatusCode >= 500 {
		return "", Fail("TEMPORARY", fmt.Sprintf("Postal is busy (%d).", res.StatusCode), false)
	}
	var out struct {
		Status string `json:"status"`
		Data   struct {
			Code      string `json:"code"`
			Message   string `json:"message"`
			MessageID string `json:"message_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return "", Fail("NOT_POSTAL", fmt.Sprintf("Postal answered %d without JSON.", res.StatusCode), false)
	}
	if out.Status == "success" {
		id := strings.Trim(out.Data.MessageID, "<>")
		if id == "" {
			return envl.MessageID, nil
		}
		return "<" + id + ">", nil
	}
	code := out.Data.Code
	if code == "" {
		code = "Error"
	}
	msg := "Postal: " + out.Data.Message
	if out.Data.Message == "" {
		msg = "Postal: " + code
	}
	switch {
	case postalAuthCodes[code]:
		return "", Fail("AUTH_FAILED", msg, false)
	case code == "UnauthenticatedFromAddress":
		return "", Fail("SENDER_NOT_VERIFIED", msg, true)
	case strings.Contains(code, "Limit") || strings.Contains(code, "Throttl"):
		return "", Fail("TEMPORARY", msg, false)
	}
	return "", Fail("REJECTED", msg, true)
}
