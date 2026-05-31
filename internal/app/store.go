package app

import (
	"context"
	"log/slog"
	"time"

	"github.com/byte-v-forge/proxy-runtime/internal/config"
	"github.com/byte-v-forge/proxy-runtime/internal/secretbox"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresStore struct {
	pool   *pgxpool.Pool
	box    secretbox.Box
	logger *slog.Logger
}

type providerCredential struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type providerAccountRecord struct {
	AccountID        string
	ProviderID       string
	DisplayName      string
	Enabled          bool
	CredentialSecret string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func NewPostgresStore(ctx context.Context, cfg config.Config, logger *slog.Logger) (*PostgresStore, error) {
	box, err := secretbox.New(cfg.EncryptionKey)
	if err != nil {
		return nil, err
	}
	pool, err := pgxpool.New(ctx, cfg.PostgresDSN)
	if err != nil {
		return nil, err
	}
	store := &PostgresStore{pool: pool, box: box, logger: logger}
	if cfg.ApplyMigrations {
		if err := store.applyMigrations(ctx); err != nil {
			pool.Close()
			return nil, err
		}
	}
	if err := store.seedFromConfig(ctx, cfg); err != nil {
		pool.Close()
		return nil, err
	}
	return store, nil
}

func (s *PostgresStore) Close() {
	if s != nil && s.pool != nil {
		s.pool.Close()
	}
}
