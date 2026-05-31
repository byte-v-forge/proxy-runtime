package app

import (
	"context"
	"time"

	proxyruntimev1 "github.com/byte-v-forge/common-lib/gen/go/byte/v/forge/contracts/proxyruntime/v1"
	"github.com/byte-v-forge/common-lib/redisx"
	"github.com/byte-v-forge/proxy-runtime/internal/config"
)

const (
	leaseCachePrefix = "byte-v-forge:proxy-runtime:leases"
	leaseIndexKey    = "__index__"
	defaultLeaseTTL  = 10 * time.Minute
	leaseLockTTL     = 2 * time.Minute
)

type leaseStore interface {
	Close() error
	SaveLease(ctx context.Context, lease *proxyruntimev1.ProxyDynamicLease) error
	ListLeases(ctx context.Context, includeInactive bool) ([]*proxyruntimev1.ProxyDynamicLease, error)
	ActiveLease(ctx context.Context, accountID string) (*proxyruntimev1.ProxyDynamicLease, error)
	DeleteLease(ctx context.Context, accountID string) error
	LockAccount(ctx context.Context, accountID string) (leaseLock, error)
	LockProviderAccount(ctx context.Context, providerAccountID string) (leaseLock, error)
}

type leaseLock interface {
	Unlock(context.Context) error
}

type redisLeaseStore struct {
	closer interface{ Close() error }
	store  *redisx.StringStore
	locker *redisx.BestEffortLocker
}

func NewLeaseStore(ctx context.Context, cfg config.Config) (leaseStore, error) {
	return newRedisLeaseStore(ctx, cfg)
}

func newRedisLeaseStore(ctx context.Context, cfg config.Config) (*redisLeaseStore, error) {
	client, err := redisx.NewRequiredClient(ctx, cfg.RedisURL, "PLATFORM_REDIS_URL is required")
	if err != nil {
		return nil, err
	}
	return &redisLeaseStore{closer: client, store: redisx.NewStringStore(client, leaseCachePrefix, 0), locker: redisx.NewBestEffortLocker(client, leaseCachePrefix+":locks", leaseLockTTL, 100*time.Millisecond)}, nil
}

func (s *redisLeaseStore) Close() error {
	if s == nil || s.closer == nil {
		return nil
	}
	return s.closer.Close()
}
