package ipfraud

import (
	"context"
	"net/http"
	"strings"
	"time"

	proxyruntimev1 "github.com/byte-v-forge/common-lib/gen/go/byte/v/forge/contracts/proxyruntime/v1"
)

type ip2LocationProvider struct {
	httpProvider
}

const ip2LocationEndpoint = "https://api.ip2location.io/?ip={ip}"

type ip2LocationPlugin struct{}

func init() { Register(ip2LocationPlugin{}) }

func (ip2LocationPlugin) Kind() proxyruntimev1.ProxyIPFraudProviderKind {
	return proxyruntimev1.ProxyIPFraudProviderKind_PROXY_IP_FRAUD_PROVIDER_KIND_IP2LOCATION
}
func (ip2LocationPlugin) ProviderID() string      { return "ip2location" }
func (ip2LocationPlugin) DisplayName() string     { return "IP2Location.io" }
func (ip2LocationPlugin) DefaultWeight() uint32   { return 80 }
func (ip2LocationPlugin) SupportsAnonymous() bool { return true }
func (ip2LocationPlugin) SupportsAPIKey() bool    { return true }
func (ip2LocationPlugin) Auth(keys []string, anonymous bool) AuthConfig {
	if anonymous {
		return AuthConfig{Anonymous: &AnonymousAuthConfig{}}
	}
	return AuthConfig{APIKey: &APIKeyAuthConfig{Keys: append([]string(nil), keys...), Placement: "query", Name: "key"}}
}
func (ip2LocationPlugin) New(client *http.Client, cfg ProviderConfig, cooldown time.Duration) provider {
	return &ip2LocationProvider{httpProvider: newHTTPProvider(client, ip2LocationEndpoint, cfg.Auth, cooldown)}
}

func (p *ip2LocationProvider) lookup(ctx context.Context, ip string) (report, error) {
	payload, err := p.lookupJSON(ctx, ip)
	if err != nil {
		return report{}, err
	}
	if stringValue(payload, "message", "error") != "" && stringValue(payload, "country_code") == "" {
		return report{}, errProviderUnavailable
	}
	return parseIP2Location(payload), nil
}

func parseIP2Location(payload map[string]any) report {
	proxyType := strings.ToUpper(stringValue(payload, "proxy.proxy_type", "proxy_type"))
	threat := strings.ToUpper(stringValue(payload, "proxy.threat", "threat"))
	usageType := strings.ToUpper(stringValue(payload, "usage_type", "as_info.as_usage_type", "address_type"))
	flags := riskFlags{
		bogon:      boolValue(payload, "proxy.is_bogon"),
		datacenter: boolValue(payload, "proxy.is_data_center") || usageType == "DCH" || usageType == "CDN" || proxyType == "DCH",
		hosting:    boolValue(payload, "proxy.is_data_center") || usageType == "DCH" || usageType == "CDN" || proxyType == "DCH",
		proxy:      boolValue(payload, "is_proxy", "proxy.is_public_proxy", "proxy.is_web_proxy") || proxyType == "PUB" || proxyType == "WEB",
		vpn:        boolValue(payload, "proxy.is_vpn", "proxy.is_consumer_privacy_network") || proxyType == "VPN" || proxyType == "CPN",
		tor:        boolValue(payload, "proxy.is_tor") || proxyType == "TOR",
		abuser:     boolValue(payload, "proxy.is_spammer", "proxy.is_scanner", "proxy.is_botnet") || threat == "SPAM" || threat == "SCANNER" || threat == "BOTNET",
		crawler:    boolValue(payload, "proxy.is_web_crawler", "proxy.is_ai_crawler") || proxyType == "SES" || proxyType == "AIC",
		mobile:     usageType == "MOB" || hasNonEmptyString(payload, "mobile_brand"),
	}
	networkKind := networkKindWithFlags(classifyNetworkKind(usageType, proxyType), flags)
	score, ok := floatValue(payload, "fraud_score", "proxy.fraud_score")
	if !ok {
		score = scoreFromFlags(flags)
	}
	return report{
		networkKind:    networkKind,
		anonymizerKind: classifyAnonymizerKind(flags.tor, flags.vpn, flags.proxy, flags.crawler),
		riskLevel:      riskLevelFromText(stringValue(payload, "risk", "risk_level", "threat_level")),
		riskScore:      clampScore(score),
		signals:        signalsFromFlags(flags),
		countryCode:    stringValue(payload, "country_code"),
		region:         stringValue(payload, "region_name", "region"),
		city:           stringValue(payload, "city_name", "city"),
		asn:            asnValue(stringValue(payload, "asn", "as_info.as_number")),
		organization:   stringValue(payload, "as", "as_info.as_name", "organization"),
		isp:            stringValue(payload, "isp"),
	}
}
