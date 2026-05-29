package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	proxyruntimev1 "github.com/byte-v-forge/common-lib/gen/go/byte/v/forge/contracts/proxyruntime/v1"
	"github.com/byte-v-forge/common-lib/httpx"
	"github.com/byte-v-forge/common-lib/protojsonx"
	"github.com/byte-v-forge/common-lib/proxyurl"
	"github.com/byte-v-forge/common-lib/randx"
	"github.com/byte-v-forge/proxy-runtime/internal/config"
	"github.com/byte-v-forge/proxy-runtime/internal/dataplane"
	"github.com/byte-v-forge/proxy-runtime/internal/provider"
	"github.com/byte-v-forge/proxy-runtime/internal/provider/accountproxy"
	"github.com/byte-v-forge/proxy-runtime/internal/sourceplane"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Runtime struct {
	cfg         config.Config
	provider    provider.Provider
	routePlane  dataplane.Driver
	sourcePlane sourceplane.Driver
	store       *PostgresStore
	leases      leaseStore
	settings    *runtimeSettingsStore
	logger      *slog.Logger

	mu          sync.RWMutex
	pool        []provider.Node
	refreshedAt time.Time

	refreshMu        sync.Mutex
	forwardMu        sync.Mutex
	forwardCancel    context.CancelFunc
	forwardListeners []net.Listener
	forwardSignature string

	fraudMu        sync.Mutex
	fraudSignature string
	fraud          ipFraudChecker

	geoMu    sync.Mutex
	geoCache map[string]cachedIPGeo
}

func NewRuntime(cfg config.Config, proxyProvider provider.Provider, routePlane dataplane.Driver, sourcePlane sourceplane.Driver, store *PostgresStore, leases leaseStore, logger *slog.Logger) *Runtime {
	if logger == nil {
		logger = slog.Default()
	}
	if sourcePlane == nil {
		sourcePlane = sourceplane.Empty{}
	}
	return &Runtime{cfg: cfg, provider: proxyProvider, routePlane: routePlane, sourcePlane: sourcePlane, store: store, leases: leases, settings: newRuntimeSettingsStore(store, logger), logger: logger}
}

func (r *Runtime) Run(ctx context.Context) error {
	if err := r.refresh(ctx); err != nil {
		return err
	}
	defer r.routePlane.Stop()
	defer r.sourcePlane.Stop()
	defer r.stopForwarders()
	errCh := make(chan error, 2)
	go r.refreshLoop(ctx)
	go r.serveHTTP(ctx, errCh)
	select {
	case <-ctx.Done():
		return nil
	case err := <-errCh:
		return err
	}
}

func (r *Runtime) refreshLoop(ctx context.Context) {
	if r.cfg.RefreshInterval == 0 {
		return
	}
	ticker := time.NewTicker(r.cfg.RefreshInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := r.refresh(ctx); err != nil {
				r.logger.Warn("proxy base refresh failed", "error", err)
			}
		}
	}
}

func (r *Runtime) refresh(ctx context.Context) error {
	r.refreshMu.Lock()
	defer r.refreshMu.Unlock()
	nodes, err := r.provider.Fetch(ctx, nil)
	if err != nil && r.cfg.Provider != config.ProviderNone {
		r.logger.Warn("base provider fetch failed", "error", err)
		nodes = nil
	}
	sourceNodes, err := r.sourcePlane.Reconcile(ctx, r.sourcePlaneConfig())
	if err != nil {
		return err
	}
	poolNodes := append(cloneNodes(nodes), cloneNodes(sourceNodes)...)
	staticChain, err := r.parseStaticChain()
	if err != nil {
		return err
	}
	cfg := dataplane.Config{Common: r.commonEgressService(), Local: r.defaultLocalService(), Listeners: r.routePlaneListeners(), StaticChain: staticChain, Pool: poolNodes, DynamicViaCommon: r.cfg.CommonEgressAddr != ""}
	if len(cfg.Listeners) > 0 || cfg.Local.Addr != "" || len(staticChain) > 0 || len(poolNodes) > 0 {
		if err := r.routePlane.ReconcileBase(ctx, cfg); err != nil {
			return err
		}
	}
	if err := r.reloadForwarders(ctx); err != nil {
		return err
	}
	r.restoreActiveLeases(ctx, staticChain, poolNodes)
	r.mu.Lock()
	r.pool = cloneNodes(poolNodes)
	r.refreshedAt = time.Now().UTC()
	r.mu.Unlock()
	r.logger.Info("proxy runtime base refreshed", "pool_size", len(poolNodes), "route_runtime", r.routePlane.Name(), "source_runtime", r.sourcePlane.Name())
	return nil
}

func (r *Runtime) serveHTTP(ctx context.Context, errCh chan<- error) {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", r.handleHealth)
	mux.HandleFunc("/readyz", r.handleReady)
	for _, prefix := range []string{"/proxy", "/api/proxy-runtime"} {
		mux.HandleFunc(prefix+"/providers", r.handleProviders)
		mux.HandleFunc(prefix+"/gateway", r.handleGateway)
		mux.HandleFunc(prefix+"/pool", r.handlePool)
		mux.HandleFunc(prefix+"/refresh", r.handleRefresh)
		mux.HandleFunc(prefix+"/provider-accounts", r.handleProviderAccounts)
		mux.HandleFunc(prefix+"/sources", r.handleSources)
		mux.HandleFunc(prefix+"/sources/fixed", r.handleFixedSources)
		mux.HandleFunc(prefix+"/sources/nodes", r.handleSourceNodes)
		mux.HandleFunc(prefix+"/chains/resolve", r.handleResolveChain)
		mux.HandleFunc(prefix+"/leases", r.handleLeases)
		mux.HandleFunc(prefix+"/leases/acquire", r.handleAcquireLease)
		mux.HandleFunc(prefix+"/leases/release", r.handleReleaseLease)
		mux.HandleFunc(prefix+"/proxy_exit_ip", r.handleGetProxyExitIP)
		mux.HandleFunc(prefix+"/proxy_exit_geo", r.handleGetProxyExitGeo)
		mux.HandleFunc(prefix+"/ip_fraud_check", r.handleCheckIPFraud)
		mux.HandleFunc(prefix+"/check_cf_access_risk", r.handleCheckEdgeAccessRisk)
		mux.HandleFunc(prefix+"/target_connectivity_check", r.handleCheckTargetConnectivity)
		mux.HandleFunc(prefix+"/settings/ip-fraud-providers", r.handleIPFraudProviders)
		mux.HandleFunc(prefix+"/settings", r.handleRuntimeSettings)
	}
	mux.Handle("/mf/proxy-runtime/", http.StripPrefix("/mf/proxy-runtime/", noCacheFileServer(r.cfg.DashboardStaticDir)))
	server := &http.Server{Addr: r.cfg.RuntimeAddr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()
	r.logger.Info("proxy-runtime http listening", "addr", r.cfg.RuntimeAddr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		errCh <- fmt.Errorf("serve proxy-runtime http: %w", err)
	}
}

func (r *Runtime) handleHealth(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (r *Runtime) handleReady(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	status := r.routePlane.Status()
	if !status.Running {
		msg := firstNonEmpty(status.LastError, "route runtime is not running")
		http.Error(w, msg, http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (r *Runtime) handleProviders(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	settings, err := r.settings.load()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	r.writeProto(w, &proxyruntimev1.ListProxyProvidersResponse{Providers: accountproxy.Descriptors(dynamicIPGatewayMap(settings))})
}
func (r *Runtime) handleGateway(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	gateway, err := r.gateway(req.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	r.writeProto(w, &proxyruntimev1.GetEgressGatewayResponse{Gateway: gateway})
}
func (r *Runtime) handlePool(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	pool, err := r.snapshot(req.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	r.writeProto(w, &proxyruntimev1.GetProxyPoolResponse{Pool: pool})
}
func (r *Runtime) handleRefresh(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	if err := r.refresh(req.Context()); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	pool, _ := r.snapshot(req.Context())
	r.writeProto(w, &proxyruntimev1.RefreshProxyPoolResponse{Pool: pool})
}

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

func (r *Runtime) handleSources(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodGet:
		sources, err := r.listSources(req.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		r.writeProto(w, &proxyruntimev1.ListProxySourcesResponse{Sources: sources})
	case http.MethodPost, http.MethodPut:
		var body proxyruntimev1.UpsertProxySubscriptionSourceRequest
		if !r.readProto(w, req, &body) {
			return
		}
		source, err := r.sourcePlane.UpsertSubscriptionSource(req.Context(), &body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		_ = r.refresh(req.Context())
		r.writeProto(w, &proxyruntimev1.UpsertProxySubscriptionSourceResponse{Source: source})
	case http.MethodDelete:
		var body proxyruntimev1.DeleteProxySourceRequest
		if !r.readProto(w, req, &body) {
			return
		}
		if err := r.sourcePlane.DeleteSource(req.Context(), body.GetSourceId()); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		_ = r.refresh(req.Context())
		r.writeProto(w, &proxyruntimev1.DeleteProxySourceResponse{})
	default:
		methodNotAllowed(w, http.MethodGet+", "+http.MethodPost+", "+http.MethodPut+", "+http.MethodDelete)
	}
}

func (r *Runtime) handleFixedSources(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodPost, http.MethodPut:
		var body proxyruntimev1.UpsertProxyFixedSourceRequest
		if !r.readProto(w, req, &body) {
			return
		}
		source, err := r.sourcePlane.UpsertFixedSource(req.Context(), &body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		_ = r.refresh(req.Context())
		r.writeProto(w, &proxyruntimev1.UpsertProxyFixedSourceResponse{Source: source})
	default:
		methodNotAllowed(w, http.MethodPost+", "+http.MethodPut)
	}
}

func (r *Runtime) handleSourceNodes(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	nodes, err := r.sourcePlane.SourceNodes(req.Context(), req.URL.Query().Get("source_id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	r.writeProto(w, &proxyruntimev1.ListProxySourceNodesResponse{Nodes: nodes})
}

func (r *Runtime) handleResolveChain(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	var body proxyruntimev1.ResolveProxyChainRequest
	if !r.readProto(w, req, &body) {
		return
	}
	response, err := r.resolveProxyChain(req.Context(), &body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	r.writeProto(w, response)
}

func (r *Runtime) listSources(ctx context.Context) ([]*proxyruntimev1.ProxySourceDescriptor, error) {
	settings, err := r.settings.load()
	if err != nil {
		return nil, err
	}
	sources, err := r.store.ListSources(ctx, len(r.cfg.StaticChain), dynamicIPGatewayMap(settings))
	if err != nil {
		return nil, err
	}
	sourcePlaneSources, err := r.sourcePlane.Sources(ctx)
	if err != nil {
		return nil, err
	}
	return append(sources, sourcePlaneSources...), nil
}

func (r *Runtime) handleLeases(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	leases, err := r.leases.ListLeases(req.Context(), false)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	r.writeProto(w, &proxyruntimev1.ListProxyDynamicLeasesResponse{Leases: leases})
}
func (r *Runtime) handleAcquireLease(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	var body proxyruntimev1.AcquireProxyLeaseRequest
	if !r.readProto(w, req, &body) {
		return
	}
	lease, err := r.acquireLease(req.Context(), req, &body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	pool, _ := r.snapshot(req.Context())
	r.writeProto(w, &proxyruntimev1.AcquireProxyLeaseResponse{Lease: lease, Pool: pool, Egress: lease.GetEgress(), ChainPlan: lease.GetChainPlan()})
}
func (r *Runtime) handleReleaseLease(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	var body proxyruntimev1.ReleaseProxyLeaseRequest
	if !r.readProto(w, req, &body) {
		return
	}
	lease, err := r.releaseLease(req.Context(), body.GetAccountId())
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	r.writeProto(w, &proxyruntimev1.ReleaseProxyLeaseResponse{Lease: lease})
}

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
	if existing, err := r.leases.ActiveLease(ctx, req.GetAccountId()); err == nil && leaseActive(existing, time.Now().UTC()) && !req.GetForceNew() {
		return existing, nil
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
	listener := listenerFromProto(lease.GetListener())
	route := dataplane.SessionRoute{SessionID: lease.GetSession().GetSessionId(), ChainID: leaseChainID(accountID), Listener: localServiceFromListener(listener, r.cfg.LocalProtocol)}
	if err := r.routePlane.DeleteSessionRoute(ctx, route); err != nil {
		return nil, err
	}
	lease.Status = proxyruntimev1.ProxyDynamicLeaseStatus_PROXY_DYNAMIC_LEASE_STATUS_RELEASED
	if err := r.leases.SaveLease(ctx, lease); err != nil {
		return nil, err
	}
	return lease, nil
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

func (r *Runtime) snapshot(ctx context.Context) (*proxyruntimev1.ProxyPoolSnapshot, error) {
	r.mu.RLock()
	nodes := cloneNodes(r.pool)
	refreshedAt := r.refreshedAt
	r.mu.RUnlock()
	endpoints := make([]*proxyruntimev1.ProxyEndpoint, 0, len(nodes))
	for _, node := range nodes {
		endpoints = append(endpoints, node.Endpoint())
	}
	leases, _ := r.leases.ListLeases(ctx, false)
	for _, lease := range leases {
		if leaseActive(lease, time.Now().UTC()) && lease.GetEgress() != nil {
			endpoints = append(endpoints, lease.GetEgress())
		}
	}
	sources, err := r.listSources(ctx)
	if err != nil {
		return nil, err
	}
	return &proxyruntimev1.ProxyPoolSnapshot{PoolId: "default", ProviderDescriptor: r.provider.Descriptor(), Endpoints: endpoints, RefreshedAt: timestamppb.New(refreshedAt), Sources: sources, DynamicLeases: leases}, nil
}

func (r *Runtime) gateway(ctx context.Context) (*proxyruntimev1.EgressGateway, error) {
	pool, err := r.snapshot(ctx)
	if err != nil {
		return nil, err
	}
	routePlaneRoute, err := r.routePlaneRoute(pool)
	if err != nil {
		return nil, err
	}
	controlRoute, err := r.controlPlaneRoute()
	if err != nil {
		return nil, err
	}
	accounts, _ := r.store.ListProviderAccounts(ctx)
	active := uint32(0)
	for _, lease := range pool.GetDynamicLeases() {
		if leaseActive(lease, time.Now().UTC()) {
			active++
		}
	}
	routeStatus := r.routePlane.Status()
	sourceStatus := r.sourcePlane.Status()
	return &proxyruntimev1.EgressGateway{GatewayId: "default", Listeners: r.protoListeners(pool.GetDynamicLeases()), Pool: pool, DataPlaneRoute: routePlaneRoute, ControlPlaneRoute: controlRoute, ProviderControlPlane: &proxyruntimev1.ProviderControlPlaneAccess{UsesProxy: r.cfg.ProviderHTTPProxy != "", ProxyRef: providerHTTPProxyRef(r.cfg.ProviderHTTPProxy), Protocols: []proxyruntimev1.ProxyProtocol{proxyruntimev1.ProxyProtocol_PROXY_PROTOCOL_HTTP}}, UpdatedAt: timestamppb.Now(), Overview: &proxyruntimev1.ProxyRuntimeOverview{RouteRuntime: routeRuntimeKind(r.routePlane.Name()), RouteRuntimeStatus: statusString(routeStatus), ProviderAccountCount: uint32(len(accounts)), SourceCount: uint32(len(pool.GetSources())), ActiveLeaseCount: active, UpdatedAt: timestamppb.Now(), SourceRuntime: sourceRuntimeKind(r.sourcePlane.Name()), SourceRuntimeStatus: sourceStatusString(r.sourcePlane.Name(), sourceStatus)}}, nil
}

func (r *Runtime) routePlaneRoute(pool *proxyruntimev1.ProxyPoolSnapshot) (*proxyruntimev1.EgressRoute, error) {
	route := &proxyruntimev1.EgressRoute{RouteId: "default-data-plane"}
	chain, err := provider.StaticChainEndpoints(r.cfg.StaticChain)
	if err != nil {
		return nil, err
	}
	for index, endpoint := range chain {
		route.Hops = append(route.Hops, &proxyruntimev1.EgressHop{HopId: fmt.Sprintf("forward-%d", index), Order: uint32(len(route.Hops) + 1), Role: proxyruntimev1.EgressHopRole_EGRESS_HOP_ROLE_FORWARD, Selector: &proxyruntimev1.ProxySelectorPolicy{Strategy: proxyruntimev1.ProxySelectorStrategy_PROXY_SELECTOR_STRATEGY_FIFO}, Endpoints: []*proxyruntimev1.ProxyEndpoint{endpoint}})
	}
	if len(pool.GetEndpoints()) > 0 {
		route.Hops = append(route.Hops, &proxyruntimev1.EgressHop{HopId: "exit", Order: uint32(len(route.Hops) + 1), Role: proxyruntimev1.EgressHopRole_EGRESS_HOP_ROLE_EXIT, Selector: &proxyruntimev1.ProxySelectorPolicy{Strategy: proxyruntimev1.ProxySelectorStrategy_PROXY_SELECTOR_STRATEGY_ROUND_ROBIN}, Endpoints: pool.GetEndpoints()})
	}
	return route, nil
}
func (r *Runtime) controlPlaneRoute() (*proxyruntimev1.EgressRoute, error) {
	if r.cfg.ProviderHTTPProxy == "" {
		return nil, nil
	}
	endpoint, err := proxyurl.Parse(r.cfg.ProviderHTTPProxy, "http")
	if err != nil {
		return nil, err
	}
	return &proxyruntimev1.EgressRoute{RouteId: "provider-control-plane", Hops: []*proxyruntimev1.EgressHop{{HopId: "control-plane-proxy", Order: 1, Role: proxyruntimev1.EgressHopRole_EGRESS_HOP_ROLE_CONTROL_PLANE, Selector: &proxyruntimev1.ProxySelectorPolicy{Strategy: proxyruntimev1.ProxySelectorStrategy_PROXY_SELECTOR_STRATEGY_FIFO}, Endpoints: []*proxyruntimev1.ProxyEndpoint{{Id: "provider-http-proxy", ProviderId: provider.StaticProviderID, Protocol: protocolFromURL(endpoint), Host: endpoint.Hostname(), Port: portFromURL(endpoint), UpstreamKind: proxyruntimev1.ProxyUpstreamKind_PROXY_UPSTREAM_KIND_SIMPLE_PROXY, RotationMode: proxyruntimev1.ProxyRotationMode_PROXY_ROTATION_MODE_NONE, Labels: map[string]string{"mode": "provider_control_plane"}}}}}}, nil
}

func (r *Runtime) parseStaticChain() ([]*url.URL, error) {
	nodes := make([]*url.URL, 0, len(r.cfg.StaticChain))
	for _, raw := range r.cfg.StaticChain {
		parsed, err := proxyurl.Parse(raw, "http")
		if err != nil {
			return nil, fmt.Errorf("parse static chain proxy: %w", err)
		}
		nodes = append(nodes, parsed)
	}
	return nodes, nil
}
func (r *Runtime) commonEgressService() *dataplane.LocalService {
	if r.cfg.CommonEgressAddr == "" {
		return nil
	}
	service := localServiceFromListener(config.EgressListener{ID: "common-egress", Addr: r.cfg.CommonEgressAddr, Protocol: r.cfg.LocalProtocol, Route: config.ListenerRouteDirect}, r.cfg.LocalProtocol)
	return &service
}
func (r *Runtime) defaultLocalService() dataplane.LocalService {
	return dataplane.LocalService{Name: "dynamic-egress", Addr: r.cfg.LocalAddr, Protocol: r.cfg.LocalProtocol, Username: r.cfg.LocalUsername, Password: r.cfg.LocalPassword}
}
func (r *Runtime) routePlaneListeners() []dataplane.LocalService {
	configs := r.baseListenerConfigs()
	services := make([]dataplane.LocalService, 0, len(configs))
	for _, listener := range configs {
		if listenerRoute(listener) == config.ListenerRouteUpstream {
			continue
		}
		services = append(services, localServiceFromListener(listener, r.cfg.LocalProtocol))
	}
	return services
}
func (r *Runtime) baseListenerConfigs() []config.EgressListener {
	if len(r.cfg.Listeners) > 0 {
		return append([]config.EgressListener(nil), r.cfg.Listeners...)
	}
	return r.defaultListenerConfigs()
}
func (r *Runtime) defaultListenerConfigs() []config.EgressListener {
	return []config.EgressListener{{ID: "dynamic-egress", Addr: r.cfg.LocalAddr, Protocol: r.cfg.LocalProtocol, Route: config.ListenerRouteProvider}}
}
func (r *Runtime) upstreamListenerConfigs() []config.EgressListener {
	configs := r.baseListenerConfigs()
	out := []config.EgressListener{}
	for _, listener := range configs {
		if listenerRoute(listener) == config.ListenerRouteUpstream {
			out = append(out, listener)
		}
	}
	return out
}
func (r *Runtime) protoListeners(leases []*proxyruntimev1.ProxyDynamicLease) []*proxyruntimev1.EgressListener {
	configs := r.baseListenerConfigs()
	out := make([]*proxyruntimev1.EgressListener, 0, len(configs)+len(leases))
	for _, listener := range configs {
		out = append(out, protoListener(listener, len(r.cfg.Listeners) == 0))
	}
	for _, lease := range leases {
		if lease.GetListener() != nil {
			out = append(out, lease.GetListener())
		}
	}
	return out
}
func protoListener(listener config.EgressListener, managed bool) *proxyruntimev1.EgressListener {
	route := listenerRoute(listener)
	kind := proxyruntimev1.EgressListenerKind_EGRESS_LISTENER_KIND_PROVIDER_ROUTE
	routeID := "default-data-plane"
	if route == config.ListenerRouteDirect {
		kind = proxyruntimev1.EgressListenerKind_EGRESS_LISTENER_KIND_DIRECT
		routeID = "direct"
	}
	labels := cloneLabels(listener.Labels)
	if route == config.ListenerRouteUpstream {
		labels["route"] = config.ListenerRouteUpstream
		routeID = listener.ID + "-chain"
	}
	if labels["mode"] == "dynamic_ip_session_lease" {
		kind = proxyruntimev1.EgressListenerKind_EGRESS_LISTENER_KIND_DYNAMIC_LEASE
		routeID = labels["chain_id"]
	}
	return &proxyruntimev1.EgressListener{ListenerId: listener.ID, Kind: kind, ListenAddr: listener.Addr, Protocol: protocolFromName(listenerProtocol(listener, "http")), RouteId: routeID, Managed: managed, Labels: labels}
}

func (r *Runtime) leaseListener(ctx context.Context, accountID string) (config.EgressListener, error) {
	leases, _ := r.leases.ListLeases(ctx, false)
	for _, lease := range leases {
		if lease.GetAccountId() == accountID && lease.GetListener() != nil {
			return listenerFromProto(lease.GetListener()), nil
		}
	}
	used := map[int]struct{}{}
	for _, lease := range leases {
		if _, portValue, err := net.SplitHostPort(lease.GetListener().GetListenAddr()); err == nil {
			var port int
			_, _ = fmt.Sscanf(portValue, "%d", &port)
			if port > 0 {
				used[port] = struct{}{}
			}
		}
	}
	span := r.cfg.SessionListener.PortEnd - r.cfg.SessionListener.PortStart + 1
	start := r.cfg.SessionListener.PortStart + int(hashModulo(accountID, uint32(span)))
	port := start
	for {
		if _, ok := used[port]; !ok && tcpPortAvailable(r.cfg.SessionListener.Host, port) {
			break
		}
		port++
		if port > r.cfg.SessionListener.PortEnd {
			port = r.cfg.SessionListener.PortStart
		}
		if port == start {
			return config.EgressListener{}, fmt.Errorf("no available proxy runtime session listener port in %d-%d", r.cfg.SessionListener.PortStart, r.cfg.SessionListener.PortEnd)
		}
	}
	id := "lease-" + shortHash(accountID)
	return config.EgressListener{ID: id, Addr: net.JoinHostPort(r.cfg.SessionListener.Host, fmt.Sprintf("%d", port)), Protocol: r.cfg.LocalProtocol, Route: config.ListenerRouteProvider, Labels: map[string]string{"mode": "dynamic_ip_session_lease", "account_id": accountID, "chain_id": leaseChainID(accountID)}}, nil
}

func tcpPortAvailable(host string, port int) bool {
	if port <= 0 {
		return false
	}
	addr := net.JoinHostPort(host, fmt.Sprintf("%d", port))
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return false
	}
	_ = listener.Close()
	return true
}
func localServiceFromListener(listener config.EgressListener, fallback string) dataplane.LocalService {
	return dataplane.LocalService{Name: listener.ID, Addr: listener.Addr, Protocol: listenerProtocol(listener, fallback), Username: listener.Username, Password: listener.Password, Route: listenerRoute(listener), Upstream: listener.Upstream}
}
func listenerFromProto(listener *proxyruntimev1.EgressListener) config.EgressListener {
	if listener == nil {
		return config.EgressListener{}
	}
	return config.EgressListener{ID: listener.GetListenerId(), Addr: listener.GetListenAddr(), Protocol: protocolName(listener.GetProtocol()), Route: config.ListenerRouteProvider, Labels: listener.GetLabels()}
}
func listenerProtocol(listener config.EgressListener, fallback string) string {
	if strings.TrimSpace(listener.Protocol) == "" {
		return fallback
	}
	return listener.Protocol
}
func listenerRoute(listener config.EgressListener) string {
	switch strings.TrimSpace(listener.Route) {
	case config.ListenerRouteDirect:
		return config.ListenerRouteDirect
	case config.ListenerRouteUpstream:
		return config.ListenerRouteUpstream
	default:
		return config.ListenerRouteProvider
	}
}
func leaseChainID(accountID string) string { return "lease-" + shortHash(accountID) + "-chain" }
func shortHash(value string) string {
	h := hashModulo(value, 0xffffffff)
	return fmt.Sprintf("%08x", h)
}
func hashModulo(value string, modulo uint32) uint32 {
	var h uint32 = 2166136261
	for _, ch := range []byte(value) {
		h ^= uint32(ch)
		h *= 16777619
	}
	if modulo > 0 {
		return h % modulo
	}
	return h
}
func leaseActive(lease *proxyruntimev1.ProxyDynamicLease, now time.Time) bool {
	if lease == nil || lease.GetStatus() != proxyruntimev1.ProxyDynamicLeaseStatus_PROXY_DYNAMIC_LEASE_STATUS_ACTIVE {
		return false
	}
	return lease.GetExpiresAt() == nil || now.Before(lease.GetExpiresAt().AsTime())
}
func routeRuntimeKind(name string) proxyruntimev1.ProxyRouteRuntimeKind {
	return proxyruntimev1.ProxyRouteRuntimeKind_PROXY_ROUTE_RUNTIME_KIND_GOST
}
func sourceRuntimeKind(name string) proxyruntimev1.ProxySourceRuntimeKind {
	if strings.EqualFold(name, "mihomo") {
		return proxyruntimev1.ProxySourceRuntimeKind_PROXY_SOURCE_RUNTIME_KIND_MIHOMO
	}
	return proxyruntimev1.ProxySourceRuntimeKind_PROXY_SOURCE_RUNTIME_KIND_NONE
}
func statusString(status dataplane.Status) string {
	if !status.Running {
		return firstNonEmpty(status.LastError, "stopped")
	}
	return "running"
}
func sourceStatusString(name string, status sourceplane.Status) string {
	if strings.EqualFold(name, "none") {
		return "disabled"
	}
	if !status.Running {
		return firstNonEmpty(status.LastError, "stopped")
	}
	return "running"
}

func (r *Runtime) sourcePlaneConfig() sourceplane.Config {
	return sourceplane.Config{
		Endpoint:            sourceplane.Endpoint{Addr: r.cfg.Mihomo.MixedAddr, Protocol: "socks5"},
		GroupStrategy:       r.cfg.Mihomo.GroupStrategy,
		HealthCheckURL:      r.cfg.Mihomo.HealthCheckURL,
		HealthCheckInterval: r.cfg.Mihomo.HealthCheckInterval,
		HealthCheckTimeout:  r.cfg.Mihomo.HealthCheckTimeout,
	}
}

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
	lineNode := lineNodeForPlan(lease.GetChainPlan(), r.sourceRuntimeNode(poolNodes))
	route := dataplane.SessionRoute{
		SessionID:   lease.GetSession().GetSessionId(),
		ChainID:     leaseChainID(lease.GetAccountId()),
		Listener:    localServiceFromListener(listenerFromProto(lease.GetListener()), r.cfg.LocalProtocol),
		StaticChain: routeStaticChain(staticChain, lineNode),
		Pool:        nodes,
	}
	return r.routePlane.UpsertSessionRoute(ctx, route)
}

func (r *Runtime) checkIPListener(listenerID string) (config.EgressListener, error) {
	configs := r.baseListenerConfigs()
	leases, _ := r.leases.ListLeases(context.Background(), false)
	for _, lease := range leases {
		if lease.GetListener() != nil {
			configs = append(configs, listenerFromProto(lease.GetListener()))
		}
	}
	if listenerID != "" {
		for _, listener := range configs {
			if listener.ID == listenerID {
				return listener, nil
			}
		}
		return config.EgressListener{}, fmt.Errorf("listener %q is not configured", listenerID)
	}
	for _, listener := range configs {
		if listenerRoute(listener) == config.ListenerRouteProvider {
			return listener, nil
		}
	}
	if len(configs) == 0 {
		return config.EgressListener{}, errors.New("no egress listener is configured")
	}
	return configs[0], nil
}
func (r *Runtime) localListenerEndpoint(listener config.EgressListener, advertisedHost string) (*proxyruntimev1.ProxyEndpoint, error) {
	hostPort, err := localListenHostPort(listener.Addr)
	if err != nil {
		return nil, err
	}
	host, portValue, err := net.SplitHostPort(hostPort)
	if err != nil {
		return nil, err
	}
	port, err := parsePort(portValue)
	if err != nil {
		return nil, err
	}
	if advertisedHost != "" && (host == "127.0.0.1" || host == "localhost" || host == "0.0.0.0" || host == "::1") {
		host = advertisedHost
	}
	return &proxyruntimev1.ProxyEndpoint{Id: listener.ID, Protocol: protocolFromName(listenerProtocol(listener, r.cfg.LocalProtocol)), Host: host, Port: port, Labels: cloneLabels(listener.Labels)}, nil
}

func (r *Runtime) sessionAdvertisedHost(req *http.Request, listener config.EgressListener) string {
	if host := strings.TrimSpace(r.cfg.SessionListener.AdvertisedHost); host != "" {
		return host
	}
	bindHost := listenerBindHost(listener.Addr)
	if bindHost != "" && !localOnlyHost(bindHost) {
		return bindHost
	}
	if unspecifiedBindHost(bindHost) {
		if host := firstLocalAdvertisedIP(); host != "" {
			return host
		}
	}
	return advertisedProxyHost(req)
}

func listenerBindHost(addr string) string {
	addr = strings.TrimSpace(addr)
	if strings.HasPrefix(addr, ":") {
		return ""
	}
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return ""
	}
	return strings.Trim(host, "[]")
}

func localOnlyHost(host string) bool {
	host = strings.Trim(strings.TrimSpace(strings.ToLower(host)), "[]")
	if host == "" || host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip == nil || ip.IsUnspecified() || ip.IsLoopback()
}

func unspecifiedBindHost(host string) bool {
	host = strings.Trim(strings.TrimSpace(strings.ToLower(host)), "[]")
	if host == "" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsUnspecified()
}

func firstLocalAdvertisedIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	ipv6 := ""
	for _, addr := range addrs {
		ip := interfaceIP(addr)
		if ip == nil || ip.IsLoopback() || ip.IsLinkLocalUnicast() || !ip.IsGlobalUnicast() {
			continue
		}
		if ipv4 := ip.To4(); ipv4 != nil {
			return ipv4.String()
		}
		if ipv6 == "" {
			ipv6 = ip.String()
		}
	}
	return ipv6
}

func interfaceIP(addr net.Addr) net.IP {
	switch value := addr.(type) {
	case *net.IPNet:
		return value.IP
	case *net.IPAddr:
		return value.IP
	default:
		return nil
	}
}

func (r *Runtime) reloadForwarders(_ context.Context) error {
	listeners := r.upstreamListenerConfigs()
	signature := upstreamForwarderSignature(listeners)
	r.forwardMu.Lock()
	if r.forwardSignature == signature {
		r.forwardMu.Unlock()
		return nil
	}
	r.forwardMu.Unlock()
	r.stopForwarders()
	if len(listeners) == 0 {
		r.forwardMu.Lock()
		r.forwardSignature = signature
		r.forwardMu.Unlock()
		return nil
	}
	forwardCtx, cancel := context.WithCancel(context.Background())
	started := make([]net.Listener, 0, len(listeners))
	for _, listener := range listeners {
		upstream, err := proxyurl.Parse(listener.Upstream, "socks5")
		if err != nil {
			closeListeners(started)
			cancel()
			return err
		}
		ln, err := net.Listen("tcp", listener.Addr)
		if err != nil {
			closeListeners(started)
			cancel()
			return err
		}
		started = append(started, ln)
		go r.serveForwarder(forwardCtx, listener.ID, ln, upstream.Host)
	}
	r.forwardMu.Lock()
	r.forwardCancel = cancel
	r.forwardListeners = started
	r.forwardSignature = signature
	r.forwardMu.Unlock()
	return nil
}
func (r *Runtime) stopForwarders() {
	r.forwardMu.Lock()
	cancel := r.forwardCancel
	listeners := r.forwardListeners
	r.forwardCancel = nil
	r.forwardListeners = nil
	r.forwardSignature = ""
	r.forwardMu.Unlock()
	if cancel != nil {
		cancel()
	}
	closeListeners(listeners)
}
func closeListeners(listeners []net.Listener) {
	for _, ln := range listeners {
		_ = ln.Close()
	}
}
func upstreamForwarderSignature(listeners []config.EgressListener) string {
	var b strings.Builder
	for _, listener := range listeners {
		b.WriteString(listener.ID)
		b.WriteByte('\x00')
		b.WriteString(listener.Addr)
		b.WriteByte('\x00')
		b.WriteString(listener.Upstream)
		b.WriteByte('\x00')
	}
	return b.String()
}
func (r *Runtime) serveForwarder(ctx context.Context, listenerID string, ln net.Listener, upstreamAddr string) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		go r.forwardConn(ctx, listenerID, conn, upstreamAddr)
	}
}
func (r *Runtime) forwardConn(ctx context.Context, _ string, client net.Conn, upstreamAddr string) {
	defer client.Close()
	upstream, err := (&net.Dialer{Timeout: 10 * time.Second}).DialContext(ctx, "tcp", upstreamAddr)
	if err != nil {
		return
	}
	defer upstream.Close()
	done := make(chan struct{}, 2)
	copyConn := func(dst net.Conn, src net.Conn) { _, _ = io.Copy(dst, src); _ = dst.Close(); done <- struct{}{} }
	go copyConn(upstream, client)
	go copyConn(client, upstream)
	select {
	case <-ctx.Done():
	case <-done:
	}
}

func (r *Runtime) readProto(w http.ResponseWriter, req *http.Request, message proto.Message) bool {
	if req.Body == nil {
		http.Error(w, "request body is required", http.StatusBadRequest)
		return false
	}
	body, err := readRequestBody(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return false
	}
	if len(strings.TrimSpace(string(body))) == 0 {
		return true
	}
	if err := protojsonx.Unmarshal(body, message); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return false
	}
	return true
}
func (r *Runtime) writeProto(w http.ResponseWriter, message proto.Message) {
	data, err := protojsonx.Marshal(message)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(data)
}
func readRequestBody(req *http.Request) ([]byte, error) {
	defer req.Body.Close()
	return httpx.ReadLimited(req.Body, 1<<20)
}
func methodNotAllowed(w http.ResponseWriter, allow string) {
	w.Header().Set("Allow", allow)
	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}
func advertisedProxyHost(req *http.Request) string {
	if req == nil {
		return ""
	}
	host := strings.TrimSpace(req.Header.Get("X-Forwarded-Host"))
	if host == "" {
		host = strings.TrimSpace(req.Host)
	}
	if parsed, _, err := net.SplitHostPort(host); err == nil {
		return parsed
	}
	return host
}
func protocolFromName(protocol string) proxyruntimev1.ProxyProtocol {
	if protocol == "socks5" {
		return proxyruntimev1.ProxyProtocol_PROXY_PROTOCOL_SOCKS5
	}
	return proxyruntimev1.ProxyProtocol_PROXY_PROTOCOL_HTTP
}
func protocolName(protocol proxyruntimev1.ProxyProtocol) string {
	if protocol == proxyruntimev1.ProxyProtocol_PROXY_PROTOCOL_SOCKS5 {
		return "socks5"
	}
	return "http"
}
func protocolFromURL(proxyURL *url.URL) proxyruntimev1.ProxyProtocol {
	if proxyURL != nil && proxyURL.Scheme == "socks5" {
		return proxyruntimev1.ProxyProtocol_PROXY_PROTOCOL_SOCKS5
	}
	return proxyruntimev1.ProxyProtocol_PROXY_PROTOCOL_HTTP
}
func portFromURL(proxyURL *url.URL) uint32 {
	if proxyURL == nil || proxyURL.Port() == "" {
		return 0
	}
	port, _ := parsePort(proxyURL.Port())
	return port
}
func parsePort(portValue string) (uint32, error) {
	var port uint32
	_, err := fmt.Sscanf(portValue, "%d", &port)
	return port, err
}
func providerHTTPProxyRef(raw string) string {
	if raw == "" {
		return ""
	}
	parsed, err := proxyurl.Parse(raw, "http")
	if err != nil {
		return "configured"
	}
	return parsed.Scheme + "://" + parsed.Host
}
func cloneLabels(labels map[string]string) map[string]string {
	cloned := map[string]string{}
	for k, v := range labels {
		cloned[k] = v
	}
	return cloned
}
func cloneNodes(in []provider.Node) []provider.Node {
	out := make([]provider.Node, 0, len(in))
	for _, node := range in {
		cloned := node
		if node.URL != nil {
			u := *node.URL
			cloned.URL = &u
		}
		cloned.Labels = cloneLabels(node.Labels)
		out = append(out, cloned)
	}
	return out
}
func buildRuntimeHTTPClient(cfg config.Config) *http.Client {
	return &http.Client{Timeout: cfg.RequestTimeout}
}
func noCacheFileServer(dir string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		path := filepath.Join(dir, filepath.Clean(req.URL.Path))
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			http.ServeFile(w, req, path)
			return
		}
		http.NotFound(w, req)
	})
}
