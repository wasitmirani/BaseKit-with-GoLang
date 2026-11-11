package messaging

import (
	"strconv"
	"sync"
	"time"
)

// Default sends per second per account, below each provider's usual limit.
// An account can override it with a `rateLimitPerSecond` credential field.
var defaultRates = map[string]float64{
	"ses":           14, // SES sandbox and new production accounts
	"smtp":          5,
	"gmail":         1,
	"outlook":       1,
	"twilio":        10,
	"meta_whatsapp": 50,
	"telegram":      25,
}

// Limiter is a token bucket per channel account (per process; with several
// workers each gets the full rate, so set MESSAGING_CONCURRENCY and the rates
// with the number of workers in mind).
type Limiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
}

type bucket struct {
	rate   float64 // tokens per second
	tokens float64
	last   time.Time
}

func NewLimiter() *Limiter { return &Limiter{buckets: map[string]*bucket{}} }

// RateFor returns the account's sends per second.
func RateFor(acc *Account) float64 {
	if v, err := strconv.ParseFloat(acc.Credentials["rateLimitPerSecond"], 64); err == nil && v > 0 {
		return v
	}
	if r, ok := defaultRates[acc.Provider]; ok {
		return r
	}
	return 5
}

// Reserve takes one token and returns how long to wait before sending.
func (l *Limiter) Reserve(accountID string, rate float64, now time.Time) time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	b, ok := l.buckets[accountID]
	if !ok || b.rate != rate {
		// Burst of one second's worth.
		b = &bucket{rate: rate, tokens: rate, last: now}
		l.buckets[accountID] = b
	}
	b.tokens += now.Sub(b.last).Seconds() * b.rate
	if b.tokens > b.rate {
		b.tokens = b.rate
	}
	b.last = now
	b.tokens--
	if b.tokens >= 0 {
		return 0
	}
	return time.Duration(-b.tokens / b.rate * float64(time.Second))
}

// Cancel gives back a token that was reserved but not used.
func (l *Limiter) Cancel(accountID string) {
	l.mu.Lock()
	if b, ok := l.buckets[accountID]; ok {
		b.tokens++
	}
	l.mu.Unlock()
}
