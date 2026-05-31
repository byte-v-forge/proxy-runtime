package app

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	proxyruntimev1 "github.com/byte-v-forge/common-lib/gen/go/byte/v/forge/contracts/proxyruntime/v1"
	"github.com/byte-v-forge/common-lib/randx"
	"github.com/byte-v-forge/proxy-runtime/internal/dataplane"
	"github.com/byte-v-forge/proxy-runtime/internal/provider/accountproxy"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (r *Runtime) acquireLease(ctx context.Context, httpReq *http.Request, req *proxyruntimev1.AcquireProxyLeaseRequest) (*proxyruntimev1.ProxyDynamicLease, error) {
	req.AccountId = strings.TrimSpace(req.GetAccountId())
	if req.AccountId == "" {
		return nil, errors.New("account_id is required")
	}
	lock, err := r.leases.LockAccount(ctx, req.GetAccountId())
	if err != nil {
		return nil, err
	}
	defer func() { _ = lock.Unlock(ctx) }()
	req.Purpose = firstNonEmpty(req.GetPurpose(), "general")
	if existing, err := r.leases.ActiveLease(ctx, req.GetAccountId()); err == nil && leaseActive(existing, time.Now().UTC()) {
		if !req.GetForceNew() {
			return existing, nil
		}
		if err := r.retireLeaseRoute(ctx, existing); err != nil {
			return nil, err
		}
	}
	planResult, err := r.planProxyChain(ctx, req)
	if err != nil {
		return nil, err
	}
	providerAccountID := planResult.plan.GetDynamicGateway().GetProviderAccountId()
	providerLock, err := r.leases.LockProviderAccount(ctx, providerAccountID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = providerLock.Unlock(ctx) }()
	if r.providerAccountBusy(ctx, providerAccountID) {
		return nil, errors.New("selected provider account is already leased")
	}
	providerCfg, providerAccountID, err := r.store.ProviderConfig(ctx, providerAccountID)
	if err != nil {
		return nil, err
	}
	providerCfg.Gateways = []accountproxy.Gateway{planResult.gateway}
	providerClient, err := accountproxy.New(providerCfg, buildRuntimeHTTPClient(r.cfg))
	if err != nil {
		return nil, err
	}
	normalizeLeasePolicy(req)
	req.Policy.Labels["chain_id"] = planResult.plan.GetChainId()
	req.Policy.Labels["dynamic_gateway_id"] = planResult.plan.GetDynamicGateway().GetGatewayId()
	if planResult.plan.GetLine() != nil {
		req.Policy.Labels["line_source_id"] = planResult.plan.GetLine().GetSourceId()
		req.Policy.Labels["line_node_id"] = planResult.plan.GetLine().GetNodeId()
		if hop := chainHopByRole(planResult.plan, proxyruntimev1.ProxyChainHopRole_PROXY_CHAIN_HOP_ROLE_LINE_PROXY); hop != nil {
			req.Policy.Labels["line_observed_ip"] = hop.GetObservedIp()
		}
	}
	session, err := providerClient.CreateSession(ctx, req)
	if err != nil {
		return nil, err
	}
	nodes, err := providerClient.Fetch(ctx, session)
	if err != nil {
		return nil, err
	}
	listener, err := r.leaseListener(ctx, req.GetAccountId())
	if err != nil {
		return nil, err
	}
	egress, err := r.localListenerEndpoint(listener, r.sessionAdvertisedHost(httpReq, listener))
	if err != nil {
		return nil, err
	}
	egress.ProviderId = providerClient.Name()
	egress.UpstreamKind = proxyruntimev1.ProxyUpstreamKind_PROXY_UPSTREAM_KIND_DYNAMIC_IP
	egress.RotationMode = proxyruntimev1.ProxyRotationMode_PROXY_ROTATION_MODE_STICKY_SESSION
	egress.SessionId = session.GetSessionId()
	egress.Labels["account_id"] = req.GetAccountId()
	egress.Labels["purpose"] = req.GetPurpose()
	egress.Labels["provider_account_id"] = providerAccountID
	egress.Labels["chain_id"] = planResult.plan.GetChainId()
	egress.Labels["dynamic_gateway_id"] = planResult.plan.GetDynamicGateway().GetGatewayId()
	if planResult.plan.GetLine() != nil {
		egress.Labels["line_source_id"] = planResult.plan.GetLine().GetSourceId()
		egress.Labels["line_node_id"] = planResult.plan.GetLine().GetNodeId()
		if hop := chainHopByRole(planResult.plan, proxyruntimev1.ProxyChainHopRole_PROXY_CHAIN_HOP_ROLE_LINE_PROXY); hop != nil {
			egress.Labels["line_observed_ip"] = hop.GetObservedIp()
		}
	}
	session.Egress = egress
	staticChain, err := r.parseStaticChain()
	if err != nil {
		return nil, err
	}
	staticChain = routeStaticChain(staticChain, planResult.lineNode)
	route := dataplane.SessionRoute{SessionID: session.GetSessionId(), ChainID: leaseChainID(req.GetAccountId()), Listener: localServiceFromListener(listener, r.cfg.LocalProtocol), StaticChain: staticChain, Pool: nodes}
	if err := r.routePlane.UpsertSessionRoute(ctx, route); err != nil {
		_ = r.routePlane.DeleteSessionRoute(ctx, route)
		return nil, err
	}
	leaseID, _ := randx.Hex(12)
	now := time.Now().UTC()
	lease := &proxyruntimev1.ProxyDynamicLease{LeaseId: leaseID, AccountId: req.GetAccountId(), Purpose: req.GetPurpose(), ProviderAccountId: providerAccountID, Status: proxyruntimev1.ProxyDynamicLeaseStatus_PROXY_DYNAMIC_LEASE_STATUS_ACTIVE, Session: session, Egress: egress, Listener: protoListener(listener, true), AcquiredAt: timestamppb.New(now), ExpiresAt: session.GetExpiresAt(), ChainPlan: planResult.plan}
	if err := r.leases.SaveLease(ctx, lease); err != nil {
		_ = r.routePlane.DeleteSessionRoute(ctx, route)
		return nil, err
	}
	return lease, nil
}

func (r *Runtime) releaseLease(ctx context.Context, accountID string) (*proxyruntimev1.ProxyDynamicLease, error) {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return nil, errors.New("account_id is required")
	}
	lock, err := r.leases.LockAccount(ctx, accountID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = lock.Unlock(ctx) }()
	lease, err := r.leases.ActiveLease(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if err := r.retireLeaseRoute(ctx, lease); err != nil {
		return nil, err
	}
	return lease, nil
}

func (r *Runtime) retireLeaseRoute(ctx context.Context, lease *proxyruntimev1.ProxyDynamicLease) error {
	if lease == nil {
		return nil
	}
	if lease.GetSession() != nil && lease.GetListener() != nil {
		listener := listenerFromProto(lease.GetListener())
		route := dataplane.SessionRoute{SessionID: lease.GetSession().GetSessionId(), ChainID: leaseChainID(lease.GetAccountId()), Listener: localServiceFromListener(listener, r.cfg.LocalProtocol)}
		if err := r.routePlane.DeleteSessionRoute(ctx, route); err != nil {
			return err
		}
	}
	lease.Status = proxyruntimev1.ProxyDynamicLeaseStatus_PROXY_DYNAMIC_LEASE_STATUS_RELEASED
	return r.leases.SaveLease(ctx, lease)
}

func normalizeLeasePolicy(req *proxyruntimev1.AcquireProxyLeaseRequest) {
	if req.Policy == nil {
		req.Policy = &proxyruntimev1.ProxySessionPolicy{}
	}
	if req.Policy.Mode == proxyruntimev1.ProxySessionMode_PROXY_SESSION_MODE_UNSPECIFIED {
		req.Policy.Mode = proxyruntimev1.ProxySessionMode_PROXY_SESSION_MODE_STICKY
	}
	if req.Policy.UpstreamKind == proxyruntimev1.ProxyUpstreamKind_PROXY_UPSTREAM_KIND_UNSPECIFIED {
		req.Policy.UpstreamKind = proxyruntimev1.ProxyUpstreamKind_PROXY_UPSTREAM_KIND_DYNAMIC_IP
	}
	if req.Policy.RotationMode == proxyruntimev1.ProxyRotationMode_PROXY_ROTATION_MODE_UNSPECIFIED {
		req.Policy.RotationMode = proxyruntimev1.ProxyRotationMode_PROXY_ROTATION_MODE_STICKY_SESSION
	}
	if req.Policy.Labels == nil {
		req.Policy.Labels = map[string]string{}
	}
	req.Policy.Labels["account_id"] = req.GetAccountId()
	req.Policy.Labels["purpose"] = req.GetPurpose()
}

func leaseActive(lease *proxyruntimev1.ProxyDynamicLease, now time.Time) bool {
	if lease == nil || lease.GetStatus() != proxyruntimev1.ProxyDynamicLeaseStatus_PROXY_DYNAMIC_LEASE_STATUS_ACTIVE {
		return false
	}
	return lease.GetExpiresAt() == nil || now.Before(lease.GetExpiresAt().AsTime())
}
