package ipfraud

import (
	"context"
	"net/http"
	"time"

	proxyruntimev1 "github.com/byte-v-forge/common-lib/gen/go/byte/v/forge/contracts/proxyruntime/v1"
)

type ipapiProvider struct {
	httpProvider
}

const ipapiEndpoint = "https://api.ipapi.is?q={ip}"

type ipapiPlugin struct{}

func init() { Register(ipapiPlugin{}) }

func (ipapiPlugin) Kind() proxyruntimev1.ProxyIPFraudProviderKind {
	return proxyruntimev1.ProxyIPFraudProviderKind_PROXY_IP_FRAUD_PROVIDER_KIND_IPAPI
}
func (ipapiPlugin) ProviderID() string      { return "ipapi" }
func (ipapiPlugin) DisplayName() string     { return "ipapi.is" }
func (ipapiPlugin) DefaultWeight() uint32   { return 90 }
func (ipapiPlugin) SupportsAnonymous() bool { return true }
func (ipapiPlugin) SupportsAPIKey() bool    { return true }
func (ipapiPlugin) Auth(keys []string, anonymous bool) AuthConfig {
	if anonymous {
		return AuthConfig{Anonymous: &AnonymousAuthConfig{}}
	}
	return AuthConfig{APIKey: &APIKeyAuthConfig{Keys: append([]string(nil), keys...), Placement: "query", Name: "key"}}
}
func (ipapiPlugin) New(client *http.Client, cfg ProviderConfig, cooldown time.Duration) provider {
	return &ipapiProvider{
		httpProvider: newHTTPProvider(client, ipapiEndpoint, cfg.Auth, cooldown),
	}
}

func (p *ipapiProvider) lookup(ctx context.Context, ip string) (report, error) {
	payload, err := p.lookupJSON(ctx, ip)
	if err != nil {
		return report{}, err
	}
	if stringValue(payload, "error") != "" {
		return report{}, errProviderUnavailable
	}
	return parseIPAPI(payload), nil
}

func parseIPAPI(payload map[string]any) report {
	flags := riskFlags{
		bogon:      boolValue(payload, "is_bogon", "bogon"),
		datacenter: boolValue(payload, "is_datacenter", "datacenter"),
		hosting:    boolValue(payload, "is_hosting", "hosting"),
		proxy:      boolValue(payload, "is_proxy", "proxy"),
		vpn:        boolValue(payload, "is_vpn", "vpn"),
		tor:        boolValue(payload, "is_tor", "tor"),
		abuser:     boolValue(payload, "is_abuser", "abuser"),
		crawler:    boolValue(payload, "is_crawler", "crawler"),
		mobile:     boolValue(payload, "is_mobile", "mobile"),
		satellite:  boolValue(payload, "is_satellite", "satellite"),
		broadcast:  boolValue(payload, "is_broadcast", "broadcast"),
		anycast:    boolValue(payload, "is_anycast", "anycast"),
	}
	score, ok := floatValue(payload, "risk_score", "fraud_score", "score", "abuse.score")
	if !ok {
		score = scoreFromFlags(flags)
	}
	networkKind := networkKindWithFlags(classifyNetworkKind(
		stringValue(payload, "company.type", "asn.type", "connection_type", "type"),
	), flags)
	return report{
		networkKind:    networkKind,
		anonymizerKind: classifyAnonymizerKind(flags.tor, flags.vpn, flags.proxy, flags.crawler),
		riskLevel:      riskLevelFromText(stringValue(payload, "risk", "risk_level")),
		riskScore:      clampScore(score),
		signals:        signalsFromFlags(flags),
		countryCode:    stringValue(payload, "location.country_code", "country_code", "countryCode"),
		region:         stringValue(payload, "location.state", "region", "state"),
		city:           stringValue(payload, "location.city", "city"),
		asn:            asnValue(stringValue(payload, "asn.asn", "asn", "autonomous_system_number")),
		organization:   stringValue(payload, "asn.org", "company.name", "organization", "org"),
		isp:            stringValue(payload, "isp", "company.name"),
	}
}
