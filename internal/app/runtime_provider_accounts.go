package app

import (
	"context"
	"net/http"
	"strings"

	proxyruntimev1 "github.com/byte-v-forge/common-lib/gen/go/byte/v/forge/contracts/proxyruntime/v1"
)

func (r *Runtime) handleProviderAccounts(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodGet:
		accounts, err := r.store.ListProviderAccounts(req.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		r.writeProto(w, &proxyruntimev1.ListProxyProviderAccountsResponse{Accounts: accounts})
	case http.MethodPost, http.MethodPut:
		var body proxyruntimev1.UpsertProxyProviderAccountRequest
		if !r.readProto(w, req, &body) {
			return
		}
		account, err := r.store.UpsertProviderAccount(req.Context(), &body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		r.writeProto(w, &proxyruntimev1.UpsertProxyProviderAccountResponse{Account: account})
	case http.MethodDelete:
		var body proxyruntimev1.DeleteProxyProviderAccountRequest
		if !r.readProto(w, req, &body) {
			return
		}
		if err := r.store.DeleteProviderAccount(req.Context(), body.GetAccountId()); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		r.releaseLeasesForProviderAccount(req.Context(), body.GetAccountId())
		r.writeProto(w, &proxyruntimev1.DeleteProxyProviderAccountResponse{})
	default:
		methodNotAllowed(w, http.MethodGet+", "+http.MethodPost+", "+http.MethodPut+", "+http.MethodDelete)
	}
}

func (r *Runtime) releaseLeasesForProviderAccount(ctx context.Context, providerAccountID string) {
	providerAccountID = strings.TrimSpace(providerAccountID)
	if providerAccountID == "" || r.leases == nil {
		return
	}
	leases, err := r.leases.ListLeases(ctx, false)
	if err != nil {
		r.logger.Warn("list proxy leases for provider account delete failed", "provider_account_id", providerAccountID, "error", err)
		return
	}
	for _, lease := range leases {
		if lease.GetProviderAccountId() != providerAccountID {
			continue
		}
		if _, err := r.releaseLease(ctx, lease.GetAccountId()); err != nil {
			r.logger.Warn("release proxy lease for deleted provider account failed", "provider_account_id", providerAccountID, "account_id", lease.GetAccountId(), "error", err)
		}
	}
}
