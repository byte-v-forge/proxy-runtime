package accountproxy

import (
	"net/http"

	proxyruntimev1 "github.com/byte-v-forge/common-lib/gen/go/byte/v/forge/contracts/proxyruntime/v1"
	"github.com/byte-v-forge/proxy-runtime/internal/provider"
)

const (
	ProviderTen24    = "1024proxy"
	ProviderB2Proxy  = "b2proxy"
	ProviderCliproxy = "cliproxy"

	defaultStickyMinutes = 10
	minStickyMinutes     = 1
	maxStickyMinutes     = 120
)

type Config struct {
	ProviderID string
	Username   string
	Password   string
	Gateways   []Gateway
}

type Gateway struct {
	ID               string
	DisplayName      string
	Addr             string
	DefaultProtocol  string
	Protocols        []string
	PreferredRegions []string
}

type Plugin interface {
	ID() string
	DisplayName() string
	Descriptor(gateways []Gateway) *proxyruntimev1.ProxyProviderDescriptor
	DynamicSource(accountID string, displayName string, gateways []Gateway) *proxyruntimev1.ProxySourceDescriptor
	NewProvider(cfg Config, client *http.Client) (provider.Provider, error)
	Validate(cfg Config) error
}

type UsernameBuilder func(base string, policy *proxyruntimev1.ProxySessionPolicy, sessionID string) string

type Definition struct {
	ProviderID               string
	DisplayName              string
	DefaultProtocol          string
	Protocols                []string
	Gateways                 []Gateway
	UsernameParameterSession bool
	BuildUsername            UsernameBuilder
}
