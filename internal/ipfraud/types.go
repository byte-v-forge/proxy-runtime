package ipfraud

import (
	"context"
	"net/http"
	"time"

	proxyruntimev1 "github.com/byte-v-forge/common-lib/gen/go/byte/v/forge/contracts/proxyruntime/v1"
)

type ProviderConfig struct {
	ID     string
	Kind   proxyruntimev1.ProxyIPFraudProviderKind
	Weight int
	Auth   AuthConfig
}

type AnonymousAuthConfig struct{}

type APIKeyAuthConfig struct {
	Keys      []string
	Placement string
	Name      string
}

type AuthConfig struct {
	Anonymous *AnonymousAuthConfig
	APIKey    *APIKeyAuthConfig
}

type Config struct {
	Providers   []ProviderConfig
	Timeout     time.Duration
	CacheTTL    time.Duration
	KeyCooldown time.Duration
}

type Plugin interface {
	Kind() proxyruntimev1.ProxyIPFraudProviderKind
	ProviderID() string
	DisplayName() string
	DefaultWeight() uint32
	SupportsAnonymous() bool
	SupportsAPIKey() bool
	Auth(apiKeys []string, anonymous bool) AuthConfig
	New(client *http.Client, cfg ProviderConfig, cooldown time.Duration) provider
}

type report struct {
	providerID     string
	providerName   string
	networkKind    proxyruntimev1.ProxyIPNetworkKind
	anonymizerKind proxyruntimev1.ProxyIPAnonymizerKind
	riskLevel      proxyruntimev1.ProxyIPFraudRiskLevel
	riskScore      float64
	signals        []proxyruntimev1.ProxyIPFraudSignal
	countryCode    string
	region         string
	city           string
	asn            string
	organization   string
	isp            string
}

type provider interface {
	lookup(ctx context.Context, ip string) (report, error)
}
