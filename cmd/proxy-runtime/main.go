package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/byte-v-forge/common-lib/httpclient"
	"github.com/byte-v-forge/common-lib/proxyurl"
	"github.com/byte-v-forge/proxy-runtime/internal/app"
	"github.com/byte-v-forge/proxy-runtime/internal/config"
	"github.com/byte-v-forge/proxy-runtime/internal/dataplane"
	"github.com/byte-v-forge/proxy-runtime/internal/dataplane/gostplane"
	"github.com/byte-v-forge/proxy-runtime/internal/gost"
	"github.com/byte-v-forge/proxy-runtime/internal/provider"
	"github.com/byte-v-forge/proxy-runtime/internal/provider/ten24"
	"github.com/byte-v-forge/proxy-runtime/internal/sourceplane"
	mihomosource "github.com/byte-v-forge/proxy-runtime/internal/sourceplane/mihomo"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{}))

	cfg, err := config.LoadFromEnv()
	if err != nil {
		logger.Error("load config failed", "error", err)
		os.Exit(1)
	}

	proxyProvider, err := buildProvider(cfg)
	if err != nil {
		logger.Error("create provider failed", "error", err)
		os.Exit(1)
	}

	routePlane, err := buildRoutePlane(cfg, logger)
	if err != nil {
		logger.Error("create route runtime failed", "error", err)
		os.Exit(1)
	}
	sourcePlane, err := buildSourcePlane(cfg, logger)
	if err != nil {
		logger.Error("create source runtime failed", "error", err)
		os.Exit(1)
	}
	store, err := app.NewPostgresStore(context.Background(), cfg, logger)
	if err != nil {
		logger.Error("create store failed", "error", err)
		os.Exit(1)
	}
	defer store.Close()
	leaseStore, err := app.NewLeaseStore(context.Background(), cfg)
	if err != nil {
		logger.Error("create lease cache failed", "error", err)
		os.Exit(1)
	}
	defer leaseStore.Close()
	runtime := app.NewRuntime(cfg, proxyProvider, routePlane, sourcePlane, store, leaseStore, logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := runtime.Run(ctx); err != nil {
		logger.Error("proxy runtime stopped", "error", err)
		os.Exit(1)
	}
}

func buildRoutePlane(cfg config.Config, logger *slog.Logger) (dataplane.Driver, error) {
	if cfg.RouteRuntime != config.RouteRuntimeGOST {
		return nil, fmt.Errorf("unsupported route runtime %q", cfg.RouteRuntime)
	}
	return gostplane.New(gost.ManagerConfig{
		GostPath:    cfg.GostPath,
		ConfigDir:   cfg.GostConfigDir,
		APIAddr:     cfg.GostAPIAddr,
		MetricsAddr: cfg.GostMetricsAddr,
	}, logger), nil
}

func buildSourcePlane(cfg config.Config, logger *slog.Logger) (sourceplane.Driver, error) {
	switch cfg.SourceRuntime {
	case config.SourceRuntimeNone:
		return sourceplane.Empty{}, nil
	case config.SourceRuntimeMihomo:
		return mihomosource.New(mihomosource.Config{
			Path:      cfg.Mihomo.Path,
			ConfigDir: cfg.Mihomo.ConfigDir,
			APIAddr:   cfg.Mihomo.APIAddr,
		}, logger), nil
	default:
		return nil, fmt.Errorf("unsupported source runtime %q", cfg.SourceRuntime)
	}
}

func buildProvider(cfg config.Config) (provider.Provider, error) {
	for _, plugin := range providerPlugins() {
		if plugin.key == cfg.Provider {
			return plugin.build(cfg)
		}
	}
	return nil, config.ErrUnsupportedProvider
}

type providerPlugin struct {
	key   string
	build func(config.Config) (provider.Provider, error)
}

func providerPlugins() []providerPlugin {
	return []providerPlugin{
		{key: config.ProviderNone, build: func(config.Config) (provider.Provider, error) { return provider.Empty{}, nil }},
		{key: config.ProviderStatic, build: func(cfg config.Config) (provider.Provider, error) { return provider.NewStatic(cfg.SimpleProxies) }},
		{key: config.ProviderTen24, build: func(cfg config.Config) (provider.Provider, error) {
			return ten24.New(cfg.Ten24, buildHTTPClient(cfg)), nil
		}},
	}
}

func buildHTTPClient(cfg config.Config) *http.Client {
	proxyURL := ""
	if cfg.ProviderHTTPProxy != "" {
		parsed, err := proxyurl.Parse(cfg.ProviderHTTPProxy, "http")
		if err == nil {
			proxyURL = parsed.String()
		}
	}
	client, err := httpclient.NewWithSchemes(cfg.RequestTimeout, proxyURL, httpclient.HTTPProxySchemes...)
	if err != nil {
		return &http.Client{Timeout: cfg.RequestTimeout}
	}
	return client
}
