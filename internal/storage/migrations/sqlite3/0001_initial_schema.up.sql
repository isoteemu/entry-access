CREATE TABLE IF NOT EXISTS migrations (
    id INTEGER PRIMARY KEY,
    applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    version_before INTEGER NOT NULL,
    version_after INTEGER NOT NULL,

    application_version TEXT NOT NULL

);
-- Index to quickly find the latest migration applied
CREATE INDEX IF NOT EXISTS idx_migrations_applied_at ON migrations (applied_at DESC);

CREATE TABLE IF NOT EXISTS entries (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    calendar_url TEXT DEFAULT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP DEFAULT NULL,
    UNIQUE(name)
);

CREATE TABLE IF NOT EXISTS nonces (
    nonce TEXT PRIMARY KEY,
    expires_at TIMESTAMP NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_nonces_expires_at ON nonces (expires_at);

-- Create table for device provisioning
CREATE TABLE IF NOT EXISTS devices (
    device_id TEXT PRIMARY KEY,
    client_ip TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    status TEXT DEFAULT 'pending' CHECK(status IN ('pending', 'approved', 'rejected')),
    approved_by TEXT DEFAULT NULL
);

-- Index to quickly find devices by status
CREATE INDEX IF NOT EXISTS idx_devices_status ON devices (status);
CREATE INDEX IF NOT EXISTS idx_devices_created_at ON devices (created_at DESC);

-- Create table for approved devices with entry associations
CREATE TABLE IF NOT EXISTS approved_devices (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    device_id TEXT NOT NULL,
    entry_id INTEGER NOT NULL,
    approved_by TEXT NOT NULL,
    approved_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    revoked_at TIMESTAMP DEFAULT NULL,
    
    FOREIGN KEY (device_id) REFERENCES devices(device_id) ON DELETE CASCADE,
    FOREIGN KEY (entry_id) REFERENCES entries(id) ON DELETE CASCADE,
    
    UNIQUE(device_id, entry_id)
);

-- Index for quick lookups by device_id
CREATE INDEX IF NOT EXISTS idx_approved_devices_device_id ON approved_devices (device_id);

-- Index for quick lookups by entry_id
CREATE INDEX IF NOT EXISTS idx_approved_devices_entry_id ON approved_devices (entry_id);

-- Index for active (non-revoked) devices
CREATE INDEX IF NOT EXISTS idx_approved_devices_active ON approved_devices (device_id, entry_id) WHERE revoked_at IS NULL;
