package app

import (
	"context"
	"errors"
	"strings"
)

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
