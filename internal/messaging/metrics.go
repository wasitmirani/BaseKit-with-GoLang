package messaging

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

// Metrics are Prometheus counters, written in the text exposition format by hand
// (no client library needed for a handful of series).
type Metrics struct {
	mu       sync.Mutex
	counters map[string]float64 // full series name with labels → value
	started  time.Time
	lastPoll time.Time
}

func NewMetrics() *Metrics {
	return &Metrics{counters: map[string]float64{}, started: time.Now()}
}

func series(name string, labels ...string) string {
	if len(labels) == 0 {
		return name
	}
	var parts []string
	for i := 0; i+1 < len(labels); i += 2 {
		parts = append(parts, fmt.Sprintf(`%s=%q`, labels[i], labels[i+1]))
	}
	return name + "{" + strings.Join(parts, ",") + "}"
}

func (m *Metrics) Add(name string, v float64, labels ...string) {
	m.mu.Lock()
	m.counters[series(name, labels...)] += v
	m.mu.Unlock()
}

func (m *Metrics) Polled() {
	m.mu.Lock()
	m.lastPoll = time.Now()
	m.mu.Unlock()
}

var help = map[string]string{
	"messaging_jobs_sent_total":        "Jobs the provider accepted.",
	"messaging_jobs_failed_total":      "Jobs that failed for good.",
	"messaging_jobs_retried_total":     "Attempts that failed and were rescheduled.",
	"messaging_provider_seconds_sum":   "Time spent in provider calls.",
	"messaging_provider_seconds_count": "Provider calls.",
	"messaging_rate_limited_total":     "Jobs handed back because an account was over its rate.",
	"messaging_queue_jobs":             "Jobs in delivery_jobs by status (sent/failed: not yet applied by NestJS).",
	"messaging_uptime_seconds":         "Seconds since the worker started.",
	"messaging_last_poll_age_seconds":  "Seconds since the queue was last polled.",
}

// Handler serves /healthz and /metrics.
func (m *Metrics) Handler(store *Store, pollEvery time.Duration) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		m.mu.Lock()
		age := time.Since(m.lastPoll)
		m.mu.Unlock()
		if err := store.pool.Ping(ctx); err != nil {
			http.Error(w, "database: "+err.Error(), http.StatusServiceUnavailable)
			return
		}
		if age > 10*pollEvery+30*time.Second {
			http.Error(w, "queue loop stalled", http.StatusServiceUnavailable)
			return
		}
		fmt.Fprintln(w, "ok")
	})
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		m.mu.Lock()
		lines := make([]string, 0, len(m.counters))
		for k, v := range m.counters {
			lines = append(lines, fmt.Sprintf("%s %g", k, v))
		}
		uptime := time.Since(m.started).Seconds()
		age := time.Since(m.lastPoll).Seconds()
		m.mu.Unlock()
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if stats, err := store.Stats(ctx); err == nil {
			for st, n := range stats {
				lines = append(lines, fmt.Sprintf("%s %d", series("messaging_queue_jobs", "status", st), n))
			}
		}
		lines = append(lines,
			fmt.Sprintf("messaging_uptime_seconds %g", uptime),
			fmt.Sprintf("messaging_last_poll_age_seconds %g", age))
		sort.Strings(lines)
		seen := map[string]bool{}
		for _, l := range lines {
			name := l[:strings.IndexAny(l, "{ ")]
			if !seen[name] {
				seen[name] = true
				if h, ok := help[name]; ok {
					fmt.Fprintf(w, "# HELP %s %s\n", name, h)
				}
				kind := "counter"
				if !strings.HasSuffix(name, "_total") && !strings.HasSuffix(name, "_sum") && !strings.HasSuffix(name, "_count") {
					kind = "gauge"
				}
				fmt.Fprintf(w, "# TYPE %s %s\n", name, kind)
			}
			fmt.Fprintln(w, l)
		}
	})
	return mux
}
