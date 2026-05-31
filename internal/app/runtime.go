package app

import (
	"context"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/byte-v-forge/proxy-runtime/internal/config"
	"github.com/byte-v-forge/proxy-runtime/internal/dataplane"
	"github.com/byte-v-forge/proxy-runtime/internal/provider"
	"github.com/byte-v-forge/proxy-runtime/internal/sourceplane"
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
