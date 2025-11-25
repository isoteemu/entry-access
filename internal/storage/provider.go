package storage

import (
	"context"
	"entry-access-control/internal/config"
	"log/slog"
	"time"
)

type Provider interface {
	Close() error
	GetSchemaVersion(ctx context.Context) (int, error)

	// Entry-related methods
	ListEntries(ctx context.Context) ([]Entry, error)
	CreateEntry(ctx context.Context, entry Entry) error
	DeleteEntry(ctx context.Context, entry Entry) error

	// Nonce-related methods
	CreateNonce(ctx context.Context, nonce string, expiresAt time.Time) error
	ExistsNonce(ctx context.Context, nonce string) (bool, error)
	ConsumeNonce(ctx context.Context, nonce string) (bool, error)
	ExpireNonces(ctx context.Context, now time.Time) error
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
