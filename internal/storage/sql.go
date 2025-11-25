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

// txKey is the context key for SQL transactions.
const txKey = iota

type SQL = string

type Queries struct {
	GetExistingTables      SQL
	GetLatestSchemaVersion SQL
	InsertMigration        SQL

	// --- Entry-related queries ---
	ListEntries SQL
	CreateEntry SQL
	DeleteEntry SQL

	// --- Nonce-related queries ---
	CreateNonce  SQL
	ExistsNonce  SQL
	ConsumeNonce SQL
	ExpireNonces SQL
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

		// --- Entry-related queries ---
		ListEntries: "SELECT id, name, created_at FROM entryways WHERE deleted_at IS NULL ORDER BY created_at DESC",
		CreateEntry: "INSERT INTO entryways (name, created_at) VALUES (?, ?)",
		DeleteEntry: "UPDATE entryways SET deleted_at = ? WHERE id = ? AND deleted_at IS NULL",

		// --- Nonce-related queries ---
		CreateNonce:  "INSERT INTO nonces (nonce, expires_at) VALUES (?, ?)",
		ExistsNonce:  "SELECT COUNT(1) FROM nonces WHERE nonce = ? AND expires_at > ?",
		ConsumeNonce: "DELETE FROM nonces WHERE nonce = ?",
		ExpireNonces: "DELETE FROM nonces WHERE expires_at <= ?",
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

// BeginTx starts a new transaction and returns a new context containing the transaction.
func (p *SQLProvider) BeginTx(ctx context.Context) (c context.Context, err error) {
	if tx, err := p.db.BeginTx(ctx, nil); err != nil {
		return nil, err
	} else {
		c = context.WithValue(ctx, txKey, tx)
		return c, nil
	}
}

// RollbackTx rolls back the transaction in the context.
func (p *SQLProvider) RollbackTx(ctx context.Context) error {
	tx, ok := ctx.Value(txKey).(*sqlx.Tx)
	if !ok {
		return fmt.Errorf("no transaction found in context")
	}
	return tx.Rollback()
}

// CommitTx commits the transaction in the context.
func (p *SQLProvider) CommitTx(ctx context.Context) error {
	tx, ok := ctx.Value(txKey).(*sqlx.Tx)
	if !ok {
		return fmt.Errorf("no transaction found in context")
	}
	return tx.Commit()
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

// --- Entry-related methods ---
func (p *SQLProvider) ListEntries(ctx context.Context) ([]Entry, error) {
	var entries []Entry

	if err := p.db.SelectContext(ctx, &entries, p.Queries.ListEntries); err != nil {
		return nil, fmt.Errorf("failed to list entries: %w", err)
	}

	return entries, nil

}

func (p *SQLProvider) CreateEntry(ctx context.Context, entry Entry) error {
	createdAt := entry.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}

	result, err := p.db.ExecContext(ctx, p.Queries.CreateEntry, entry.Name, createdAt)
	if err != nil {
		return fmt.Errorf("failed to create entry: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}

	entry.ID = id
	p.logger.Debug("Entry created", "id", id, "name", entry.Name)

	return nil
}

func (p *SQLProvider) DeleteEntry(ctx context.Context, entry Entry) error {
	result, err := p.db.ExecContext(ctx, p.Queries.DeleteEntry, time.Now(), entry.ID)
	if err != nil {
		return fmt.Errorf("failed to delete entry: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("entry not found or already deleted: %d", entry.ID)
	}

	p.logger.Debug("Entry deleted", "id", entry.ID, "name", entry.Name)

	return nil
}

// --- Nonce-related methods ---
func (p *SQLProvider) CreateNonce(ctx context.Context, nonce string, expiresAt time.Time) error {

	_, err := p.db.ExecContext(ctx, p.Queries.CreateNonce, nonce, expiresAt.UTC().Unix())
	if err != nil {
		return fmt.Errorf("failed to create nonce: %w", err)
	}
	return nil
}

func (p *SQLProvider) ExistsNonce(ctx context.Context, nonce string) (bool, error) {
	var count int
	now := time.Now().UTC().Unix()
	err := p.db.GetContext(ctx, &count, p.Queries.ExistsNonce, nonce, now)
	if err != nil {
		return false, fmt.Errorf("failed to check nonce existence: %w", err)
	}
	return count > 0, nil
}

func (p *SQLProvider) ConsumeNonce(ctx context.Context, nonce string) (bool, error) {
	result, err := p.db.ExecContext(ctx, p.Queries.ConsumeNonce, nonce)
	if err != nil {
		return false, fmt.Errorf("failed to consume nonce: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("failed to get rows affected: %w", err)
	}

	return rowsAffected > 0, nil
}

func (p *SQLProvider) ExpireNonces(ctx context.Context, now time.Time) error {
	_, err := p.db.ExecContext(ctx, p.Queries.ExpireNonces, now.UTC().Unix())
	if err != nil {
		return fmt.Errorf("failed to expire nonces: %w", err)
	}
	return nil
}
