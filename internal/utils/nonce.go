package utils

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	. "entry-access-control/internal/config"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

var NonceStore NonceStoreInterface

// Number of random bytes. 16 → 128‑bit
const NONCE_SIZE = 16

type NonceStoreType string

// Supported nonce stores.
const (
	Memory NonceStoreType = "memory"
	// Redis  NonceStoreType = "redis"
	// SQL    NonceStoreType = "sql"
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
	if err := NonceStore.Put(ctx, nonce, nonceTTL); err != nil {
		slog.Error("failed to store nonce", "error", err)
	}
	return nonce, nil
}

// NewStore builds the appropriate Store implementation based on cfg.
func NewStore(cfg *Config) (NonceStoreInterface, error) {
	switch cfg.NonceStore {
	case "memory":
		return NewMemoryStore(), nil
	// case "redis":
	//     rdb := redis.NewClient(&redis.Options{
	//         Addr:     cfg.RedisAddr,
	//         Password: cfg.RedisPassword,
	//         DB:       cfg.RedisDB,
	//     })
	//     // Ping once to verify connectivity.
	//     if err := rdb.Ping(context.Background()).Err(); err != nil {
	//         return nil, fmt.Errorf("redis ping failed: %w", err)
	//     }
	//     return NewRedisStore(rdb), nil
	default:
		return nil, fmt.Errorf("unknown store type %q", cfg.NonceStore)
	}
}

// ---------------------------------------------------------------------------
// / In-memory implementation
// ---------------------------------------------------------------------------

// MemoryStore holds nonces in a map protected by a RWMutex.
// Expiration is handled by a background janitor goroutine.
type MemoryStore struct {
	mu      sync.RWMutex
	entries map[string]time.Time // value = expiry timestamp
	stop    chan struct{}
}

func NewMemoryStore() *MemoryStore {
	ms := &MemoryStore{
		entries: make(map[string]time.Time),
		stop:    make(chan struct{}),
	}
	go ms.janitor()
	return ms
}

func (m *MemoryStore) Put(ctx context.Context, nonce string, ttl time.Duration) error {
	m.mu.Lock()
	if ttl <= 0 {
		return errors.New("ttl must be > 0")
	}
	defer m.mu.Unlock()
	m.entries[nonce] = time.Now().Add(ttl)
	return nil
}

func (m *MemoryStore) Consume(ctx context.Context, nonce string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	exp, ok := m.entries[nonce]
	if !ok {
		return false, &NonceMissingError{Nonce: nonce}
	}
	if time.Now().After(exp) {
		delete(m.entries, nonce)
		return false, &NonceExpiredError{Nonce: nonce, Expiry: exp}
	}
	delete(m.entries, nonce)
	return true, nil
}

func (m *MemoryStore) Exists(ctx context.Context, nonce string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	exp, ok := m.entries[nonce]
	if !ok {
		return false
	}
	if time.Now().After(exp) {
		return false
	}
	return true
}

// janitor runs every second (configurable) and purges expired keys.
func (m *MemoryStore) janitor() {
	// Skew is x2 to allow safe margin
	ticker := time.NewTicker(time.Duration(float64(Cfg.TokenExpirySkew)*2.0) * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			now := time.Now()
			m.mu.Lock()
			for k, exp := range m.entries {
				if now.After(exp) {
					slog.Debug("Purging expired nonce", "nonce", k)
					delete(m.entries, k)
				}
			}
			m.mu.Unlock()
		case <-m.stop:
			return
		}
	}
}

// Close stops the janitor
func (m *MemoryStore) Close() {
	close(m.stop)
}

func InitNonceStore(cfg *Config) error {
	store, err := NewStore(cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize nonce store: %w", err)
	}
	NonceStore = store
	slog.Info("Initialized nonce store", "type", cfg.NonceStore)
	return nil
}
