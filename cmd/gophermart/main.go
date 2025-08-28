package main

import (
	"context"
	"fmt"
	"log"
	"loyaltySys/internal/auth"
	"loyaltySys/internal/config"
	"loyaltySys/internal/handlers"
	"loyaltySys/internal/logger"
	"loyaltySys/internal/server"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	// Load server configuration
	cfg, err := config.GetConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	// Initialize logger
	l, err := logger.Initialize(cfg.LogLevel)
	if err != nil {
		return fmt.Errorf("failed to initialize logger: %w", err)
	}
	defer l.SafeSync()

	// create a context that listens for OS signals to shut down the server
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Initialize JWT from environment variables
	if err := auth.InitJWTFromEnv(); err != nil {
		return fmt.Errorf("failed to initialize JWT: %w", err)
	}

	// Initialize storage
	storage := handlers.NewStorage(ctx, cfg.DSN, l.SugaredLogger)
	// Initialize handler
	h := handlers.NewHandler(storage, l.SugaredLogger)
	// Initialize server
	srv := server.NewServer(cfg, h, l.SugaredLogger)
	// Start server
	if err := srv.Start(ctx); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}
	return nil
}
