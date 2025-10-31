// TODO: Implement locking mechanism to prevent concurrent migrations

package storage

import (
	"context"
	"entry-access-control/internal/config"
	"entry-access-control/internal/utils"
	"fmt"
	"log/slog"
	"time"

	"github.com/jmoiron/sqlx"
)

type Queries struct {
	GetExistingTables      string
	GetLatestSchemaVersion string
	InsertMigration        string
}

type SQLProvider struct {
	db *sqlx.DB

	logger *slog.Logger

	Queries
}

func defaultQueries() Queries {

	return Queries{
		// GetExistingTables:      "",
		GetLatestSchemaVersion: "SELECT COALESCE(MAX(version_after), 0) FROM migrations",
		InsertMigration:        "INSERT INTO migrations (applied_at, version_before, version_after, application_version) VALUES (?, ?, ?, ?)",
	}
}

func NewSQLProvider(config *config.Storage, driverName string, dataSource string) (provider *SQLProvider) {
	db, err := sqlx.Open(driverName, dataSource)
	if err != nil {
		slog.Error("Failed to open database", "driver", driverName, "error", err)
		return nil
	}

	logger := slog.With("component", "storage")

	provider = &SQLProvider{
		db:     db,
		logger: logger,

		Queries: defaultQueries(),
	}

	return provider
}

func (p *SQLProvider) GetSchemaVersion(ctx context.Context) (int, error) {
	const tableName = "migrations"

	p.logger.Debug("Getting schema version", "sql", p.Queries.GetExistingTables)
	// Verify the table exists
	rows, err := p.db.QueryContext(ctx, p.Queries.GetExistingTables)
	if err != nil {
		p.logger.Error("Failed to get existing tables", "error", err)
		return -1, err
	}

	version := 0
	var name string

	defer rows.Close()

	for rows.Next() {
		if err := rows.Scan(&name); err != nil {
			p.logger.Error("Failed to scan table name", "error", err)
			return -1, err
		}
		if name == tableName {
			// Migrations table exists, get the latest schema version
			if err := p.db.GetContext(ctx, &version, p.Queries.GetLatestSchemaVersion); err != nil {
				p.logger.Error("Failed to get latest schema version", "error", err)
				return -1, err
			}
			return version, nil
		}
	}

	// No migrations table found, return version 0
	return 0, nil
}

// runMigrations executes database migrations
func (p *SQLProvider) runMigrations(driverName string) error {
	runner := NewMigrationRunner(driverName)

	currentVersion, err := p.GetSchemaVersion(context.Background())
	if err != nil {
		return err
	}

	targetVersion, err := runner.GetLatestMigrationVersion()
	if err != nil {
		return err
	}

	if currentVersion == targetVersion {
		p.logger.Info("Database schema is up to date", "version", currentVersion)
		return nil
	}

	migrations, err := runner.LoadMigrations(currentVersion, targetVersion)
	if err != nil {
		return err
	}

	for _, migration := range migrations.([]SchemaMigration) {
		p.logger.Info("Applying migration", "version", migration.Version, "name", migration.Name)
		if err := p.ApplyMigration(migration); err != nil {
			p.logger.Error("Failed to apply migration", "version", migration.Version, "name", migration.Name, "error", err)
			return err
		}
	}

	return nil
}

func (p *SQLProvider) ApplyMigration(migration SchemaMigration) error {
	tx, err := p.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	ctx := context.Background()

	// Execute migration SQL
	if _, err := tx.ExecContext(ctx, migration.SQL); err != nil {
		return fmt.Errorf("failed to execute migration SQL: %w", err)
	}

	// Insert migration record
	if _, err := tx.ExecContext(ctx,
		p.Queries.InsertMigration,
		time.Now(),
		migration.Before(),
		migration.After(),
		utils.GetVersion(),
	); err != nil {
		return fmt.Errorf("failed to insert migration record: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	p.logger.Info("Migration applied successfully",
		"version", migration.Version,
		"name", migration.Name,
		"before", migration.Before(),
		"after", migration.After())

	return nil
}

func (p *SQLProvider) Close() error {
	if p.db != nil {
		return p.db.Close()
	}
	return nil
}
