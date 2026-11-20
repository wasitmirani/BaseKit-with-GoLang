package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/wasitmirani/linqflow-backend-golang-apis/config"
	"github.com/wasitmirani/linqflow-backend-golang-apis/internal/app"
	"github.com/wasitmirani/linqflow-backend-golang-apis/internal/shared/logger"
)

func main() {
	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize logger based on configuration
	var appLogger logger.Logger
	factory := logger.NewLoggerFactory()

	switch logger.LoggerType(cfg.Logging.Channel) {
	case logger.LoggerTypeFile, logger.LoggerTypeStack:
		// Create file logger config
		fileConfig := logger.FileLoggerConfig{
			LogPath:     cfg.Logging.LogPath,
			LogFileName: cfg.Logging.LogFileName,
			MaxSize:     cfg.Logging.MaxSize,
			MaxBackups:  cfg.Logging.MaxBackups,
			MaxAge:      cfg.Logging.MaxAge,
			Compress:    cfg.Logging.Compress,
			LocalTime:   true,
			DailyRotate: cfg.Logging.DailyRotate,
		}

		appLogger, err = factory.CreateLogger(logger.LoggerType(cfg.Logging.Channel), fileConfig)
		if err != nil {
			log.Fatalf("Failed to create file logger: %v", err)
		}

		// Close logger on exit if it has a Close method
		defer func() {
			if closer, ok := appLogger.(interface{ Close() error }); ok {
				closer.Close()
			}
		}()

	default:
		// Use stdout logger
		appLogger = logger.NewSimpleLogger()
	}

	appLogger.Info("Application starting", logger.Field{Key: "version", Value: cfg.App.Version})

	// Create application
	application, err := app.New(cfg, appLogger)
	if err != nil {
		appLogger.Fatal("Failed to create application", logger.Field{Key: "error", Value: err.Error()})
	}

	// Start server in a goroutine
	go func() {
		if err := application.Start(); err != nil && err.Error() != "http: Server closed" {
			appLogger.Fatal("Failed to start server", logger.Field{Key: "error", Value: err.Error()})
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	appLogger.Info("Shutting down server...")

	// The context is used to inform the server it has 30 seconds to finish
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := application.Shutdown(ctx); err != nil {
		appLogger.Error("Server forced to shutdown", logger.Field{Key: "error", Value: err.Error()})
	}

	appLogger.Info("Server exited")
}
