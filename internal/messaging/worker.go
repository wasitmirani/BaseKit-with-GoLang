package messaging

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// MaxAttempts per job; waits before attempts 2..5 match NestJS's in-process worker.
const MaxAttempts = 5

var Backoff = []time.Duration{time.Minute, 5 * time.Minute, 30 * time.Minute, 2 * time.Hour}

// Longest rate-limit wait spent holding a job; longer waits hand it back.
const maxInlineWait = 5 * time.Second

// Worker claims delivery jobs and sends them with bounded concurrency.
type Worker struct {
	cfg     *Config
	pool    *pgxpool.Pool
	store   *Store
	sender  *Sender
	limiter *Limiter
	metrics *Metrics
	log     *slog.Logger
}

func NewWorker(cfg *Config, pool *pgxpool.Pool, store *Store, sender *Sender, metrics *Metrics, log *slog.Logger) *Worker {
	return &Worker{cfg: cfg, pool: pool, store: store, sender: sender, limiter: NewLimiter(), metrics: metrics, log: log}
}

// Run polls until ctx is cancelled, then waits for in-flight sends to finish.
func (w *Worker) Run(ctx context.Context) error {
	wake := make(chan struct{}, 1)
	go w.listen(ctx, wake)

	slots := make(chan struct{}, w.cfg.Concurrency)
	var inflight sync.WaitGroup
	defer inflight.Wait()

	ticker := time.NewTicker(w.cfg.PollInterval)
	defer ticker.Stop()
	for {
		w.metrics.Polled()
		free := w.cfg.Concurrency - len(slots)
		if free > w.cfg.BatchSize {
			free = w.cfg.BatchSize
		}
		claimed := 0
		if free > 0 {
			jobs, err := w.store.Claim(ctx, w.cfg.WorkerID, free)
			if err != nil && ctx.Err() == nil {
				w.log.Error("claim failed", "error", err)
			}
			claimed = len(jobs)
			for _, job := range jobs {
				slots <- struct{}{}
				inflight.Add(1)
				go func(job Job) {
					defer func() { <-slots; inflight.Done() }()
					w.process(ctx, job)
				}(job)
			}
		}
		// A full batch means more is probably waiting: go again right away.
		if claimed > 0 && claimed == free {
			select {
			case <-ctx.Done():
				return nil
			case slots <- struct{}{}: // wait for one free slot
				<-slots
			}
			continue
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		case <-wake:
		}
	}
}

// listen wakes the loop on NOTIFY delivery_jobs (sent by NestJS at commit).
func (w *Worker) listen(ctx context.Context, wake chan<- struct{}) {
	for ctx.Err() == nil {
		conn, err := w.pool.Acquire(ctx)
		if err != nil {
			sleep(ctx, 5*time.Second)
			continue
		}
		if _, err = conn.Exec(ctx, "LISTEN delivery_jobs"); err == nil {
			for {
				if _, err = conn.Conn().WaitForNotification(ctx); err != nil {
					break
				}
				select {
				case wake <- struct{}{}:
				default:
				}
			}
		}
		conn.Release()
		if ctx.Err() == nil {
			w.log.Warn("LISTEN connection lost; polling until it is back", "error", err)
			sleep(ctx, 5*time.Second)
		}
	}
}

func sleep(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}

// process sends one claimed job and records the outcome. Store writes use a
// fresh context so a shutdown never leaves a sent message unrecorded.
func (w *Worker) process(ctx context.Context, job Job) {
	save, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	log := w.log.With("job", job.ID, "kind", job.Kind, "tenant", job.TenantID, "attempt", job.Attempts)

	result, err := w.attempt(ctx, job)
	var f *Failure
	switch {
	case err == nil:
		if err := w.store.Sent(save, job, w.cfg.WorkerID, result, time.Now().UTC()); err != nil {
			log.Error("could not record sent job", "error", err)
		}
		w.metrics.Add("messaging_jobs_sent_total", 1, "kind", job.Kind)
		return
	case errors.Is(err, errRateLimited) || errors.Is(err, context.Canceled):
		_ = w.store.Release(save, job, w.cfg.WorkerID, time.Now().Add(time.Second))
		return
	case errors.As(err, &f):
	default:
		f = Fail("ENGINE_ERROR", err.Error(), false)
	}

	if f.Permanent || job.Attempts >= MaxAttempts {
		if err := w.store.Failed(save, job, w.cfg.WorkerID, f, time.Now().UTC()); err != nil {
			log.Error("could not record failed job", "error", err)
		}
		w.metrics.Add("messaging_jobs_failed_total", 1, "kind", job.Kind, "code", f.Code)
		log.Info("job failed", "code", f.Code, "reason", f.Message)
		return
	}
	next := time.Now().Add(Backoff[min(job.Attempts-1, len(Backoff)-1)])
	if err := w.store.Retry(save, job, w.cfg.WorkerID, f, next); err != nil {
		log.Error("could not reschedule job", "error", err)
	}
	w.metrics.Add("messaging_jobs_retried_total", 1, "kind", job.Kind, "code", f.Code)
	log.Warn("attempt failed; retrying", "code", f.Code, "reason", f.Message, "next", next)
}

var errRateLimited = errors.New("rate limited")

func (w *Worker) attempt(ctx context.Context, job Job) (Result, error) {
	if job.CampaignID != nil {
		stopped, err := w.store.CampaignStopped(ctx, job.TenantID, *job.CampaignID)
		if err != nil {
			return Result{}, err
		}
		if stopped != "" {
			return Result{}, Fail("CAMPAIGN_STOPPED", "The campaign is "+stopped+".", true)
		}
	}
	acc, err := w.store.Account(ctx, job.TenantID, job.AccountID)
	if err != nil {
		return Result{}, err
	}
	if wait := w.limiter.Reserve(acc.ID, RateFor(acc), time.Now()); wait > 0 {
		if wait > maxInlineWait {
			w.limiter.Cancel(acc.ID)
			w.metrics.Add("messaging_rate_limited_total", 1, "provider", acc.Provider)
			return Result{}, errRateLimited
		}
		sleep(ctx, wait)
		if ctx.Err() != nil {
			return Result{}, ctx.Err()
		}
	}
	callCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	start := time.Now()
	result, err := w.sender.Send(callCtx, job, acc)
	w.metrics.Add("messaging_provider_seconds_sum", time.Since(start).Seconds(), "provider", acc.Provider)
	w.metrics.Add("messaging_provider_seconds_count", 1, "provider", acc.Provider)
	return result, err
}
