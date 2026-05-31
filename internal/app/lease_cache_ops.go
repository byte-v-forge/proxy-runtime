package app

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	proxyruntimev1 "github.com/byte-v-forge/common-lib/gen/go/byte/v/forge/contracts/proxyruntime/v1"
	"github.com/byte-v-forge/common-lib/protojsonx"
)

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
