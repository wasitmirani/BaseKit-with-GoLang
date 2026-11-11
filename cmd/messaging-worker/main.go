// Command messaging-worker is the delivery engine: it sends the messages NestJS
// queues in delivery_jobs (run NestJS with DELIVERY_ENGINE=go).
//
//	go run ./cmd/messaging-worker -env ../backend-nestjs/.env
//
// Several workers may run at once; jobs are claimed with FOR UPDATE SKIP LOCKED.
// Health and Prometheus metrics: http://localhost:8090/healthz and /metrics.
package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/subosito/gotenv"

	"BaseKit-with-GoLang/internal/messaging"
)

func main() {
	envFile := flag.String("env", "", "load environment variables from this file (e.g. ../backend-nestjs/.env); existing variables win")
	flag.Parse()
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if *envFile != "" {
		if err := gotenv.Load(*envFile); err != nil {
			log.Error("could not read env file", "file", *envFile, "error", err)
			os.Exit(1)
		}
	}
	if err := run(log); err != nil {
		log.Error("messaging worker stopped", "error", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	cfg, err := messaging.LoadConfig()
	if err != nil {
		return err
	}
	box, err := messaging.NewSecretBox(cfg.EncryptionKey, cfg.JWTSecret, cfg.Production)
	if err != nil {
		return err
	}
	if len(cfg.EncryptionKey) < 32 {
		log.Warn("SECRETS_ENCRYPTION_KEY is not set; using the development key derived from JWT_SECRET (same as NestJS)")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	poolCfg, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return err
	}
	poolCfg.MaxConns = int32(cfg.Concurrency + 4) // senders, the claim loop, LISTEN, metrics
	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		return errors.New("database unreachable: " + err.Error())
	}

	store := messaging.NewStore(pool, box)
	metrics := messaging.NewMetrics()
	worker := messaging.NewWorker(cfg, pool, store, messaging.NewSender(cfg), metrics, log)

	var srv *http.Server
	if cfg.HTTPAddr != "" {
		srv = &http.Server{Addr: cfg.HTTPAddr, Handler: metrics.Handler(store, cfg.PollInterval), ReadHeaderTimeout: 5 * time.Second}
		go func() {
			if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				log.Error("health server failed", "error", err)
			}
		}()
	}

	log.Info("messaging worker started", "worker", cfg.WorkerID, "concurrency", cfg.Concurrency, "http", cfg.HTTPAddr)
	err = worker.Run(ctx)
	if srv != nil {
		shut, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shut)
	}
	log.Info("messaging worker stopped")
	return err
}
