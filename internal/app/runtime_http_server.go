package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

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
