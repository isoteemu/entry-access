package storage

import (
	"context"
	"entry-access-control/internal/config"
	"log/slog"
)

type Provider interface {
	Close() error
	GetSchemaVersion(ctx context.Context) (int, error)
}

func NewProvider(config *config.Storage) Provider {
	switch {
	case config.SQLite != nil:
		provider := NewSQLiteProvider(config)
		if err := provider.runMigrations("sqlite3"); err != nil {
			slog.Error("Failed to run migrations", "error", err)
			return nil
		}
		return provider

	default:
		slog.Error("Unsupported storage configuration", "config", config)
	}

	return nil
}
