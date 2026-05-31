package app

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"time"

	proxyruntimev1 "github.com/byte-v-forge/common-lib/gen/go/byte/v/forge/contracts/proxyruntime/v1"
	"github.com/byte-v-forge/proxy-runtime/internal/dataplane"
	"github.com/byte-v-forge/proxy-runtime/internal/provider"
	"github.com/byte-v-forge/proxy-runtime/internal/provider/accountproxy"
)

func (r *Runtime) restoreActiveLeases(ctx context.Context, staticChain []*url.URL, poolNodes []provider.Node) {
	if r.leases == nil {
		return
	}
	leases, err := r.leases.ListLeases(ctx, false)
	if err != nil {
		r.logger.Warn("list proxy leases for restore failed", "error", err)
		return
	}
	now := time.Now().UTC()
	for _, lease := range leases {
		if !leaseActive(lease, now) {
			if lease.GetStatus() == proxyruntimev1.ProxyDynamicLeaseStatus_PROXY_DYNAMIC_LEASE_STATUS_ACTIVE {
				lease.Status = proxyruntimev1.ProxyDynamicLeaseStatus_PROXY_DYNAMIC_LEASE_STATUS_EXPIRED
				_ = r.leases.SaveLease(ctx, lease)
			}
			continue
		}
		if err := r.restoreLeaseRoute(ctx, lease, staticChain, poolNodes); err != nil {
			r.logger.Warn("restore proxy lease route failed", "account_id", lease.GetAccountId(), "error", err)
		}
	}
}

func (r *Runtime) restoreLeaseRoute(ctx context.Context, lease *proxyruntimev1.ProxyDynamicLease, staticChain []*url.URL, poolNodes []provider.Node) error {
	if lease.GetSession() == nil || lease.GetListener() == nil {
		return errors.New("lease session or listener is missing")
	}
	providerCfg, _, err := r.store.ProviderConfig(ctx, lease.GetProviderAccountId())
	if err != nil {
		return err
	}
	settings, err := r.settings.load()
	if err != nil {
		return err
	}
	providerCfg.Gateways = gatewaysForPlan(settings, lease.GetChainPlan(), providerCfg.ProviderID)
	providerClient, err := accountproxy.New(providerCfg, buildRuntimeHTTPClient(r.cfg))
	if err != nil {
		return err
	}
	nodes, err := providerClient.Fetch(ctx, lease.GetSession())
	if err != nil {
		return err
	}
	rawLineNode := r.sourceRuntimeNodeForLine(poolNodes, lease.GetChainPlan().GetLine())
	if lease.GetChainPlan().GetLine() != nil && rawLineNode == nil {
		return fmt.Errorf("selected line proxy listener is not available: %s/%s", lease.GetChainPlan().GetLine().GetSourceId(), lease.GetChainPlan().GetLine().GetNodeId())
	}
	lineNode := lineNodeForPlan(lease.GetChainPlan(), rawLineNode)
	route := dataplane.SessionRoute{
		SessionID:   lease.GetSession().GetSessionId(),
		ChainID:     leaseChainID(lease.GetAccountId()),
		Listener:    localServiceFromListener(listenerFromProto(lease.GetListener()), r.cfg.LocalProtocol),
		StaticChain: routeStaticChain(staticChain, lineNode),
		Pool:        nodes,
	}
	return r.routePlane.UpsertSessionRoute(ctx, route)
}
