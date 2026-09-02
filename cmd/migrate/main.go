package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/erizkiatama/ansara-gym/internal/config"
	"github.com/erizkiatama/ansara-gym/internal/db"
	"github.com/pressly/goose/v3"
)

func main() {
	if err := run(); err != nil {
		slog.Error("migrate failed", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.Log.Level}))
	slog.SetDefault(log)

	cmd := "up"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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

	provider, err := db.NewMigrationProvider(database, log)
	if err != nil {
		return err
	}

	switch cmd {
	case "up":
		results, err := provider.Up(ctx)
		if err != nil {
			return fmt.Errorf("up: %w", err)
		}
		logResults(log, results, "up")
	case "down":
		result, err := provider.Down(ctx)
		if err != nil {
			if errors.Is(err, goose.ErrNoNextVersion) {
				log.Info("no migrations to roll back")
				return nil
			}
			return fmt.Errorf("down: %w", err)
		}
		logResult(log, result)
	case "reset":
		results, err := provider.DownTo(ctx, 0)
		if err != nil {
			return fmt.Errorf("reset: %w", err)
		}
		logResults(log, results, "reset")
	case "status":
		statuses, err := provider.Status(ctx)
		if err != nil {
			return fmt.Errorf("status: %w", err)
		}
		for _, s := range statuses {
			args := []any{
				"version", s.Source.Version,
				"path", s.Source.Path,
				"state", s.State,
			}
			if !s.AppliedAt.IsZero() {
				args = append(args, "applied_at", s.AppliedAt)
			}
			log.Info("migration status", args...)
		}
	default:
		return fmt.Errorf("unknown command %q (want up, down, reset, status)", cmd)
	}

	return nil
}

func logResults(log *slog.Logger, results []*goose.MigrationResult, direction string) {
	if len(results) == 0 {
		log.Info("no migrations to apply", "command", direction)
		return
	}
	for _, r := range results {
		logResult(log, r)
	}
}

func logResult(log *slog.Logger, r *goose.MigrationResult) {
	log.Info("migration",
		"version", r.Source.Version,
		"path", r.Source.Path,
		"direction", r.Direction,
		"empty", r.Empty,
		"duration", r.Duration,
	)
}
