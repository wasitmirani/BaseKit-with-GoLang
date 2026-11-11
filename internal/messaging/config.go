package messaging

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config comes from environment variables. The names match backend-nestjs/.env,
// so the worker can run with `-env ../backend-nestjs/.env`.
type Config struct {
	DatabaseURL string

	EncryptionKey string // SECRETS_ENCRYPTION_KEY (must equal NestJS's)
	JWTSecret     string // development fallback for the key, like NestJS
	Production    bool

	WorkerID     string
	Concurrency  int
	BatchSize    int
	PollInterval time.Duration
	HTTPAddr     string // health and metrics; empty disables

	// Provider endpoints, overridable for tests and sandboxes.
	TwilioBase        string
	MetaBase          string
	TelegramBase      string
	SESBase           string // empty: https://email.<region>.amazonaws.com
	GoogleTokenURL    string
	MicrosoftTokenURL string

	GoogleClientID        string
	GoogleClientSecret    string
	MicrosoftClientID     string
	MicrosoftClientSecret string
}

func env(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if n, err := strconv.Atoi(os.Getenv(key)); err == nil && n > 0 {
		return n
	}
	return fallback
}

// LoadConfig reads the environment.
func LoadConfig() (*Config, error) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		u := url.URL{
			Scheme: "postgres",
			User:   url.UserPassword(env("DB_USER", "postgres"), os.Getenv("DB_PASS")),
			Host:   env("DB_HOST", "localhost") + ":" + env("DB_PORT", "5432"),
			Path:   "/" + env("DB_NAME", "birdcrm_db"),
		}
		q := url.Values{}
		q.Set("sslmode", env("DB_SSLMODE", "disable"))
		u.RawQuery = q.Encode()
		dbURL = u.String()
	}
	host, _ := os.Hostname()
	cfg := &Config{
		DatabaseURL:   dbURL,
		EncryptionKey: os.Getenv("SECRETS_ENCRYPTION_KEY"),
		JWTSecret:     os.Getenv("JWT_SECRET"),
		Production:    os.Getenv("NODE_ENV") == "production" || os.Getenv("APP_ENV") == "production",
		WorkerID:      env("MESSAGING_WORKER_ID", fmt.Sprintf("%s-%d", host, os.Getpid())),
		Concurrency:   envInt("MESSAGING_CONCURRENCY", 8),
		BatchSize:     envInt("MESSAGING_BATCH", 50),
		PollInterval:  time.Duration(envInt("MESSAGING_POLL_MS", 2000)) * time.Millisecond,
		HTTPAddr:      env("MESSAGING_HTTP_ADDR", ":8090"),

		TwilioBase:        strings.TrimRight(env("TWILIO_API_BASE", "https://api.twilio.com"), "/"),
		MetaBase:          strings.TrimRight(env("META_GRAPH_BASE", "https://graph.facebook.com"), "/"),
		TelegramBase:      strings.TrimRight(env("TELEGRAM_API_BASE", "https://api.telegram.org"), "/"),
		SESBase:           os.Getenv("SES_API_BASE"),
		GoogleTokenURL:    env("GOOGLE_TOKEN_URL", "https://oauth2.googleapis.com/token"),
		MicrosoftTokenURL: env("MICROSOFT_TOKEN_URL", "https://login.microsoftonline.com/common/oauth2/v2.0/token"),

		GoogleClientID:        os.Getenv("GOOGLE_OAUTH_CLIENT_ID"),
		GoogleClientSecret:    os.Getenv("GOOGLE_OAUTH_CLIENT_SECRET"),
		MicrosoftClientID:     os.Getenv("MICROSOFT_OAUTH_CLIENT_ID"),
		MicrosoftClientSecret: os.Getenv("MICROSOFT_OAUTH_CLIENT_SECRET"),
	}
	if os.Getenv("MESSAGING_HTTP_ADDR") == "off" {
		cfg.HTTPAddr = ""
	}
	return cfg, nil
}
