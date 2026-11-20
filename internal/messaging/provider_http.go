package messaging

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

// Twilio, Meta WhatsApp Cloud API and Telegram Bot API over REST (no SDKs).
// Same requests and error codes as backend-nestjs/src/modules/delivery/messaging-transport.ts.

const metaGraphVersion = "v21.0"

type apiResponse struct {
	status int
	body   map[string]any
}

func callJSON(ctx context.Context, client *http.Client, method, u string, headers map[string]string, body io.Reader) (*apiResponse, error) {
	req, err := http.NewRequestWithContext(ctx, method, u, body)
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, Fail("TEMPORARY", "Network error: "+err.Error(), false)
	}
	defer res.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	out := map[string]any{}
	_ = json.Unmarshal(data, &out)
	return &apiResponse{status: res.StatusCode, body: out}, nil
}

// retryable: 429 and 5xx are worth retrying; other 4xx are not.
func retryable(status int) bool { return status == 429 || status >= 500 }

// str renders a JSON value as text (strings and numbers only).
func str(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case float64:
		if x == float64(int64(x)) {
			return fmt.Sprintf("%d", int64(x))
		}
		return fmt.Sprintf("%v", x)
	case bool:
		return fmt.Sprintf("%v", x)
	}
	return ""
}

func orDefault(s, d string) string {
	if s == "" {
		return d
	}
	return s
}

// ---------- Twilio SMS ----------

func sendTwilio(ctx context.Context, client *http.Client, cfg *Config, c Credentials, r SMSRequest) (Result, error) {
	form := url.Values{"To": {r.To}, "Body": {r.Body}}
	if c["messagingServiceSid"] != "" {
		form.Set("MessagingServiceSid", c["messagingServiceSid"])
	} else {
		form.Set("From", c["from"])
	}
	if r.StatusCallback != "" {
		form.Set("StatusCallback", r.StatusCallback)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		cfg.TwilioBase+"/2010-04-01/Accounts/"+url.PathEscape(c["accountSid"])+"/Messages.json",
		strings.NewReader(form.Encode()))
	if err != nil {
		return Result{}, err
	}
	req.SetBasicAuth(c["accountSid"], c["authToken"])
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := client.Do(req)
	if err != nil {
		return Result{}, Fail("TEMPORARY", "Network error: "+err.Error(), false)
	}
	defer res.Body.Close()
	body := map[string]any{}
	_ = json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&body)
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		code := orDefault(str(body["code"]), fmt.Sprint(res.StatusCode))
		// 21211 invalid number, 21610 unsubscribed (STOP), 21408 region not enabled.
		errCode := "TWILIO_" + code
		if code == "21610" {
			errCode = "UNSUBSCRIBED"
		}
		return Result{}, Fail(errCode, "Twilio: "+orDefault(str(body["message"]), res.Status), !retryable(res.StatusCode))
	}
	segments := 1
	if n, err := fmt.Sscan(str(body["num_segments"]), &segments); n != 1 || err != nil || segments < 1 {
		segments = 1
	}
	return Result{ProviderMessageID: str(body["sid"]), Segments: segments}, nil
}

// ---------- Meta WhatsApp Cloud API ----------

var nonDigits = regexp.MustCompile(`\D`)

func metaPost(ctx context.Context, client *http.Client, cfg *Config, c Credentials, payload map[string]any) (Result, error) {
	payload["messaging_product"] = "whatsapp"
	payload["recipient_type"] = "individual"
	body, _ := json.Marshal(payload)
	res, err := callJSON(ctx, client, http.MethodPost,
		cfg.MetaBase+"/"+metaGraphVersion+"/"+url.PathEscape(c["phoneNumberId"])+"/messages",
		map[string]string{"Authorization": "Bearer " + c["accessToken"], "Content-Type": "application/json"},
		bytes.NewReader(body))
	if err != nil {
		return Result{}, err
	}
	if res.status < 200 || res.status >= 300 {
		e, _ := res.body["error"].(map[string]any)
		code := orDefault(str(e["code"]), fmt.Sprint(res.status))
		detail := str(e["message"])
		if d, ok := e["error_data"].(map[string]any); ok && str(d["details"]) != "" {
			detail = str(d["details"])
		}
		errCode := "META_" + code
		// 131047: outside the 24-hour window; 130429: rate limited.
		if code == "131047" {
			errCode = "WINDOW_CLOSED"
		}
		return Result{}, Fail(errCode, "WhatsApp: "+orDefault(detail, http.StatusText(res.status)), !retryable(res.status) && code != "130429")
	}
	id := ""
	if list, ok := res.body["messages"].([]any); ok && len(list) > 0 {
		if m, ok := list[0].(map[string]any); ok {
			id = str(m["id"])
		}
	}
	return Result{ProviderMessageID: id}, nil
}

func sendWhatsAppTemplate(ctx context.Context, client *http.Client, cfg *Config, c Credentials, r WhatsAppTemplateRequest) (Result, error) {
	var components []map[string]any
	if r.HeaderParam != nil && r.HeaderParam.Value != "" {
		param := map[string]any{"type": "text", "text": r.HeaderParam.Value}
		if r.HeaderParam.Type == "image" {
			param = map[string]any{"type": "image", "image": map[string]string{"link": r.HeaderParam.Value}}
		}
		components = append(components, map[string]any{"type": "header", "parameters": []any{param}})
	}
	if len(r.BodyParams) > 0 {
		params := make([]any, len(r.BodyParams))
		for i, t := range r.BodyParams {
			params[i] = map[string]string{"type": "text", "text": orDefault(t, "-")}
		}
		components = append(components, map[string]any{"type": "body", "parameters": params})
	}
	tpl := map[string]any{"name": r.Name, "language": map[string]string{"code": orDefault(r.Language, "en")}}
	if len(components) > 0 {
		tpl["components"] = components
	}
	return metaPost(ctx, client, cfg, c, map[string]any{
		"to": nonDigits.ReplaceAllString(r.To, ""), "type": "template", "template": tpl,
	})
}

func sendWhatsAppText(ctx context.Context, client *http.Client, cfg *Config, c Credentials, r WhatsAppTextRequest) (Result, error) {
	return metaPost(ctx, client, cfg, c, map[string]any{
		"to": nonDigits.ReplaceAllString(r.To, ""), "type": "text",
		"text": map[string]any{"preview_url": true, "body": r.Body},
	})
}

// ---------- Meta Messenger and Instagram (Page Send API) ----------

func sendPageMessage(ctx context.Context, client *http.Client, cfg *Config, c Credentials, network string, r PageMessageRequest) (Result, error) {
	body, _ := json.Marshal(map[string]any{
		"recipient":      map[string]string{"id": r.RecipientID},
		"messaging_type": "RESPONSE",
		"message":        map[string]string{"text": r.Body},
	})
	res, err := callJSON(ctx, client, http.MethodPost,
		cfg.MetaBase+"/"+metaGraphVersion+"/"+url.PathEscape(c["pageId"])+"/messages",
		map[string]string{"Authorization": "Bearer " + c["pageAccessToken"], "Content-Type": "application/json"},
		bytes.NewReader(body))
	if err != nil {
		return Result{}, err
	}
	if res.status < 200 || res.status >= 300 {
		e, _ := res.body["error"].(map[string]any)
		code := orDefault(str(e["code"]), fmt.Sprint(res.status))
		sub := str(e["error_subcode"])
		errCode := "META_" + code
		// 10 / 2018278: outside the messaging window; 551: the person is unavailable.
		switch {
		case code == "10" || sub == "2018278":
			errCode = "WINDOW_CLOSED"
		case code == "551":
			errCode = "BLOCKED"
		}
		// 4 and 613 are rate limits.
		permanent := !retryable(res.status) && code != "4" && code != "613"
		return Result{}, Fail(errCode, network+": "+orDefault(str(e["message"]), http.StatusText(res.status)), permanent)
	}
	return Result{ProviderMessageID: str(res.body["message_id"])}, nil
}

// ---------- Telegram Bot API ----------

func sendTelegram(ctx context.Context, client *http.Client, cfg *Config, c Credentials, r TelegramRequest) (Result, error) {
	payload := map[string]any{"chat_id": r.ChatID, "text": r.Body}
	if r.HTML {
		payload["parse_mode"] = "HTML"
	}
	if len(r.Buttons) > 0 {
		rows := make([][]map[string]string, len(r.Buttons))
		for i, b := range r.Buttons {
			rows[i] = []map[string]string{{"text": b.Text, "url": b.URL}}
		}
		payload["reply_markup"] = map[string]any{"inline_keyboard": rows}
	}
	if r.DisablePreview {
		payload["link_preview_options"] = map[string]bool{"is_disabled": true}
	}
	if r.Silent {
		payload["disable_notification"] = true
	}
	body, _ := json.Marshal(payload)
	res, err := callJSON(ctx, client, http.MethodPost,
		cfg.TelegramBase+"/bot"+c["botToken"]+"/sendMessage",
		map[string]string{"Content-Type": "application/json"}, bytes.NewReader(body))
	if err != nil {
		return Result{}, err
	}
	if res.status < 200 || res.status >= 300 || res.body["ok"] == false {
		status := res.status
		if n, ok := res.body["error_code"].(float64); ok {
			status = int(n)
		}
		code := fmt.Sprintf("TELEGRAM_%d", status)
		// 403: the user blocked the bot or never started it.
		if status == 403 {
			code = "BLOCKED"
		}
		return Result{}, Fail(code, "Telegram: "+orDefault(str(res.body["description"]), http.StatusText(status)), !retryable(status))
	}
	id := ""
	if m, ok := res.body["result"].(map[string]any); ok {
		id = str(m["message_id"])
	}
	return Result{ProviderMessageID: id}, nil
}
