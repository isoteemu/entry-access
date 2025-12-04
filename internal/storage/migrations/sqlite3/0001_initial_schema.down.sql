-- Rollback initial schema
DROP TABLE IF EXISTS migrations;
DROP INDEX IF EXISTS idx_migrations_applied_at;

DROP TABLE IF EXISTS nonces;
DROP INDEX IF EXISTS idx_nonces_expires_at

DROP TABLE IF EXISTS entries;

-- Drop approved_devices table
DROP INDEX IF EXISTS idx_approved_devices_active;
DROP INDEX IF EXISTS idx_approved_devices_entry_id;
DROP INDEX IF EXISTS idx_approved_devices_device_id;
DROP TABLE IF EXISTS approved_devices;

-- Drop devices table
DROP INDEX IF EXISTS idx_devices_created_at;
DROP INDEX IF EXISTS idx_devices_status;
DROP TABLE IF EXISTS devices;
