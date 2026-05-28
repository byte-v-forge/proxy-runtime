package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/byte-v-forge/common-lib/envx"
	"github.com/byte-v-forge/proxy-runtime/internal/provider/ten24"
)

const (
	ProviderTen24  = "1024proxy"
	ProviderB2     = "b2proxy"
	ProviderCli    = "cliproxy"
	ProviderNone   = "none"
	ProviderStatic = "static"
)

const (
	RouteRuntimeGOST = "gost"
)

const (
	SourceRuntimeNone   = "none"
	SourceRuntimeMihomo = "mihomo"
)

var ErrUnsupportedProvider = errors.New("unsupported proxy provider")

const (
	ListenerRouteDirect   = "direct"
	ListenerRouteProvider = "provider"
	ListenerRouteUpstream = "upstream"
)

type EgressListener struct {
	ID       string            `json:"id"`
	Addr     string            `json:"addr"`
	Protocol string            `json:"protocol"`
	Route    string            `json:"route"`
	Upstream string            `json:"upstream"`
	Username string            `json:"username"`
	Password string            `json:"password"`
	Labels   map[string]string `json:"labels"`
}

type SessionListenerConfig struct {
	Host           string
	AdvertisedHost string
	PortStart      int
	PortEnd        int
}

type MihomoConfig struct {
	Path                string
	ConfigDir           string
	MixedAddr           string
	APIAddr             string
	GroupStrategy       string
	HealthCheckURL      string
	HealthCheckInterval time.Duration
	HealthCheckTimeout  time.Duration
}

type IPFraudConfig struct {
	Timeout     time.Duration
	CacheTTL    time.Duration
	KeyCooldown time.Duration
}

type Config struct {
	RuntimeAddr         string
	RouteRuntime        string
	SourceRuntime       string
	PostgresDSN         string
	RedisURL            string
	ApplyMigrations     bool
	EncryptionKey       string
	GostPath            string
	GostConfigDir       string
	GostAPIAddr         string
	GostMetricsAddr     string
	Mihomo              MihomoConfig
	CommonEgressAddr    string
	LocalAddr           string
	LocalProtocol       string
	LocalUsername       string
	LocalPassword       string
	SessionListener     SessionListenerConfig
	StaticChain         []string
	SimpleProxies       []string
	ProviderHTTPProxy   string
	Provider            string
	Listeners           []EgressListener
	RefreshInterval     time.Duration
	RequestTimeout      time.Duration
	ProxyExitGeoTimeout time.Duration
	ProxyExitGeoURLs    []string
	IPFraud             IPFraudConfig
	EdgeCanaryTimeout   time.Duration
	DashboardStaticDir  string
	Ten24               ten24.Config
}

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
		StaticChain:         envx.List("PROXY_RUNTIME_STATIC_CHAIN"),
		SimpleProxies:       envx.List("PROXY_RUNTIME_SIMPLE_PROXIES"),
		ProviderHTTPProxy:   strings.TrimSpace(os.Getenv("PROXY_RUNTIME_PROVIDER_HTTP_PROXY")),
		Provider:            normalizeConfigToken(envx.StringDefault("PROXY_RUNTIME_PROVIDER", ProviderTen24)),
		Listeners:           envListeners("PROXY_RUNTIME_LISTENERS_JSON"),
		RefreshInterval:     envx.DurationSeconds("PROXY_RUNTIME_REFRESH_SECONDS", 300*time.Second),
		RequestTimeout:      envx.DurationSeconds("PROXY_RUNTIME_REQUEST_TIMEOUT_SECONDS", 10*time.Second),
		ProxyExitGeoTimeout: envx.DurationSeconds("PROXY_RUNTIME_PROXY_EXIT_GEO_TIMEOUT_SECONDS", 10*time.Second),
		ProxyExitGeoURLs:    proxyExitGeoURLs("PROXY_RUNTIME_PROXY_EXIT_GEO_URLS"),
		EdgeCanaryTimeout:   envx.DurationSeconds("PROXY_RUNTIME_EDGE_CANARY_TIMEOUT_SECONDS", 10*time.Second),
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
	if c.ProxyExitGeoTimeout <= 0 {
		return errors.New("PROXY_RUNTIME_PROXY_EXIT_GEO_TIMEOUT_SECONDS must be > 0")
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

func validateListeners(listeners []EgressListener) error {
	seen := map[string]struct{}{}
	for index, listener := range listeners {
		id := strings.TrimSpace(listener.ID)
		if id == "" {
			return fmt.Errorf("PROXY_RUNTIME_LISTENERS_JSON[%d].id is required", index)
		}
		if _, ok := seen[id]; ok {
			return fmt.Errorf("duplicate proxy runtime listener id %q", id)
		}
		seen[id] = struct{}{}
		if strings.TrimSpace(listener.Addr) == "" {
			return fmt.Errorf("PROXY_RUNTIME_LISTENERS_JSON[%d].addr is required", index)
		}
		protocol := normalizeConfigToken(listener.Protocol)
		if protocol == "" {
			protocol = "http"
		}
		if !isLocalProtocol(protocol) {
			return fmt.Errorf("unsupported listener protocol %q", listener.Protocol)
		}
		route := normalizeConfigToken(listener.Route)
		switch route {
		case "", ListenerRouteProvider, ListenerRouteDirect, ListenerRouteUpstream:
		default:
			return fmt.Errorf("unsupported listener route %q", listener.Route)
		}
		if route == ListenerRouteUpstream && strings.TrimSpace(listener.Upstream) == "" {
			return fmt.Errorf("PROXY_RUNTIME_LISTENERS_JSON[%d].upstream is required for upstream route", index)
		}
	}
	return nil
}

func isLocalProtocol(protocol string) bool {
	switch protocol {
	case "http", "socks5":
		return true
	default:
		return false
	}
}

func envListeners(name string) []EgressListener {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return nil
	}
	var listeners []EgressListener
	if err := json.Unmarshal([]byte(raw), &listeners); err != nil {
		return []EgressListener{{
			ID:    "__invalid__",
			Addr:  "invalid",
			Route: fmt.Sprintf("invalid JSON: %v", err),
		}}
	}
	for index := range listeners {
		listeners[index].ID = strings.TrimSpace(listeners[index].ID)
		listeners[index].Addr = strings.TrimSpace(listeners[index].Addr)
		listeners[index].Protocol = normalizeConfigToken(listeners[index].Protocol)
		listeners[index].Route = normalizeConfigToken(listeners[index].Route)
		listeners[index].Upstream = strings.TrimSpace(listeners[index].Upstream)
		listeners[index].Username = strings.TrimSpace(listeners[index].Username)
		listeners[index].Password = strings.TrimSpace(listeners[index].Password)
	}
	return listeners
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func normalizeConfigToken(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func proxyExitGeoURLs(name string) []string {
	values := envx.List(name)
	if len(values) == 0 {
		values = []string{"https://cloudflare.com/cdn-cgi/trace", "https://api.ipify.org?format=json"}
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			out = append(out, value)
		}
	}
	return out
}

func (c IPFraudConfig) validate() error {
	if c.Timeout <= 0 {
		return errors.New("PROXY_RUNTIME_IP_FRAUD_TIMEOUT_SECONDS must be > 0")
	}
	if c.CacheTTL <= 0 {
		return errors.New("PROXY_RUNTIME_IP_FRAUD_CACHE_TTL_SECONDS must be > 0")
	}
	if c.KeyCooldown <= 0 {
		return errors.New("PROXY_RUNTIME_IP_FRAUD_KEY_COOLDOWN_SECONDS must be > 0")
	}
	return nil
}

func (c SessionListenerConfig) validate() error {
	if strings.TrimSpace(c.Host) == "" {
		return errors.New("PROXY_RUNTIME_SESSION_LISTEN_HOST is required")
	}
	if c.PortStart <= 0 || c.PortStart > 65535 {
		return errors.New("PROXY_RUNTIME_SESSION_PORT_START must be between 1 and 65535")
	}
	if c.PortEnd <= 0 || c.PortEnd > 65535 {
		return errors.New("PROXY_RUNTIME_SESSION_PORT_END must be between 1 and 65535")
	}
	if c.PortEnd < c.PortStart {
		return errors.New("PROXY_RUNTIME_SESSION_PORT_END must be >= PROXY_RUNTIME_SESSION_PORT_START")
	}
	return nil
}

func (c MihomoConfig) validate() error {
	if strings.TrimSpace(c.Path) == "" {
		return errors.New("PROXY_RUNTIME_MIHOMO_PATH is required")
	}
	if strings.TrimSpace(c.MixedAddr) == "" {
		return errors.New("PROXY_RUNTIME_MIHOMO_MIXED_ADDR is required")
	}
	if strings.TrimSpace(c.APIAddr) == "" {
		return errors.New("PROXY_RUNTIME_MIHOMO_API_ADDR is required")
	}
	switch strings.TrimSpace(c.GroupStrategy) {
	case "", "select", "url-test", "fallback", "load-balance":
	default:
		return fmt.Errorf("unsupported mihomo group strategy %q", c.GroupStrategy)
	}
	if c.HealthCheckInterval < 0 {
		return errors.New("PROXY_RUNTIME_MIHOMO_HEALTH_CHECK_INTERVAL_SECONDS must be >= 0")
	}
	if c.HealthCheckTimeout < 0 {
		return errors.New("PROXY_RUNTIME_MIHOMO_HEALTH_CHECK_TIMEOUT_SECONDS must be >= 0")
	}
	return nil
}
