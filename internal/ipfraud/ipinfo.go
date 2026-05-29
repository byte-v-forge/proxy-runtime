package ipfraud

import (
	"context"
	"net/http"
	"time"

	proxyruntimev1 "github.com/byte-v-forge/common-lib/gen/go/byte/v/forge/contracts/proxyruntime/v1"
)

type ipinfoProvider struct {
	details httpProvider
	privacy httpProvider
}

const (
	ipinfoDetailsEndpoint = "https://ipinfo.io/{ip}/json"
	ipinfoPrivacyEndpoint = "https://ipinfo.io/{ip}/privacy"
)

type ipinfoPlugin struct{}

func init() { Register(ipinfoPlugin{}) }

func (ipinfoPlugin) Kind() proxyruntimev1.ProxyIPFraudProviderKind {
	return proxyruntimev1.ProxyIPFraudProviderKind_PROXY_IP_FRAUD_PROVIDER_KIND_IPINFO
}
func (ipinfoPlugin) ProviderID() string      { return "ipinfo" }
func (ipinfoPlugin) DisplayName() string     { return "IPinfo" }
func (ipinfoPlugin) DefaultWeight() uint32   { return 90 }
func (ipinfoPlugin) SupportsAnonymous() bool { return false }
func (ipinfoPlugin) SupportsAPIKey() bool    { return true }
func (ipinfoPlugin) Auth(keys []string, _ bool) AuthConfig {
	return AuthConfig{APIKey: &APIKeyAuthConfig{Keys: append([]string(nil), keys...), Placement: "query", Name: "token"}}
}
func (ipinfoPlugin) New(client *http.Client, cfg ProviderConfig, cooldown time.Duration) provider {
	return &ipinfoProvider{
		details: newHTTPProvider(client, ipinfoDetailsEndpoint, cfg.Auth, cooldown),
		privacy: newHTTPProvider(client, ipinfoPrivacyEndpoint, cfg.Auth, cooldown),
	}
}

func (p *ipinfoProvider) lookup(ctx context.Context, ip string) (report, error) {
	payload, detailsErr := p.details.lookupJSON(ctx, ip)
	privacy, privacyErr := p.privacy.lookupJSON(ctx, ip)
	if detailsErr != nil && privacyErr != nil {
		return report{}, detailsErr
	}
	if payload == nil {
		payload = map[string]any{}
	}
	if privacy != nil {
		payload["privacy"] = privacy
	}
	if stringValue(payload, "error.message", "error.title", "privacy.error.message", "privacy.error.title") != "" {
		return report{}, errProviderUnavailable
	}
	return parseIPinfo(payload), nil
}

func parseIPinfo(payload map[string]any) report {
	flags := riskFlags{
		hosting:   boolValue(payload, "privacy.hosting", "anonymous.is_hosting", "is_hosting"),
		proxy:     boolValue(payload, "privacy.proxy", "privacy.relay", "anonymous.is_proxy", "anonymous.is_relay", "is_proxy", "is_relay"),
		vpn:       boolValue(payload, "privacy.vpn", "anonymous.is_vpn", "is_vpn"),
		tor:       boolValue(payload, "privacy.tor", "anonymous.is_tor", "is_tor"),
		mobile:    boolValue(payload, "is_mobile", "mobile"),
		satellite: boolValue(payload, "is_satellite", "satellite"),
		anycast:   boolValue(payload, "is_anycast", "anycast"),
	}
	networkKind := networkKindWithFlags(classifyNetworkKind(
		stringValue(payload, "as.type", "asn.type", "company.type"),
	), flags)
	return report{
		networkKind:    networkKind,
		anonymizerKind: classifyAnonymizerKind(flags.tor, flags.vpn, flags.proxy, false),
		riskLevel:      riskLevelFromText(stringValue(payload, "risk", "risk_level")),
		riskScore:      clampScore(scoreFromFlags(flags)),
		signals:        signalsFromFlags(flags),
		countryCode:    stringValue(payload, "geo.country_code", "country"),
		region:         stringValue(payload, "geo.region", "region"),
		city:           stringValue(payload, "geo.city", "city"),
		asn:            asnValue(stringValue(payload, "as.asn", "asn.asn")),
		organization:   stringValue(payload, "as.name", "asn.name", "company.name", "org"),
		isp:            stringValue(payload, "as.name", "asn.name", "org"),
	}
}
