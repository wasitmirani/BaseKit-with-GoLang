package messaging

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSecretBoxSealRoundTrip(t *testing.T) {
	box, _ := NewSecretBox("test-key-0123456789-0123456789-abcdef", "", true)
	sealed, err := box.Seal(Credentials{"email": "a@example.com", "refreshToken": "r2"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(sealed, "v1:") || strings.Count(sealed, ":") != 3 || strings.Contains(sealed, "=") {
		t.Fatalf("not the NestJS format: %s", sealed)
	}
	out := Credentials{}
	if err := box.Open(sealed, &out); err != nil || out["refreshToken"] != "r2" || out["email"] != "a@example.com" {
		t.Fatalf("round trip: %v %v", out, err)
	}
	other, _ := NewSecretBox("another-key-0123456789-0123456789-xyz", "", true)
	if err := other.Open(sealed, &out); err == nil {
		t.Fatal("opened with the wrong key")
	}
}

func TestRefreshTokenRotationIsReportedAndStored(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		// Microsoft answers every refresh with a new refresh token.
		w.Write([]byte(`{"access_token":"at-1","expires_in":3600,"refresh_token":"rt-new"}`))
	}))
	defer srv.Close()
	cfg := &Config{MicrosoftTokenURL: srv.URL, MicrosoftClientID: "id", MicrosoftClientSecret: "s"}
	ts := NewTokenSource(cfg, srv.Client())

	tok, rotated, err := ts.TokenRotating(context.Background(), "acc", "outlook", "rt-old")
	if err != nil || tok != "at-1" || rotated != "rt-new" {
		t.Fatalf("got %q %q %v", tok, rotated, err)
	}
	// Cached: no new refresh token to store.
	if _, rotated, _ := ts.TokenRotating(context.Background(), "acc", "outlook", "rt-new"); rotated != "" {
		t.Fatalf("cached call reported rotation %q", rotated)
	}
	// Same refresh token returned (Google): nothing to store.
	ts2 := NewTokenSource(cfg, srv.Client())
	if _, rotated, _ := ts2.TokenRotating(context.Background(), "acc", "outlook", "rt-new"); rotated != "" {
		t.Fatalf("unchanged token reported as rotated")
	}
}
