package nonce

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"entry-access-control/internal/config"
	"entry-access-control/internal/storage"
	"fmt"
	"log/slog"
	"time"
)

var Store NonceStoreInterface

// Number of random bytes. 16 → 128‑bit
const NONCE_SIZE = 16

type NonceStoreType string

// Supported nonce stores.
const (
	Memory NonceStoreType = "memory"
	SQL    NonceStoreType = "sql"
	// Redis  NonceStoreType = "redis"
)

type NonceMissingError struct {
	Nonce string
}

// Error implements the error interface.
func (e *NonceMissingError) Error() string {
	return fmt.Sprintf("nonce not found: %s", e.Nonce)
}

type NonceExpiredError struct {
	Nonce  string
	Expiry time.Time
}

// Error implements the error interface.
func (e *NonceExpiredError) Error() string {
	return fmt.Sprintf("nonce expired: %s (expiry: %s)", e.Nonce, e.Expiry)
}

type NonceStoreInterface interface {
	// stores a nonce with a TTL.
	Put(ctx context.Context, nonce string, ttl time.Duration) error
	// verifies and deletes the nonce.
	// Returns true if the nonce existed (valid request), false otherwise.
	Consume(ctx context.Context, nonce string) (bool, error)

	Exists(ctx context.Context, nonce string) bool

	ExpireNonces(ctx context.Context) error
}

func generateNonceToken() (string, error) {
	size := NONCE_SIZE
	if size <= 0 {
		fmt.Println("Invalid nonce size, using default 16 bytes")
		size = 16
	}
	b := make([]byte, size)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// Creates a new nonce, stores it in the nonce store, and returns it.
func Nonce(ttl uint) (string, error) {
	nonce, err := generateNonceToken()
	if err != nil {
		return "", err
	}

	nonceTTL := time.Duration(ttl) * time.Second

	ctx := context.Background()
	if err := Store.Put(ctx, nonce, nonceTTL); err != nil {
		slog.Error("failed to store nonce", "error", err)
	}
	return nonce, nil
}

// NewStore builds the appropriate Store implementation based on cfg.
func NewStore(cfg *config.Config) (NonceStoreInterface, error) {
	switch cfg.NonceStore {
	case "memory":
		return NewMemoryStore(), nil
	case "sql":
		return NewSQLNonceStore(cfg), nil
	default:
		return nil, fmt.Errorf("unknown store type %q", cfg.NonceStore)
	}
}

func InitNonceStore(cfg *config.Config, storageProvider storage.Provider) error {
	store, err := NewStore(cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize nonce store: %w", err)
	}

	// If SQL store, set the storage provider
	switch s := store.(type) {
	case *SQLNonceStore:
		s.storage = storageProvider
		go s.janitor()
	case *MemoryStore:
		go s.janitor()
	}

	// Make the store globally accessible
	Store = store

	slog.Info("Initialized nonce store", "type", cfg.NonceStore)
	return nil
}
