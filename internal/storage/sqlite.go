package storage

import "entry-access-control/internal/config"

type SQLiteProvider struct {
	SQLProvider
}

func NewSQLiteProvider(config *config.Storage) (provider *SQLiteProvider) {
	return &SQLiteProvider{
		SQLProvider: *NewSQLProvider(config, "sqlite", config.SQLite.Path),
	}
}
