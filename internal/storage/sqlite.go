package storage

import (
	"entry-access-control/internal/config"

	_ "github.com/mattn/go-sqlite3" // SQLite driver
)

type SQLiteProvider struct {
	SQLProvider
}

func NewSQLiteProvider(config *config.Storage) (provider *SQLiteProvider) {
	sqlProvider := NewSQLProvider(config, "sqlite3", config.SQLite.Path)
	if sqlProvider == nil {
		panic("failed to create SQLite provider")
	}

	// Override queries for SQLite
	sqlProvider.Queries.GetExistingTables = `SELECT name FROM sqlite_master WHERE type='table';`
	sqlProvider.Queries.GetLatestSchemaVersion = `SELECT COALESCE(MAX(version_after), 0) FROM migrations;`

	storage := &SQLiteProvider{
		SQLProvider: *sqlProvider,
	}

	return storage

}

func (p *SQLiteProvider) Close() error {
	return p.db.Close()
}
