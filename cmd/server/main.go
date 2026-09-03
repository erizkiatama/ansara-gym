package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/erizkiatama/ansara-gym/internal/config"
	"github.com/erizkiatama/ansara-gym/internal/db"
	"github.com/erizkiatama/ansara-gym/internal/server"
)

func main() {
	if err := run(); err != nil {
		slog.Error("startup failed", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	if err := cfg.Auth.Validate(); err != nil {
		return fmt.Errorf("config: %w", err)
	}

	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.Log.Level}))
	slog.SetDefault(log)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	database, err := db.Open(ctx, cfg.Database)
	if err != nil {
		return fmt.Errorf("database: %w", err)
	}
	defer func() {
		if cerr := database.Close(); cerr != nil {
			log.Error("closing database", "err", cerr)
		}
	}()

	addr := fmt.Sprintf(":%d", cfg.App.Port)
	httpServer := &http.Server{
		Addr:              addr,
		Handler:           server.New(log, database, cfg.Auth),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Info("listening", "addr", addr)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("http: %w", err)
	}
	return nil
}
