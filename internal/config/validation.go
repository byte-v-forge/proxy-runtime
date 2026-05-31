package config

import (
	"errors"
	"fmt"
	"strings"
)

func (c Config) validate() error {
	if c.RuntimeAddr == "" {
		return errors.New("PROXY_RUNTIME_ADDR is required")
	}
	if strings.TrimSpace(c.PostgresDSN) == "" {
		return errors.New("PROXY_RUNTIME_POSTGRES_DSN or PG_DSN is required")
	}
	if strings.TrimSpace(c.EncryptionKey) == "" {
		return errors.New("PROXY_RUNTIME_ENCRYPTION_KEY is required")
	}
	if strings.TrimSpace(c.RedisURL) == "" {
		return errors.New("PLATFORM_REDIS_URL is required")
	}
	switch c.RouteRuntime {
	case RouteRuntimeGOST:
	default:
		return fmt.Errorf("unsupported route runtime %q", c.RouteRuntime)
	}
	if c.RouteRuntime == RouteRuntimeGOST && c.GostPath == "" {
		return errors.New("PROXY_RUNTIME_GOST_PATH is required")
	}
	switch c.SourceRuntime {
	case SourceRuntimeNone:
	case SourceRuntimeMihomo:
		if err := c.Mihomo.validate(); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported source runtime %q", c.SourceRuntime)
	}
	if !isLocalProtocol(c.LocalProtocol) {
		return fmt.Errorf("unsupported local protocol %q", c.LocalProtocol)
	}
	if err := c.SessionListener.validate(); err != nil {
		return err
	}
	if err := validateListeners(c.Listeners); err != nil {
		return err
	}
	if c.Provider != ProviderTen24 && c.Provider != ProviderNone && c.Provider != ProviderStatic {
		return ErrUnsupportedProvider
	}
	if c.Provider == ProviderStatic && len(c.SimpleProxies) == 0 {
		return errors.New("PROXY_RUNTIME_SIMPLE_PROXIES is required for static provider")
	}
	if c.Provider == ProviderTen24 && c.Ten24.HasRuntimeConfig() {
		if err := c.Ten24.Validate(); err != nil {
			return err
		}
	}
	if c.RefreshInterval < 0 {
		return errors.New("PROXY_RUNTIME_REFRESH_SECONDS must be >= 0")
	}
	if c.RequestTimeout <= 0 {
		return errors.New("PROXY_RUNTIME_REQUEST_TIMEOUT_SECONDS must be > 0")
	}
	if len(c.ProxyExitGeoURLs) == 0 {
		return errors.New("PROXY_RUNTIME_PROXY_EXIT_GEO_URLS must not be empty")
	}
	if c.EdgeCanaryTimeout <= 0 {
		return errors.New("PROXY_RUNTIME_EDGE_CANARY_TIMEOUT_SECONDS must be > 0")
	}
	if err := c.IPFraud.validate(); err != nil {
		return err
	}
	return nil
}
