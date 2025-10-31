package storage

import (
	"entry-access-control/internal/config"
	"log/slog"
)

type Provider interface {
	Close() error
}

func NewProvider(config *config.Storage) Provider {
	switch {
	case config.SQLite != nil:
		return NewSQLiteProvider(config)
	default:
		slog.Error("Unsupported storage configuration", "config", config)
	}

	return nil
}
