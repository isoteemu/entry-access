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

	// Device provisioning methods
	CreateDevice(ctx context.Context, device Device) error
	GetDevice(ctx context.Context, deviceID string) (*Device, error)
	ListDevices(ctx context.Context, status DeviceStatus) ([]Device, error)
	UpdateDeviceStatus(ctx context.Context, deviceID string, status DeviceStatus, approvedBy *string) error

	// Approved device methods
	CreateApprovedDevice(ctx context.Context, device ApprovedDevice) error
	GetApprovedDevice(ctx context.Context, deviceID string, entryID int64) (*ApprovedDevice, error)
	ListApprovedDevicesByDevice(ctx context.Context, deviceID string) ([]ApprovedDevice, error)
	ListApprovedDevicesByEntry(ctx context.Context, entryID int64) ([]ApprovedDevice, error)
	RevokeApprovedDevice(ctx context.Context, deviceID string, entryID int64) error

	// Device maintenance methods
	PruneDevices(ctx context.Context, olderThan time.Time, statusFilter DeviceStatus) (int64, error)
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
