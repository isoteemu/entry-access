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

	// --- Device provisioning queries ---
	CreateDevice       SQL
	GetDevice          SQL
	ListDevices        SQL
	UpdateDeviceStatus SQL

	// --- Approved device queries ---
	CreateApprovedDevice        SQL
	GetApprovedDevice           SQL
	ListApprovedDevicesByDevice SQL
	ListApprovedDevicesByEntry  SQL
	RevokeApprovedDevice        SQL
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
		ListEntries: "SELECT id, name, calendar_url, created_at FROM entries WHERE deleted_at IS NULL ORDER BY created_at DESC",
		CreateEntry: "INSERT INTO entries (name, calendar_url, created_at) VALUES (?, ?, ?)",
		DeleteEntry: "UPDATE entries SET deleted_at = ? WHERE id = ? AND deleted_at IS NULL",

		// --- Nonce-related queries ---
		CreateNonce:  "INSERT INTO nonces (nonce, expires_at) VALUES (?, ?)",
		ExistsNonce:  "SELECT COUNT(1) FROM nonces WHERE nonce = ? AND expires_at > ?",
		ConsumeNonce: "DELETE FROM nonces WHERE nonce = ?",
		ExpireNonces: "DELETE FROM nonces WHERE expires_at <= ?",

		// --- Device provisioning queries ---
		CreateDevice:       "INSERT INTO devices (device_id, client_ip, created_at, updated_at, status) VALUES (?, ?, ?, ?, ?)",
		GetDevice:          "SELECT device_id, client_ip, created_at, updated_at, status, approved_by FROM devices WHERE device_id = ?",
		ListDevices:        "SELECT device_id, client_ip, created_at, updated_at, status, approved_by FROM devices WHERE status = ? ORDER BY created_at DESC",
		UpdateDeviceStatus: "UPDATE devices SET status = ?, updated_at = ?, approved_by = ? WHERE device_id = ?",

		// --- Approved device queries ---
		CreateApprovedDevice:        "INSERT INTO approved_devices (device_id, entry_id, approved_by, approved_at) VALUES (?, ?, ?, ?)",
		GetApprovedDevice:           "SELECT id, device_id, entry_id, approved_by, approved_at, revoked_at FROM approved_devices WHERE device_id = ? AND entry_id = ? AND revoked_at IS NULL",
		ListApprovedDevicesByDevice: "SELECT id, device_id, entry_id, approved_by, approved_at, revoked_at FROM approved_devices WHERE device_id = ? AND revoked_at IS NULL ORDER BY approved_at DESC",
		ListApprovedDevicesByEntry:  "SELECT id, device_id, entry_id, approved_by, approved_at, revoked_at FROM approved_devices WHERE entry_id = ? AND revoked_at IS NULL ORDER BY approved_at DESC",
		RevokeApprovedDevice:        "UPDATE approved_devices SET revoked_at = ? WHERE device_id = ? AND entry_id = ? AND revoked_at IS NULL",
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

	previousLogger := p.logger
	defer func() {
		p.logger = previousLogger
	}()

	p.logger = p.logger.With("component", "migration").With("migration_driver", driverName)

	currentVersion, err := p.GetSchemaVersion(context.Background())
	if err != nil {
		p.logger.Error("Failed to get current schema version", "error", err)
		return err
	}

	targetVersion, err := runner.GetLatestMigrationVersion()
	if err != nil {
		p.logger.Error("Failed to get target schema version", "error", err)
		return err
	}

	if currentVersion == targetVersion {
		p.logger.Info("Database schema is up to date", "version", currentVersion)
		return nil
	}

	migrations, err := runner.LoadMigrations(currentVersion, targetVersion)
	if err != nil {
		p.logger.Error("Failed to load migrations", "error", err)
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
		p.logger.Error("Failed to execute migration SQL", "error", err, "sql", migration.SQL)
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
		p.logger.Error("Failed to insert migration record", "error", err, "sql", p.Queries.InsertMigration)
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

	result, err := p.db.ExecContext(ctx, p.Queries.CreateEntry, entry.Name, entry.CalendarURL, createdAt)
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

// --- Device provisioning methods ---
func (p *SQLProvider) CreateDevice(ctx context.Context, device Device) error {
	createdAt := device.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}
	updatedAt := device.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = time.Now()
	}
	status := device.Status
	if status == "" {
		status = DeviceStatusPending
	}

	_, err := p.db.ExecContext(ctx, p.Queries.CreateDevice, device.DeviceID, device.ClientIP, createdAt, updatedAt, status)
	if err != nil {
		return fmt.Errorf("failed to create device: %w", err)
	}

	p.logger.Debug("Device created", "device_id", device.DeviceID, "client_ip", device.ClientIP)

	return nil
}

func (p *SQLProvider) GetDevice(ctx context.Context, deviceID string) (*Device, error) {
	var device Device

	err := p.db.GetContext(ctx, &device, p.Queries.GetDevice, deviceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get device: %w", err)
	}

	return &device, nil
}

func (p *SQLProvider) ListDevices(ctx context.Context, status DeviceStatus) ([]Device, error) {
	var devices []Device

	if err := p.db.SelectContext(ctx, &devices, p.Queries.ListDevices, status); err != nil {
		return nil, fmt.Errorf("failed to list devices: %w", err)
	}

	return devices, nil
}

func (p *SQLProvider) UpdateDeviceStatus(ctx context.Context, deviceID string, status DeviceStatus, approvedBy *string) error {
	result, err := p.db.ExecContext(ctx, p.Queries.UpdateDeviceStatus, status, time.Now(), approvedBy, deviceID)
	if err != nil {
		return fmt.Errorf("failed to update device status: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("device not found: %s", deviceID)
	}

	p.logger.Debug("Device status updated", "device_id", deviceID, "status", status, "approved_by", approvedBy)

	return nil
}

// --- Approved device methods ---
func (p *SQLProvider) CreateApprovedDevice(ctx context.Context, device ApprovedDevice) error {
	approvedAt := device.ApprovedAt
	if approvedAt.IsZero() {
		approvedAt = time.Now()
	}

	_, err := p.db.ExecContext(ctx, p.Queries.CreateApprovedDevice, device.DeviceID, device.EntryID, device.ApprovedBy, approvedAt)
	if err != nil {
		return fmt.Errorf("failed to create approved device: %w", err)
	}

	p.logger.Debug("Approved device created", "device_id", device.DeviceID, "entry_id", device.EntryID, "approved_by", device.ApprovedBy)

	return nil
}

func (p *SQLProvider) GetApprovedDevice(ctx context.Context, deviceID string, entryID int64) (*ApprovedDevice, error) {
	var device ApprovedDevice

	err := p.db.GetContext(ctx, &device, p.Queries.GetApprovedDevice, deviceID, entryID)
	if err != nil {
		return nil, fmt.Errorf("failed to get approved device: %w", err)
	}

	return &device, nil
}

func (p *SQLProvider) ListApprovedDevicesByDevice(ctx context.Context, deviceID string) ([]ApprovedDevice, error) {
	var devices []ApprovedDevice

	if err := p.db.SelectContext(ctx, &devices, p.Queries.ListApprovedDevicesByDevice, deviceID); err != nil {
		return nil, fmt.Errorf("failed to list approved devices by device: %w", err)
	}

	return devices, nil
}

func (p *SQLProvider) ListApprovedDevicesByEntry(ctx context.Context, entryID int64) ([]ApprovedDevice, error) {
	var devices []ApprovedDevice

	if err := p.db.SelectContext(ctx, &devices, p.Queries.ListApprovedDevicesByEntry, entryID); err != nil {
		return nil, fmt.Errorf("failed to list approved devices by entry: %w", err)
	}

	return devices, nil
}

func (p *SQLProvider) RevokeApprovedDevice(ctx context.Context, deviceID string, entryID int64) error {
	result, err := p.db.ExecContext(ctx, p.Queries.RevokeApprovedDevice, time.Now(), deviceID, entryID)
	if err != nil {
		return fmt.Errorf("failed to revoke approved device: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("approved device not found: device_id=%s, entry_id=%d", deviceID, entryID)
	}

	p.logger.Debug("Approved device revoked", "device_id", deviceID, "entry_id", entryID)

	return nil
}
