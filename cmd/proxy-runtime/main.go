package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/byte-v-forge/proxy-runtime/internal/app"
	"github.com/byte-v-forge/proxy-runtime/internal/config"
	"github.com/byte-v-forge/proxy-runtime/internal/gost"
	"github.com/byte-v-forge/proxy-runtime/internal/provider"
	"github.com/byte-v-forge/proxy-runtime/internal/provider/ten24"
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

	manager := gost.NewManager(gost.ManagerConfig{
		GostPath:    cfg.GostPath,
		ConfigDir:   cfg.GostConfigDir,
		APIAddr:     cfg.GostAPIAddr,
		MetricsAddr: cfg.GostMetricsAddr,
	}, logger)
	runtime := app.NewRuntime(cfg, proxyProvider, manager, logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := runtime.Run(ctx); err != nil {
		logger.Error("proxy runtime stopped", "error", err)
		os.Exit(1)
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
	client := &http.Client{Timeout: cfg.RequestTimeout}
	if cfg.ProviderHTTPProxy == "" {
		return client
	}
	proxyURL, err := provider.ParseProxy(cfg.ProviderHTTPProxy, "http")
	if err != nil {
		return client
	}
	client.Transport = &http.Transport{Proxy: http.ProxyURL(proxyURL)}
	return client
}
