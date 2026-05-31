package mihomo

import (
	"time"

	"github.com/byte-v-forge/proxy-runtime/internal/sourceplane"
)

type renderOptions struct {
	Providers           []sourceplane.SubscriptionProvider
	FixedProxies        []sourceplane.FixedProxy
	Endpoint            sourceplane.Endpoint
	ConfigDir           string
	APIAddr             string
	GroupStrategy       string
	HealthCheckURL      string
	HealthCheckInterval time.Duration
	HealthCheckTimeout  time.Duration
	NodeListeners       []nodeListener
}

type mihomoConfig struct {
	MixedPort          int                       `json:"mixed-port"`
	BindAddress        string                    `json:"bind-address,omitempty"`
	AllowLAN           bool                      `json:"allow-lan"`
	Mode               string                    `json:"mode"`
	LogLevel           string                    `json:"log-level"`
	ExternalController string                    `json:"external-controller,omitempty"`
	Proxies            []map[string]any          `json:"proxies,omitempty"`
	ProxyProviders     map[string]mihomoProvider `json:"proxy-providers,omitempty"`
	ProxyGroups        []mihomoGroup             `json:"proxy-groups,omitempty"`
	Listeners          []mihomoListener          `json:"listeners,omitempty"`
	Rules              []string                  `json:"rules"`
}

type mihomoProvider struct {
	Type        string              `json:"type"`
	URL         string              `json:"url"`
	Path        string              `json:"path,omitempty"`
	Interval    int                 `json:"interval,omitempty"`
	Filter      string              `json:"filter,omitempty"`
	Exclude     string              `json:"exclude-filter,omitempty"`
	HealthCheck *mihomoHealthCheck  `json:"health-check,omitempty"`
	Header      map[string][]string `json:"header,omitempty"`
}

type mihomoHealthCheck struct {
	Enable         bool   `json:"enable"`
	URL            string `json:"url,omitempty"`
	Interval       int    `json:"interval,omitempty"`
	Timeout        int    `json:"timeout,omitempty"`
	Lazy           bool   `json:"lazy"`
	ExpectedStatus uint32 `json:"expected-status,omitempty"`
}

type mihomoGroup struct {
	Name           string   `json:"name"`
	Type           string   `json:"type"`
	Proxies        []string `json:"proxies,omitempty"`
	Use            []string `json:"use,omitempty"`
	Filter         string   `json:"filter,omitempty"`
	URL            string   `json:"url,omitempty"`
	Interval       int      `json:"interval,omitempty"`
	Timeout        int      `json:"timeout,omitempty"`
	Lazy           bool     `json:"lazy"`
	ExpectedStatus uint32   `json:"expected-status,omitempty"`
}

type mihomoListener struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	Listen string `json:"listen,omitempty"`
	Port   int    `json:"port"`
	Proxy  string `json:"proxy"`
	UDP    bool   `json:"udp"`
}

type nodeListener struct {
	SourceID       string
	NodeID         string
	DisplayName    string
	ProxyName      string
	ProviderBacked bool
	Endpoint       sourceplane.Endpoint
}
