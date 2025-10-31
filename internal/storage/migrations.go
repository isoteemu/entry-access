// Package storage provides a simple, embedded-file based schema migration system.
//
// The implementation expects migration SQL files to be embedded via the Go 1.16+ embed.FS
// under the "migrations" directory. Migrations are discovered and parsed from a driver-
// specific subdirectory.
//
// Migration file naming and format
//   - Filenames must match the pattern: NNNN_name.up.sql or NNNN_name.down.sql
//     (regex: ^(?P<Version>\d{4})\_(?P<Name>[^.]+)\.(?P<Direction>(up|down))\.sql$).
//   - Version is a four-digit integer (e.g. 0001, 0002).
//   - Direction is either "up" (apply) or "down" (rollback).
//   - Each file contains raw SQL that will be applied to the database when that
//     migration is executed.
//
// Usage notes
//   - Migrations are loaded from the embedded files at runtime, so adding or removing
//     migration files requires rebuilding the binary.

// Heavily influenced by Authelia's migration system https://github.com/authelia/authelia/blob/master/internal/storage/migrations.go

package storage

import (
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
)

//go:embed migrations/**/*.sql
var migrationsFS embed.FS

var reMigrationFilename = regexp.MustCompile(`^(?P<Version>\d{4})\_(?P<Name>[^.]+)\.(?P<Direction>(up|down))\.sql$`)

var (
	ErrMigrateCurrentVersionSameAsTarget = errors.New("current version is the same as target version")
)

// SchemaMigration represents a single database migration
type SchemaMigration struct {
	Version int
	Name    string
	Up      bool
	SQL     string
}

func (m *SchemaMigration) Before() int {
	if m.Up {
		return m.Version - 1
	}
	return m.Version
}

func (m *SchemaMigration) After() int {
	if m.Up {
		return m.Version
	}
	return m.Version - 1
}

// MigrationRunner handles database migrations
type MigrationRunner struct {
	db         *sql.DB
	driver     string
	migrations []SchemaMigration
	logger     *slog.Logger
}

// NewMigrationRunner creates a new migration runner
func NewMigrationRunner(driver string) *MigrationRunner {
	logger := slog.With("component", "migrations", "driver", driver)

	return &MigrationRunner{
		driver:     driver,
		migrations: []SchemaMigration{},
		logger:     logger,
	}
}

// ...existing code...

// GetLatestMigrationVersion scans migration files and returns the highest version number
func (mr *MigrationRunner) GetLatestMigrationVersion() (int, error) {
	var dirPath string

	switch mr.driver {
	case "sqlite3":
		dirPath = "migrations/sqlite3"
	default:
		return -1, fmt.Errorf("unsupported driver: %s", mr.driver)
	}

	entries, err := migrationsFS.ReadDir(dirPath)
	if err != nil {
		return -1, fmt.Errorf("failed to read migration directory: %w", err)
	}

	latestVersion := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		filename := entry.Name()
		migration, err := mr.parseMigrationFile(migrationsFS, filepath.Join(dirPath, filename))
		if err != nil {
			continue
		}

		// Only consider "up" migrations
		if !migration.Up {
			continue
		}

		if migration.Version > latestVersion {
			latestVersion = migration.Version
		}
	}

	return latestVersion, nil
}

// LoadMigrations loads migrations from embedded filesystem
// If the target version is -1 this indicates the latest version. If the target version is 0 this indicates the database zero state.
func (mr *MigrationRunner) LoadMigrations(prior int, target int) (any, error) {
	// Resolve -1 to latest version
	if target == -1 {
		latestVersion, err := mr.GetLatestMigrationVersion()
		if err != nil {
			return nil, fmt.Errorf("failed to get latest migration version: %w", err)
		}
		target = latestVersion
		mr.logger.Info("Target version set to latest", "version", target)
	}

	if prior == target {
		return nil, ErrMigrateCurrentVersionSameAsTarget
	}

	var dirPath string

	switch mr.driver {
	case "sqlite3":
		dirPath = "migrations/sqlite3"
	default:
		return nil, fmt.Errorf("unsupported driver: %s", mr.driver)
	}

	entries, err := migrationsFS.ReadDir(dirPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read migration directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		filename := entry.Name()
		migration, err := mr.parseMigrationFile(migrationsFS, filepath.Join(dirPath, filename))
		if err != nil {
			mr.logger.Warn("Failed to parse migration file", "file", filename, "error", err)
			continue
		}

		if mr.skipMigration(migration, prior, target) {
			mr.logger.Debug("Skipping migration", "version", migration.Version, "name", migration.Name, "up", migration.Up)
			continue
		}

		mr.migrations = append(mr.migrations, migration)
	}

	if prior < target {
		// Sort migrations by version
		sort.Slice(mr.migrations, func(i, j int) bool {
			return mr.migrations[i].Version < mr.migrations[j].Version
		})
	} else {
		// Sort migrations by version descending
		sort.Slice(mr.migrations, func(i, j int) bool {
			return mr.migrations[i].Version > mr.migrations[j].Version
		})
	}

	mr.logger.Info("Loaded migrations", "count", len(mr.migrations), "from_version", prior, "to_version", target)
	return mr.migrations, nil
}

func (mr *MigrationRunner) skipMigration(migration SchemaMigration, currentVersion int, targetVersion int) bool {
	doUp := targetVersion == -1 || targetVersion > currentVersion
	if doUp {
		// Skip if not up migration
		if !migration.Up {
			return true
		}

		// Skip if the migration version is greater than the target or less than or equal to the previous version.
		if migration.Version > targetVersion || migration.Version <= currentVersion {
			return true
		}
	} else {
		if migration.Up {
			return true
		}

		// Skip the migration if we want to go down and the migration version is less than or equal to the target
		// or greater than the previous version.
		if migration.Version <= targetVersion || migration.Version > currentVersion {
			return true
		}
	}

	return false
}

// parseMigrationFile parses a migration filename and reads its content
// Expected format: NNNN_description.up.sql or NNNN_description.down.sql
func (mr *MigrationRunner) parseMigrationFile(fs embed.FS, path string) (SchemaMigration, error) {

	// Get filename
	filename := filepath.Base(path)
	if !reMigrationFilename.MatchString(filename) {
		return SchemaMigration{}, fmt.Errorf("invalid migration filename: %s", filename)
	}

	filenameParts := reMigrationFilename.FindStringSubmatch(filename)
	if len(filenameParts) != 5 {
		return SchemaMigration{}, fmt.Errorf("invalid migration filename format: %s, parts: %v", filename, filenameParts)
	}

	sql, err := migrationsFS.ReadFile(path)
	if err != nil {
		return SchemaMigration{}, fmt.Errorf("failed to read migration file: %w", err)
	}

	version, _ := strconv.Atoi(filenameParts[reMigrationFilename.SubexpIndex("Version")])
	migration := SchemaMigration{
		Version: version,
		Name:    filenameParts[reMigrationFilename.SubexpIndex("Name")],
		Up:      filenameParts[reMigrationFilename.SubexpIndex("Direction")] == "up",
		SQL:     string(sql),
	}

	return migration, nil
}
