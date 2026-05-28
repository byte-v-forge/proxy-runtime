package ipfraud

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	proxyruntimev1 "github.com/byte-v-forge/common-lib/gen/go/byte/v/forge/contracts/proxyruntime/v1"
)

type ffraudProvider struct {
	public  httpProvider
	client  *http.Client
	keys    *keyRing
	useKeys bool
}

const (
	ffraudPublicEndpoint = "https://api.ffraud.com/public/ip/{ip}"
	ffraudAPIEndpoint    = "https://api.ffraud.com/v1/ip/check"
)

type ffraudPlugin struct{}

func init() { Register(ffraudPlugin{}) }

func (ffraudPlugin) Kind() proxyruntimev1.ProxyIPFraudProviderKind {
	return proxyruntimev1.ProxyIPFraudProviderKind_PROXY_IP_FRAUD_PROVIDER_KIND_FFRAUD
}
func (ffraudPlugin) ProviderID() string      { return "ffraud" }
func (ffraudPlugin) DisplayName() string     { return "FFraud" }
func (ffraudPlugin) DefaultWeight() uint32   { return 100 }
func (ffraudPlugin) SupportsAnonymous() bool { return true }
func (ffraudPlugin) SupportsAPIKey() bool    { return true }
func (ffraudPlugin) Auth(keys []string, anonymous bool) AuthConfig {
	if anonymous {
		return AuthConfig{Anonymous: &AnonymousAuthConfig{}}
	}
	return AuthConfig{APIKey: &APIKeyAuthConfig{Keys: append([]string(nil), keys...), Placement: "header", Name: "X-API-Key"}}
}
func (ffraudPlugin) New(client *http.Client, cfg ProviderConfig, cooldown time.Duration) provider {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	keys := []string{}
	if cfg.Auth.APIKey != nil {
		keys = append(keys, cfg.Auth.APIKey.Keys...)
	}
	return &ffraudProvider{
		public:  newHTTPProvider(client, ffraudPublicEndpoint, AuthConfig{Anonymous: &AnonymousAuthConfig{}}, cooldown),
		client:  client,
		keys:    newKeyRing(keys, cooldown),
		useKeys: len(keys) > 0,
	}
}

func (p *ffraudProvider) lookup(ctx context.Context, ip string) (report, error) {
	var payload map[string]any
	var err error
	if p.useKeys {
		payload, err = p.lookupAuthenticatedJSON(ctx, ip)
	} else {
		payload, err = p.public.lookupJSON(ctx, ip)
	}
	if err != nil {
		return report{}, err
	}
	return parseFFraud(payload), nil
}

func (p *ffraudProvider) lookupAuthenticatedJSON(ctx context.Context, ip string) (map[string]any, error) {
	attempts := p.keys.size()
	if attempts == 0 {
		attempts = 1
	}
	for attempt := 0; attempt < attempts; attempt++ {
		keyIndex, key, ok := p.keys.nextAvailable(time.Now())
		if !ok {
			return nil, errQuotaExhausted
		}
		payload, retryAfter, err := p.requestAuthenticatedJSON(ctx, ip, key)
		if err == nil {
			return payload, nil
		}
		if err == errQuotaExhausted {
			p.keys.markUnavailable(keyIndex, retryAfter)
			continue
		}
		return nil, err
	}
	return nil, errQuotaExhausted
}

func (p *ffraudProvider) requestAuthenticatedJSON(ctx context.Context, ip string, key string) (map[string]any, time.Duration, error) {
	body, err := json.Marshal(map[string]string{"ip": ip})
	if err != nil {
		return nil, 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ffraudAPIEndpoint, bytes.NewReader(body))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", key)
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, 0, err
	}
	if retryAfter, quota := quotaResponse(resp.StatusCode, resp.Header, raw); quota {
		return nil, retryAfter, errQuotaExhausted
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, 0, fmt.Errorf("IP fraud request failed")
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, 0, fmt.Errorf("decode IP fraud response")
	}
	if quotaPayload(payload) {
		return nil, 0, errQuotaExhausted
	}
	return payload, 0, nil
}

func parseFFraud(payload map[string]any) report {
	flags := riskFlags{
		bogon:      boolValue(payload, "bogon", "is_bogon"),
		datacenter: boolValue(payload, "datacenter", "is_datacenter"),
		hosting:    boolValue(payload, "hosting", "is_hosting"),
		proxy:      boolValue(payload, "proxy", "is_proxy"),
		vpn:        boolValue(payload, "vpn", "is_vpn"),
		tor:        boolValue(payload, "tor", "is_tor"),
		abuser:     boolValue(payload, "abuser", "is_abuser"),
		crawler:    boolValue(payload, "crawler", "is_crawler"),
		mobile:     boolValue(payload, "mobile", "is_mobile"),
		satellite:  boolValue(payload, "satellite", "is_satellite"),
		broadcast:  boolValue(payload, "broadcast", "is_broadcast"),
		anycast:    boolValue(payload, "anycast", "is_anycast"),
	}
	score, ok := floatValue(payload, "fraud_score", "risk_score", "score", "abuse_score")
	if !ok {
		score = scoreFromFlags(flags)
	}
	networkKind := classifyNetworkKind(
		stringValue(payload, "connection_type", "connection.type", "usage_type", "type"),
	)
	if flags.datacenter || flags.hosting {
		networkKind = chooseNetworkKind(networkKind, proxyruntimev1.ProxyIPNetworkKind_PROXY_IP_NETWORK_KIND_DATACENTER)
	}
	if flags.mobile {
		networkKind = chooseNetworkKind(networkKind, proxyruntimev1.ProxyIPNetworkKind_PROXY_IP_NETWORK_KIND_MOBILE)
	}
	if flags.satellite {
		networkKind = chooseNetworkKind(networkKind, proxyruntimev1.ProxyIPNetworkKind_PROXY_IP_NETWORK_KIND_SATELLITE)
	}
	if flags.broadcast {
		networkKind = chooseNetworkKind(networkKind, proxyruntimev1.ProxyIPNetworkKind_PROXY_IP_NETWORK_KIND_BROADCAST)
	}
	if flags.anycast {
		networkKind = chooseNetworkKind(networkKind, proxyruntimev1.ProxyIPNetworkKind_PROXY_IP_NETWORK_KIND_ANYCAST)
	}
	return report{
		networkKind:    networkKind,
		anonymizerKind: classifyAnonymizerKind(flags.tor, flags.vpn, flags.proxy, flags.crawler),
		riskLevel:      riskLevelFromText(stringValue(payload, "risk", "risk_level")),
		riskScore:      clampScore(score),
		signals:        signalsFromFlags(flags),
		countryCode:    stringValue(payload, "country_code", "countryCode", "geo.country_code", "geo.country", "location.country_code", "country"),
		region:         stringValue(payload, "region", "state", "geo.region", "location.state"),
		city:           stringValue(payload, "city", "geo.city", "location.city"),
		asn:            asnValue(stringValue(payload, "asn", "ASN", "as.number", "autonomous_system_number")),
		organization:   stringValue(payload, "organization", "org", "as.organization", "as.org"),
		isp:            stringValue(payload, "isp", "ISP", "connection.isp"),
	}
}
