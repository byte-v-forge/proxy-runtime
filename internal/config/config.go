package config

import (
	"errors"
	"time"

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
	RuntimeAddr        string
	RouteRuntime       string
	SourceRuntime      string
	PostgresDSN        string
	RedisURL           string
	ApplyMigrations    bool
	EncryptionKey      string
	GostPath           string
	GostConfigDir      string
	GostAPIAddr        string
	GostMetricsAddr    string
	Mihomo             MihomoConfig
	CommonEgressAddr   string
	LocalAddr          string
	LocalProtocol      string
	LocalUsername      string
	LocalPassword      string
	SessionListener    SessionListenerConfig
	StaticChain        []string
	SimpleProxies      []string
	ProviderHTTPProxy  string
	Provider           string
	Listeners          []EgressListener
	RefreshInterval    time.Duration
	RequestTimeout     time.Duration
	ProxyExitGeoURLs   []string
	IPFraud            IPFraudConfig
	EdgeCanaryTimeout  time.Duration
	DashboardStaticDir string
	Ten24              ten24.Config
}
