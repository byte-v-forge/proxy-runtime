package ipfraud

import (
	"context"
	"net/http"
	"time"

	proxyruntimev1 "github.com/byte-v-forge/common-lib/gen/go/byte/v/forge/contracts/proxyruntime/v1"
)

type ipQualityScoreProvider struct{ httpProvider }

const ipQualityScoreEndpoint = "https://www.ipqualityscore.com/api/json/ip/{key}/{ip}?strictness=1&allow_public_access_points=true"

type ipQualityScorePlugin struct{}

func init() { Register(ipQualityScorePlugin{}) }

func (ipQualityScorePlugin) Kind() proxyruntimev1.ProxyIPFraudProviderKind {
	return proxyruntimev1.ProxyIPFraudProviderKind_PROXY_IP_FRAUD_PROVIDER_KIND_IPQUALITYSCORE
}
func (ipQualityScorePlugin) ProviderID() string      { return "ipqualityscore" }
func (ipQualityScorePlugin) DisplayName() string     { return "IPQualityScore" }
func (ipQualityScorePlugin) DefaultWeight() uint32   { return 95 }
func (ipQualityScorePlugin) SupportsAnonymous() bool { return false }
func (ipQualityScorePlugin) SupportsAPIKey() bool    { return true }
func (ipQualityScorePlugin) Auth(keys []string, _ bool) AuthConfig {
	return AuthConfig{APIKey: &APIKeyAuthConfig{Keys: append([]string(nil), keys...), Placement: "path"}}
}
func (ipQualityScorePlugin) New(client *http.Client, cfg ProviderConfig, cooldown time.Duration) provider {
	return &ipQualityScoreProvider{httpProvider: newHTTPProvider(client, ipQualityScoreEndpoint, cfg.Auth, cooldown)}
}

func (p *ipQualityScoreProvider) lookup(ctx context.Context, ip string) (report, error) {
	payload, err := p.lookupJSON(ctx, ip)
	if err != nil {
		return report{}, err
	}
	if _, ok := pathValue(payload, "success"); ok && !boolValue(payload, "success") {
		return report{}, errProviderUnavailable
	}
	return parseIPQualityScore(payload), nil
}

func parseIPQualityScore(payload map[string]any) report {
	flags := riskFlags{
		datacenter: boolValue(payload, "hosting") || compactLower(stringValue(payload, "connection_type")) == "data center",
		hosting:    boolValue(payload, "hosting") || compactLower(stringValue(payload, "connection_type")) == "data center",
		proxy:      boolValue(payload, "proxy"),
		vpn:        boolValue(payload, "vpn") || boolValue(payload, "active_vpn"),
		tor:        boolValue(payload, "tor") || boolValue(payload, "active_tor"),
		abuser: boolValue(payload, "recent_abuse") ||
			boolValue(payload, "frequent_abuser") ||
			boolValue(payload, "high_risk_attacks") ||
			boolValue(payload, "bot_status") ||
			boolValue(payload, "security_scanner"),
		crawler: boolValue(payload, "is_crawler"),
		mobile:  boolValue(payload, "mobile"),
	}
	score, ok := floatValue(payload, "fraud_score", "risk_score")
	if !ok {
		score = scoreFromFlags(flags)
	}
	return report{
		networkKind:    networkKindWithFlags(classifyNetworkKind(stringValue(payload, "connection_type")), flags),
		anonymizerKind: classifyAnonymizerKind(flags.tor, flags.vpn, flags.proxy, flags.crawler),
		riskLevel:      riskLevelFromText(stringValue(payload, "risk", "risk_level")),
		riskScore:      clampScore(score),
		signals:        signalsFromFlags(flags),
		countryCode:    stringValue(payload, "country_code"),
		region:         stringValue(payload, "region"),
		city:           stringValue(payload, "city"),
		asn:            asnValue(stringValue(payload, "ASN", "asn")),
		organization:   stringValue(payload, "organization"),
		isp:            stringValue(payload, "ISP", "isp"),
	}
}
