CREATE TABLE IF NOT EXISTS migrations (
    id INTEGER PRIMARY KEY,
    applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    version_before INTEGER NOT NULL,
    version_after INTEGER NOT NULL,

    application_version TEXT NOT NULL

);
-- Index to quickly find the latest migration applied
CREATE INDEX IF NOT EXISTS idx_migrations_applied_at ON migrations (applied_at DESC);

CREATE TABLE IF NOT EXISTS entryways (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP DEFAULT NULL,
    UNIQUE(name)
);

CREATE TABLE IF NOT EXISTS nonces (
    nonce TEXT PRIMARY KEY,
    expires_at TIMESTAMP NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_nonces_expires_at ON nonces (expires_at);
