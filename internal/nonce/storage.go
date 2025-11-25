package nonce

import (
	"context"
	"entry-access-control/internal/config"
	"entry-access-control/internal/storage"
	"log/slog"
	"time"
)

// ---------------------------------------------------------------------------
// SQL implementation
// ---------------------------------------------------------------------------

type SQLNonceStore struct {
	logger  *slog.Logger
	storage storage.Provider

	stop chan struct{}
}

// NewSQLNonceStore creates a new SQLNonceStore.
// Warning: storage.Provider must be set separately after creation.
func NewSQLNonceStore(cfg *config.Config) *SQLNonceStore {
	return &SQLNonceStore{
		logger: slog.With("component", "SQLNonceStore"),
		stop:   make(chan struct{}),
	}
}

func (s *SQLNonceStore) Put(ctx context.Context, nonce string, ttl time.Duration) error {
	expiry := time.Now().Add(ttl)
	return s.storage.CreateNonce(ctx, nonce, expiry)
}

func (s *SQLNonceStore) Consume(ctx context.Context, nonce string) (bool, error) {
	exists, err := s.storage.ConsumeNonce(ctx, nonce)
	if err != nil {
		return false, err
	}
	if !exists {
		return false, &NonceMissingError{Nonce: nonce}
	}
	return true, nil
}

func (s *SQLNonceStore) Exists(ctx context.Context, nonce string) bool {
	exists, err := s.storage.ExistsNonce(ctx, nonce)
	if err != nil {
		s.logger.Error("Failed to check nonce existence", "error", err)
		return false
	}
	return exists
}

func (s *SQLNonceStore) ExpireNonces(ctx context.Context) error {
	now := time.Now()
	return s.storage.ExpireNonces(ctx, now)
}

func (s *SQLNonceStore) janitor() {
	ticker := time.NewTicker(time.Duration(float64(config.Cfg.TokenExpirySkew)*2.0) * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if err := s.ExpireNonces(context.Background()); err != nil {
				s.logger.Error("Failed to expire nonces", "error", err)
			}
		case <-s.stop:
			// Stop the janitor
			return
		}
	}
}

func (s *SQLNonceStore) Close() {
	close(s.stop)
}
