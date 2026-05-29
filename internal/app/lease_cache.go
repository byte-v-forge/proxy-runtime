package app

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	proxyruntimev1 "github.com/byte-v-forge/common-lib/gen/go/byte/v/forge/contracts/proxyruntime/v1"
	"github.com/byte-v-forge/common-lib/protojsonx"
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

func (s *redisLeaseStore) SaveLease(ctx context.Context, lease *proxyruntimev1.ProxyDynamicLease) error {
	if lease == nil || strings.TrimSpace(lease.GetAccountId()) == "" {
		return errors.New("lease account_id is required")
	}
	if lease.GetStatus() != proxyruntimev1.ProxyDynamicLeaseStatus_PROXY_DYNAMIC_LEASE_STATUS_ACTIVE {
		return s.DeleteLease(ctx, lease.GetAccountId())
	}
	data, err := protojsonx.Marshal(lease)
	if err != nil {
		return err
	}
	accountID := strings.TrimSpace(lease.GetAccountId())
	if err := s.store.SaveTTL(ctx, leaseKey(accountID), string(data), leaseTTL(lease)); err != nil {
		return err
	}
	return s.saveIndex(ctx, appendLeaseIndex(s.loadIndex(ctx), accountID))
}

func (s *redisLeaseStore) ListLeases(ctx context.Context, _ bool) ([]*proxyruntimev1.ProxyDynamicLease, error) {
	ids := s.loadIndex(ctx)
	out := make([]*proxyruntimev1.ProxyDynamicLease, 0, len(ids))
	activeIDs := make([]string, 0, len(ids))
	now := time.Now().UTC()
	for _, id := range ids {
		lease, err := s.loadLease(ctx, id)
		if errors.Is(err, errLeaseNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if !leaseActive(lease, now) {
			continue
		}
		out = append(out, lease)
		activeIDs = append(activeIDs, id)
	}
	if len(activeIDs) != len(ids) {
		_ = s.saveIndex(ctx, activeIDs)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].GetAcquiredAt().AsTime().After(out[j].GetAcquiredAt().AsTime()) })
	return out, nil
}

func (s *redisLeaseStore) ActiveLease(ctx context.Context, accountID string) (*proxyruntimev1.ProxyDynamicLease, error) {
	lease, err := s.loadLease(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if !leaseActive(lease, time.Now().UTC()) {
		_ = s.DeleteLease(ctx, accountID)
		return nil, errLeaseNotFound
	}
	return lease, nil
}

func (s *redisLeaseStore) DeleteLease(ctx context.Context, accountID string) error {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return nil
	}
	if err := s.store.Delete(ctx, leaseKey(accountID)); err != nil {
		return err
	}
	return s.saveIndex(ctx, removeLeaseIndex(s.loadIndex(ctx), accountID))
}

func (s *redisLeaseStore) LockAccount(ctx context.Context, accountID string) (leaseLock, error) {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return nil, errors.New("lease account_id is required")
	}
	return s.locker.Lock(ctx, "account:"+accountID)
}

func (s *redisLeaseStore) LockProviderAccount(ctx context.Context, providerAccountID string) (leaseLock, error) {
	providerAccountID = strings.TrimSpace(providerAccountID)
	if providerAccountID == "" {
		return nil, errors.New("provider account id is required")
	}
	return s.locker.Lock(ctx, "provider-account:"+providerAccountID)
}

func (s *redisLeaseStore) loadLease(ctx context.Context, accountID string) (*proxyruntimev1.ProxyDynamicLease, error) {
	value, ok, err := s.store.Load(ctx, leaseKey(accountID))
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errLeaseNotFound
	}
	lease := &proxyruntimev1.ProxyDynamicLease{}
	if err := protojsonx.Unmarshal([]byte(value), lease); err != nil {
		return nil, err
	}
	return lease, nil
}

func (s *redisLeaseStore) loadIndex(ctx context.Context) []string {
	value, ok, err := s.store.Load(ctx, leaseIndexKey)
	if err != nil || !ok {
		return nil
	}
	var ids []string
	_ = json.Unmarshal([]byte(value), &ids)
	return cleanIDs(ids)
}

func (s *redisLeaseStore) saveIndex(ctx context.Context, ids []string) error {
	data, err := json.Marshal(cleanIDs(ids))
	if err != nil {
		return err
	}
	return s.store.SaveTTL(ctx, leaseIndexKey, string(data), 0)
}

var errLeaseNotFound = errors.New("active proxy lease not found")

func leaseKey(accountID string) string { return "lease:" + strings.TrimSpace(accountID) }

func leaseTTL(lease *proxyruntimev1.ProxyDynamicLease) time.Duration {
	if lease.GetExpiresAt() == nil {
		return defaultLeaseTTL
	}
	ttl := time.Until(lease.GetExpiresAt().AsTime())
	if ttl <= 0 {
		return time.Second
	}
	return ttl
}

func appendLeaseIndex(ids []string, id string) []string {
	ids = append(ids, id)
	return cleanIDs(ids)
}

func removeLeaseIndex(ids []string, id string) []string {
	id = strings.TrimSpace(id)
	out := make([]string, 0, len(ids))
	for _, item := range ids {
		if strings.TrimSpace(item) != "" && strings.TrimSpace(item) != id {
			out = append(out, item)
		}
	}
	return cleanIDs(out)
}

func cleanIDs(ids []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}
