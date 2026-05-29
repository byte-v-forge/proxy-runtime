package ipfraud

import (
	"context"
	"net/http"
	"strings"
	"time"

	proxyruntimev1 "github.com/byte-v-forge/common-lib/gen/go/byte/v/forge/contracts/proxyruntime/v1"
)

type ipAPIComProvider struct {
	httpProvider
}

const ipAPIComEndpoint = "http://ip-api.com/json/{ip}?fields=status,message,query,countryCode,regionName,city,isp,org,as,asname,mobile,proxy,hosting"

type ipAPIComPlugin struct{}

func init() { Register(ipAPIComPlugin{}) }

func (ipAPIComPlugin) Kind() proxyruntimev1.ProxyIPFraudProviderKind {
	return proxyruntimev1.ProxyIPFraudProviderKind_PROXY_IP_FRAUD_PROVIDER_KIND_IP_API_COM
}
func (ipAPIComPlugin) ProviderID() string      { return "ip-api-com" }
func (ipAPIComPlugin) DisplayName() string     { return "IP-API.com" }
func (ipAPIComPlugin) DefaultWeight() uint32   { return 40 }
func (ipAPIComPlugin) SupportsAnonymous() bool { return true }
func (ipAPIComPlugin) SupportsAPIKey() bool    { return false }
func (ipAPIComPlugin) Auth(_ []string, _ bool) AuthConfig {
	return AuthConfig{Anonymous: &AnonymousAuthConfig{}}
}
func (ipAPIComPlugin) New(client *http.Client, cfg ProviderConfig, cooldown time.Duration) provider {
	return &ipAPIComProvider{httpProvider: newHTTPProvider(client, ipAPIComEndpoint, cfg.Auth, cooldown)}
}

func (p *ipAPIComProvider) lookup(ctx context.Context, ip string) (report, error) {
	payload, err := p.lookupJSON(ctx, ip)
	if err != nil {
		return report{}, err
	}
	if strings.ToLower(stringValue(payload, "status")) != "success" {
		return report{}, errProviderUnavailable
	}
	return parseIPAPICom(payload), nil
}

func parseIPAPICom(payload map[string]any) report {
	flags := riskFlags{
		hosting: boolValue(payload, "hosting"),
		proxy:   boolValue(payload, "proxy"),
		mobile:  boolValue(payload, "mobile"),
	}
	networkKind := networkKindWithFlags(classifyNetworkKind(
		stringValue(payload, "asname", "org", "isp"),
	), flags)
	asn := strings.Fields(stringValue(payload, "as"))
	asnValueText := ""
	if len(asn) > 0 {
		asnValueText = asn[0]
	}
	return report{
		networkKind:    networkKind,
		anonymizerKind: classifyAnonymizerKind(flags.tor, flags.vpn, flags.proxy, false),
		riskLevel:      riskLevelFromText(stringValue(payload, "risk", "risk_level")),
		riskScore:      clampScore(scoreFromFlags(flags)),
		signals:        signalsFromFlags(flags),
		countryCode:    stringValue(payload, "countryCode"),
		region:         stringValue(payload, "regionName"),
		city:           stringValue(payload, "city"),
		asn:            asnValue(asnValueText),
		organization:   firstNonEmpty(stringValue(payload, "org"), stringValue(payload, "asname")),
		isp:            stringValue(payload, "isp"),
	}
}
