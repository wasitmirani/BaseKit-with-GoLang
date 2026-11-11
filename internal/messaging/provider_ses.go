package messaging

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// AWS Signature Version 4 (header form) for one request. Hand-written to avoid
// the AWS SDK; checked against AWS's published example in sigv4_test.go.
func signV4(req *http.Request, body []byte, accessKey, secretKey, region, service string, now time.Time) {
	amzDate := now.UTC().Format("20060102T150405Z")
	day := amzDate[:8]
	payloadHash := sha256Hex(body)
	req.Header.Set("X-Amz-Date", amzDate)
	host := req.URL.Host
	if req.Host != "" {
		host = req.Host
	}

	headers := map[string]string{"host": host}
	for k, v := range req.Header {
		lk := strings.ToLower(k)
		if lk == "content-type" || strings.HasPrefix(lk, "x-amz-") {
			headers[lk] = strings.Join(strings.Fields(strings.Join(v, ",")), " ")
		}
	}
	names := make([]string, 0, len(headers))
	for k := range headers {
		names = append(names, k)
	}
	sort.Strings(names)
	var canonHeaders strings.Builder
	for _, k := range names {
		canonHeaders.WriteString(k + ":" + headers[k] + "\n")
	}
	signed := strings.Join(names, ";")

	path := req.URL.EscapedPath()
	if path == "" {
		path = "/"
	}
	canonical := strings.Join([]string{
		req.Method, path, canonicalQuery(req.URL.Query()),
		canonHeaders.String(), signed, payloadHash,
	}, "\n")
	scope := day + "/" + region + "/" + service + "/aws4_request"
	toSign := "AWS4-HMAC-SHA256\n" + amzDate + "\n" + scope + "\n" + sha256Hex([]byte(canonical))
	k := hmacSHA256([]byte("AWS4"+secretKey), day)
	k = hmacSHA256(k, region)
	k = hmacSHA256(k, service)
	k = hmacSHA256(k, "aws4_request")
	sig := hex.EncodeToString(hmacSHA256(k, toSign))
	req.Header.Set("Authorization", fmt.Sprintf("AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s", accessKey, scope, signed, sig))
}

func canonicalQuery(q url.Values) string {
	keys := make([]string, 0, len(q))
	for k := range q {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var parts []string
	for _, k := range keys {
		vals := append([]string(nil), q[k]...)
		sort.Strings(vals)
		for _, v := range vals {
			parts = append(parts, awsEscape(k)+"="+awsEscape(v))
		}
	}
	return strings.Join(parts, "&")
}

func awsEscape(s string) string {
	return strings.ReplaceAll(url.QueryEscape(s), "+", "%20")
}

func sha256Hex(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func hmacSHA256(key []byte, data string) []byte {
	m := hmac.New(sha256.New, key)
	m.Write([]byte(data))
	return m.Sum(nil)
}

// sendSES sends raw MIME through the SES v2 SendEmail API.
func sendSES(ctx context.Context, client *http.Client, cfg *Config, c Credentials, raw []byte) (string, error) {
	region := c["region"]
	if region == "" {
		return "", Fail("NOT_CONFIGURED", "The SES account has no region.", true)
	}
	endpoint := fmt.Sprintf("https://email.%s.amazonaws.com", region)
	if cfg.SESBase != "" {
		endpoint = strings.TrimRight(cfg.SESBase, "/")
	}
	payload := map[string]any{"Content": map[string]any{"Raw": map[string]string{"Data": base64.StdEncoding.EncodeToString(raw)}}}
	if c["configurationSet"] != "" {
		payload["ConfigurationSetName"] = c["configurationSet"]
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint+"/v2/email/outbound-emails", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	signV4(req, body, c["accessKeyId"], c["secretAccessKey"], region, "ses", time.Now())
	res, err := client.Do(req)
	if err != nil {
		return "", Fail("TEMPORARY", "Amazon SES: "+err.Error(), false)
	}
	defer res.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	var out struct {
		MessageID string `json:"MessageId"`
		Message   string `json:"message"`
	}
	_ = json.Unmarshal(data, &out)
	if res.StatusCode == http.StatusOK {
		return out.MessageID, nil
	}
	kind := res.Header.Get("X-Amzn-Errortype")
	if i := strings.Index(kind, ":"); i >= 0 {
		kind = kind[:i]
	}
	msg := "Amazon SES: " + strings.TrimSpace(kind+" "+out.Message)
	switch {
	case res.StatusCode == 429 || res.StatusCode >= 500 || strings.Contains(kind, "Throttl") || strings.Contains(kind, "TooManyRequests"):
		return "", Fail("TEMPORARY", msg, false)
	case strings.Contains(kind, "MessageRejected") || strings.Contains(out.Message, "not verified"):
		return "", Fail("SENDER_NOT_VERIFIED", msg, true)
	case res.StatusCode == 403 || strings.Contains(kind, "Signature") || strings.Contains(kind, "Credential"):
		return "", Fail("AUTH_FAILED", msg, false)
	case strings.Contains(kind, "AccountSuspended") || strings.Contains(kind, "SendingPaused"):
		return "", Fail("ACCOUNT_PAUSED", msg, false)
	}
	return "", Fail("REJECTED", msg, true)
}
