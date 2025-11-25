package nonce

import (
	"context"
	"entry-access-control/internal/config"
	"errors"
	"log/slog"
	"sync"
	"time"
)

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

func (m *MemoryStore) ExpireNonces(ctx context.Context) error {
	now := time.Now()
	m.mu.Lock()
	defer m.mu.Unlock()
	for k, exp := range m.entries {
		if now.After(exp) {
			slog.Debug("Pruning expired nonce", "nonce", k)
			delete(m.entries, k)
		}
	}
	return nil
}

// janitor runs every second (configurable) and purges expired keys.
func (m *MemoryStore) janitor() {
	// Skew is x2 to allow safe margin
	ticker := time.NewTicker(time.Duration(float64(config.Cfg.TokenExpirySkew)*2.0) * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			m.ExpireNonces(context.Background())
		case <-m.stop:
			return
		}
	}
}

// Close stops the janitor
func (m *MemoryStore) Close() {
	close(m.stop)
}
