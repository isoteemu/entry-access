package storage

import (
	"entry-access-control/internal/config"
	"log/slog"

	"github.com/jmoiron/sqlx"
)

type SQLProvider struct {
	db *sqlx.DB

	config *config.Storage // Maybe not needed?

	logger *slog.Logger
}

func NewSQLProvider(config *config.Storage, driverName string, dataSource string) (provider *SQLProvider) {
	db, err := sqlx.Open(driverName, dataSource)
	if err != nil {
		return nil
	}

	logger := slog.With("component", "storage")

	return &SQLProvider{
		db:     db,
		config: config,
		logger: logger,
	}
}

func (p *SQLProvider) Close() error {
	if p.db != nil {
		return p.db.Close()
	}
	return nil
}
