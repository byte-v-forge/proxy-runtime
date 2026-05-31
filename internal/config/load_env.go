package config

import (
	"os"
	"strings"
	"time"

	"github.com/byte-v-forge/common-lib/envx"
	"github.com/byte-v-forge/proxy-runtime/internal/provider/ten24"
)

func LoadFromEnv() (Config, error) {
	cfg := Config{
		RuntimeAddr:     envx.StringDefault("PROXY_RUNTIME_ADDR", ":8080"),
		RouteRuntime:    normalizeConfigToken(envx.StringDefault("PROXY_RUNTIME_ROUTE_RUNTIME", RouteRuntimeGOST)),
		SourceRuntime:   normalizeConfigToken(envx.StringDefault("PROXY_RUNTIME_SOURCE_RUNTIME", SourceRuntimeMihomo)),
		PostgresDSN:     firstNonEmpty(strings.TrimSpace(os.Getenv("PROXY_RUNTIME_POSTGRES_DSN")), strings.TrimSpace(os.Getenv("PG_DSN"))),
		RedisURL:        strings.TrimSpace(os.Getenv("PLATFORM_REDIS_URL")),
		ApplyMigrations: envx.Bool("PROXY_RUNTIME_APPLY_MIGRATIONS", true),
		EncryptionKey:   strings.TrimSpace(os.Getenv("PROXY_RUNTIME_ENCRYPTION_KEY")),
		GostPath:        envx.StringDefault("PROXY_RUNTIME_GOST_PATH", "gost"),
		GostConfigDir:   strings.TrimSpace(os.Getenv("PROXY_RUNTIME_GOST_CONFIG_DIR")),
		GostAPIAddr:     envx.StringDefault("PROXY_RUNTIME_GOST_API_ADDR", "127.0.0.1:18080"),
		GostMetricsAddr: strings.TrimSpace(os.Getenv("PROXY_RUNTIME_GOST_METRICS_ADDR")),
		Mihomo: MihomoConfig{
			Path:                envx.StringDefault("PROXY_RUNTIME_MIHOMO_PATH", "mihomo"),
			ConfigDir:           envx.StringDefault("PROXY_RUNTIME_MIHOMO_CONFIG_DIR", "/var/lib/byte-v-forge/proxy-runtime/mihomo"),
			MixedAddr:           envx.StringDefault("PROXY_RUNTIME_MIHOMO_MIXED_ADDR", "127.0.0.1:18900"),
			APIAddr:             envx.StringDefault("PROXY_RUNTIME_MIHOMO_API_ADDR", "127.0.0.1:18901"),
			GroupStrategy:       normalizeConfigToken(envx.StringDefault("PROXY_RUNTIME_MIHOMO_GROUP_STRATEGY", "fallback")),
			HealthCheckURL:      envx.StringDefault("PROXY_RUNTIME_MIHOMO_HEALTH_CHECK_URL", "https://www.gstatic.com/generate_204"),
			HealthCheckInterval: envx.DurationSeconds("PROXY_RUNTIME_MIHOMO_HEALTH_CHECK_INTERVAL_SECONDS", 300*time.Second),
			HealthCheckTimeout:  envx.DurationSeconds("PROXY_RUNTIME_MIHOMO_HEALTH_CHECK_TIMEOUT_SECONDS", 5*time.Second),
		},
		CommonEgressAddr: strings.TrimSpace(os.Getenv("PROXY_RUNTIME_COMMON_EGRESS_ADDR")),
		LocalAddr:        envx.StringDefault("PROXY_RUNTIME_DYNAMIC_EGRESS_ADDR", envx.StringDefault("PROXY_RUNTIME_LOCAL_ADDR", ":1080")),
		LocalProtocol:    normalizeConfigToken(envx.StringDefault("PROXY_RUNTIME_LOCAL_PROTOCOL", "http")),
		LocalUsername:    strings.TrimSpace(os.Getenv("PROXY_RUNTIME_LOCAL_USERNAME")),
		LocalPassword:    strings.TrimSpace(os.Getenv("PROXY_RUNTIME_LOCAL_PASSWORD")),
		SessionListener: SessionListenerConfig{
			Host:           envx.StringDefault("PROXY_RUNTIME_SESSION_LISTEN_HOST", "127.0.0.1"),
			AdvertisedHost: strings.TrimSpace(os.Getenv("PROXY_RUNTIME_SESSION_ADVERTISED_HOST")),
			PortStart:      envx.Int("PROXY_RUNTIME_SESSION_PORT_START", 19080),
			PortEnd:        envx.Int("PROXY_RUNTIME_SESSION_PORT_END", 19179),
		},
		StaticChain:       envx.List("PROXY_RUNTIME_STATIC_CHAIN"),
		SimpleProxies:     envx.List("PROXY_RUNTIME_SIMPLE_PROXIES"),
		ProviderHTTPProxy: strings.TrimSpace(os.Getenv("PROXY_RUNTIME_PROVIDER_HTTP_PROXY")),
		Provider:          normalizeConfigToken(envx.StringDefault("PROXY_RUNTIME_PROVIDER", ProviderTen24)),
		Listeners:         envListeners("PROXY_RUNTIME_LISTENERS_JSON"),
		RefreshInterval:   envx.DurationSeconds("PROXY_RUNTIME_REFRESH_SECONDS", 300*time.Second),
		RequestTimeout:    envx.DurationSeconds("PROXY_RUNTIME_REQUEST_TIMEOUT_SECONDS", 10*time.Second),
		ProxyExitGeoURLs:  proxyExitGeoURLs("PROXY_RUNTIME_PROXY_EXIT_GEO_URLS"),
		EdgeCanaryTimeout: envx.DurationSeconds("PROXY_RUNTIME_EDGE_CANARY_TIMEOUT_SECONDS", 10*time.Second),
		IPFraud: IPFraudConfig{
			Timeout:     envx.DurationSeconds("PROXY_RUNTIME_IP_FRAUD_TIMEOUT_SECONDS", 10*time.Second),
			CacheTTL:    envx.DurationSeconds("PROXY_RUNTIME_IP_FRAUD_CACHE_TTL_SECONDS", 10*time.Minute),
			KeyCooldown: envx.DurationSeconds("PROXY_RUNTIME_IP_FRAUD_KEY_COOLDOWN_SECONDS", 24*time.Hour),
		},
		DashboardStaticDir: envx.StringDefault("PROXY_RUNTIME_DASHBOARD_STATIC_DIR", "/app/dashboard/proxy-runtime"),
		Ten24: ten24.Config{
			APIURL:        strings.TrimSpace(os.Getenv("PROXY_RUNTIME_1024_API_URL")),
			APIRegion:     strings.TrimSpace(os.Getenv("PROXY_RUNTIME_1024_API_REGION")),
			APIFormat:     strings.TrimSpace(os.Getenv("PROXY_RUNTIME_1024_API_FORMAT")),
			APITime:       strings.TrimSpace(os.Getenv("PROXY_RUNTIME_1024_API_TIME")),
			APINum:        strings.TrimSpace(os.Getenv("PROXY_RUNTIME_1024_API_NUM")),
			APIType:       strings.TrimSpace(os.Getenv("PROXY_RUNTIME_1024_API_TYPE")),
			ProxyAddr:     strings.TrimSpace(os.Getenv("PROXY_RUNTIME_1024_PROXY_ADDR")),
			Username:      strings.TrimSpace(os.Getenv("PROXY_RUNTIME_1024_USERNAME")),
			Password:      strings.TrimSpace(os.Getenv("PROXY_RUNTIME_1024_PASSWORD")),
			Protocol:      normalizeConfigToken(envx.StringDefault("PROXY_RUNTIME_1024_PROTOCOL", "http")),
			SessionID:     strings.TrimSpace(os.Getenv("PROXY_RUNTIME_1024_SESSION_ID")),
			StickyMinutes: envx.Int("PROXY_RUNTIME_1024_STICKY_MINUTES", 0),
		},
	}
	return cfg, cfg.validate()
}
