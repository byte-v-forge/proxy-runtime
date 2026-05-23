package ten24

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	proxyruntimev1 "github.com/byte-v-forge/proxy-runtime/gen/byte/v/forge/contracts/proxyruntime/v1"
	"github.com/byte-v-forge/proxy-runtime/internal/provider"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	defaultStickyMinutes = 30
	minStickyMinutes     = 1
	maxStickyMinutes     = 120
)

type Provider struct {
	cfg        Config
	httpClient *http.Client
}

func New(cfg Config, httpClient *http.Client) *Provider {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Provider{cfg: cfg, httpClient: httpClient}
}

func (p *Provider) Name() string {
	return "1024proxy"
}

func (p *Provider) Descriptor() *proxyruntimev1.ProxyProviderDescriptor {
	capabilities := []proxyruntimev1.ProxyCapability{
		proxyruntimev1.ProxyCapability_PROXY_CAPABILITY_CHAINING,
		proxyruntimev1.ProxyCapability_PROXY_CAPABILITY_POOL_REFRESH,
		proxyruntimev1.ProxyCapability_PROXY_CAPABILITY_UNIFIED_EGRESS_GATEWAY,
	}
	upstreamKinds := []proxyruntimev1.ProxyUpstreamKind{}
	rotationModes := []proxyruntimev1.ProxyRotationMode{}
	if p.cfg.APIURL != "" {
		capabilities = append(capabilities, proxyruntimev1.ProxyCapability_PROXY_CAPABILITY_API_POOL)
		upstreamKinds = append(upstreamKinds, proxyruntimev1.ProxyUpstreamKind_PROXY_UPSTREAM_KIND_PROXY_POOL)
		rotationModes = append(rotationModes,
			proxyruntimev1.ProxyRotationMode_PROXY_ROTATION_MODE_PER_REQUEST,
			proxyruntimev1.ProxyRotationMode_PROXY_ROTATION_MODE_SCHEDULED_POOL_REFRESH,
		)
	}
	supportsSession := p.supportsCredentialSession()
	if supportsSession {
		capabilities = append(capabilities,
			proxyruntimev1.ProxyCapability_PROXY_CAPABILITY_STICKY_SESSION,
			proxyruntimev1.ProxyCapability_PROXY_CAPABILITY_ACTIVE_SESSION_ROTATION,
			proxyruntimev1.ProxyCapability_PROXY_CAPABILITY_USERNAME_PARAMETER_SESSION,
		)
		upstreamKinds = append(upstreamKinds, proxyruntimev1.ProxyUpstreamKind_PROXY_UPSTREAM_KIND_DYNAMIC_IP)
		rotationModes = append(rotationModes,
			proxyruntimev1.ProxyRotationMode_PROXY_ROTATION_MODE_PER_REQUEST,
			proxyruntimev1.ProxyRotationMode_PROXY_ROTATION_MODE_STICKY_SESSION,
		)
	}
	return &proxyruntimev1.ProxyProviderDescriptor{
		ProviderId:               p.Name(),
		Provider:                 proxyruntimev1.ProxyProvider_PROXY_PROVIDER_1024PROXY,
		DisplayName:              "1024Proxy",
		Capabilities:             capabilities,
		Protocols:                []proxyruntimev1.ProxyProtocol{protocolEnum(defaultProtocol(p.cfg.Protocol))},
		SupportsActiveNewSession: supportsSession,
		MinStickyTtlMinutes:      minStickyMinutes,
		MaxStickyTtlMinutes:      maxStickyMinutes,
		UpstreamKinds:            upstreamKinds,
		RotationModes:            rotationModes,
	}
}

func (p *Provider) Fetch(ctx context.Context, session *proxyruntimev1.ProxySession) ([]provider.Node, error) {
	if session == nil && p.cfg.APIURL != "" {
		return p.fetchAPI(ctx)
	}
	node, err := p.credentialNode(session)
	if err != nil {
		return nil, err
	}
	return []provider.Node{node}, nil
}

func (p *Provider) CreateSession(_ context.Context, req *proxyruntimev1.CreateProxySessionRequest) (*proxyruntimev1.ProxySession, error) {
	if !p.supportsCredentialSession() {
		return nil, provider.ErrUnsupportedCapability
	}
	policy := p.sessionPolicy(req.GetPolicy())
	if policy.Mode == proxyruntimev1.ProxySessionMode_PROXY_SESSION_MODE_UNSPECIFIED {
		policy.Mode = proxyruntimev1.ProxySessionMode_PROXY_SESSION_MODE_STICKY
	}
	if policy.Mode != proxyruntimev1.ProxySessionMode_PROXY_SESSION_MODE_STICKY {
		return nil, provider.ErrUnsupportedCapability
	}
	policy.UpstreamKind = proxyruntimev1.ProxyUpstreamKind_PROXY_UPSTREAM_KIND_DYNAMIC_IP
	policy.RotationMode = proxyruntimev1.ProxyRotationMode_PROXY_ROTATION_MODE_STICKY_SESSION
	if policy.Asn != "" && (policy.State != "" || policy.City != "") {
		return nil, errors.New("1024proxy ASN cannot be combined with state or city")
	}
	sessionID, err := randomSessionID()
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	return &proxyruntimev1.ProxySession{
		SessionId: sessionID,
		Provider:  proxyruntimev1.ProxyProvider_PROXY_PROVIDER_1024PROXY,
		Policy:    policy,
		CreatedAt: timestamppb.New(now),
		ExpiresAt: timestamppb.New(now.Add(time.Duration(policy.StickyTtlMinutes) * time.Minute)),
		Labels: map[string]string{
			"provider":     p.Name(),
			"session_mode": "username_parameter",
		},
	}, nil
}

func (p *Provider) credentialNode(session *proxyruntimev1.ProxySession) (provider.Node, error) {
	username := p.buildUsername(session)
	proxyURL := url.URL{
		Scheme: defaultProtocol(p.cfg.Protocol),
		Host:   p.cfg.ProxyAddr,
		User:   url.UserPassword(username, p.cfg.Password),
	}
	sessionID := ""
	if session != nil {
		sessionID = session.SessionId
	} else {
		sessionID = p.cfg.SessionID
	}
	rotationMode := proxyruntimev1.ProxyRotationMode_PROXY_ROTATION_MODE_PER_REQUEST
	if sessionID != "" {
		rotationMode = proxyruntimev1.ProxyRotationMode_PROXY_ROTATION_MODE_STICKY_SESSION
	}
	return provider.Node{
		ID:           "1024proxy-credential",
		URL:          &proxyURL,
		Provider:     proxyruntimev1.ProxyProvider_PROXY_PROVIDER_1024PROXY,
		SessionID:    sessionID,
		UpstreamKind: proxyruntimev1.ProxyUpstreamKind_PROXY_UPSTREAM_KIND_DYNAMIC_IP,
		RotationMode: rotationMode,
		Labels: map[string]string{
			"mode":     "credential",
			"protocol": defaultProtocol(p.cfg.Protocol),
		},
	}, nil
}

func (p *Provider) buildUsername(session *proxyruntimev1.ProxySession) string {
	policy := p.sessionPolicy(nil)
	sessionID := p.cfg.SessionID
	if session != nil {
		policy = p.sessionPolicy(session.Policy)
		sessionID = session.SessionId
	}
	parts := []string{p.cfg.Username}
	appendPair := func(key string, value string) {
		value = strings.TrimSpace(value)
		if value != "" {
			parts = append(parts, key, value)
		}
	}
	appendPair("region", policy.Region)
	appendPair("st", policy.State)
	appendPair("city", policy.City)
	appendPair("asn", policy.Asn)
	if sessionID != "" {
		appendPair("sid", sessionID)
		appendPair("t", strconv.Itoa(int(policy.StickyTtlMinutes)))
	}
	return strings.Join(parts, "-")
}

func (p *Provider) sessionPolicy(input *proxyruntimev1.ProxySessionPolicy) *proxyruntimev1.ProxySessionPolicy {
	policy := &proxyruntimev1.ProxySessionPolicy{
		Mode:             proxyruntimev1.ProxySessionMode_PROXY_SESSION_MODE_STICKY,
		Region:           p.cfg.Region,
		State:            p.cfg.State,
		City:             p.cfg.City,
		Asn:              p.cfg.ASN,
		StickyTtlMinutes: uint32(defaultStickyTTL(p.cfg.StickyMinutes)),
		UpstreamKind:     proxyruntimev1.ProxyUpstreamKind_PROXY_UPSTREAM_KIND_DYNAMIC_IP,
		RotationMode:     proxyruntimev1.ProxyRotationMode_PROXY_ROTATION_MODE_STICKY_SESSION,
	}
	if input == nil {
		return policy
	}
	if input.Mode != proxyruntimev1.ProxySessionMode_PROXY_SESSION_MODE_UNSPECIFIED {
		policy.Mode = input.Mode
	}
	if input.Region != "" {
		policy.Region = input.Region
	}
	if input.State != "" {
		policy.State = input.State
	}
	if input.City != "" {
		policy.City = input.City
	}
	if input.Asn != "" {
		policy.Asn = input.Asn
	}
	if input.StickyTtlMinutes != 0 {
		policy.StickyTtlMinutes = input.StickyTtlMinutes
	}
	if policy.StickyTtlMinutes < minStickyMinutes {
		policy.StickyTtlMinutes = minStickyMinutes
	}
	if policy.StickyTtlMinutes > maxStickyMinutes {
		policy.StickyTtlMinutes = maxStickyMinutes
	}
	if len(input.Labels) > 0 {
		policy.Labels = make(map[string]string, len(input.Labels))
		for key, value := range input.Labels {
			policy.Labels[key] = value
		}
	}
	return policy
}

func (p *Provider) fetchAPI(ctx context.Context) ([]provider.Node, error) {
	apiURL, err := p.buildAPIURL()
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build 1024proxy API request: %w", err)
	}
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call 1024proxy API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read 1024proxy API response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("1024proxy API returned status %d", resp.StatusCode)
	}

	nodes, err := p.parseAPIResponse(body)
	if err != nil {
		return nil, err
	}
	if len(nodes) == 0 {
		return nil, errors.New("1024proxy API returned no proxies")
	}
	return nodes, nil
}

func (p *Provider) buildAPIURL() (string, error) {
	parsed, err := url.Parse(p.cfg.APIURL)
	if err != nil {
		return "", errors.New("invalid 1024proxy API URL")
	}
	query := parsed.Query()
	setQuery(query, "region", p.cfg.APIRegion)
	setQuery(query, "format", p.cfg.APIFormat)
	setQuery(query, "time", p.cfg.APITime)
	setQuery(query, "num", p.cfg.APINum)
	setQuery(query, "type", p.cfg.APIType)
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func (p *Provider) parseAPIResponse(body []byte) ([]provider.Node, error) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return nil, nil
	}
	if trimmed[0] == '{' || trimmed[0] == '[' || p.cfg.APIType == "json" {
		return p.parseJSON(trimmed)
	}
	return p.parseText(string(trimmed))
}

func (p *Provider) parseJSON(body []byte) ([]provider.Node, error) {
	var payload any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("parse 1024proxy JSON response: %w", err)
	}
	rawValues := collectProxyValues(payload)
	return p.parseRawValues(rawValues)
}

func (p *Provider) parseText(body string) ([]provider.Node, error) {
	rawValues := strings.FieldsFunc(body, func(r rune) bool {
		return r == '\n' || r == '\r'
	})
	return p.parseRawValues(rawValues)
}

func (p *Provider) parseRawValues(rawValues []string) ([]provider.Node, error) {
	nodes := make([]provider.Node, 0, len(rawValues))
	for _, raw := range rawValues {
		proxyURL, err := provider.ParseProxy(raw, defaultProtocol(p.cfg.Protocol))
		if err != nil {
			continue
		}
		nodes = append(nodes, provider.Node{
			ID:           fmt.Sprintf("1024proxy-api-%d", len(nodes)),
			URL:          proxyURL,
			Provider:     proxyruntimev1.ProxyProvider_PROXY_PROVIDER_1024PROXY,
			UpstreamKind: proxyruntimev1.ProxyUpstreamKind_PROXY_UPSTREAM_KIND_PROXY_POOL,
			RotationMode: proxyruntimev1.ProxyRotationMode_PROXY_ROTATION_MODE_PER_REQUEST,
			Labels: map[string]string{
				"mode": "api",
			},
		})
	}
	return nodes, nil
}

func collectProxyValues(value any) []string {
	var values []string
	collectValue(value, &values)
	return values
}

func collectValue(value any, values *[]string) {
	switch typed := value.(type) {
	case string:
		appendLines(typed, values)
	case []any:
		for _, item := range typed {
			collectValue(item, values)
		}
	case map[string]any:
		if raw := proxyValueFromMap(typed); raw != "" {
			appendLines(raw, values)
			return
		}
		for _, item := range typed {
			collectValue(item, values)
		}
	}
}

func proxyValueFromMap(value map[string]any) string {
	for _, key := range []string{"proxy", "url", "server"} {
		if raw, ok := value[key].(string); ok && strings.TrimSpace(raw) != "" {
			return raw
		}
	}
	host, _ := valueString(value, "host", "hostname", "ip")
	port, _ := valueString(value, "port")
	if host == "" || port == "" {
		return ""
	}
	username, _ := valueString(value, "username", "user")
	password, _ := valueString(value, "password", "pass")
	protocol, _ := valueString(value, "protocol", "scheme", "type")
	if protocol == "" {
		protocol = "http"
	}
	if username == "" && password == "" {
		return protocol + "://" + host + ":" + port
	}
	return protocol + "://" + url.UserPassword(username, password).String() + "@" + host + ":" + port
}

func valueString(value map[string]any, keys ...string) (string, bool) {
	for _, key := range keys {
		switch typed := value[key].(type) {
		case string:
			return strings.TrimSpace(typed), true
		case float64:
			return strconv.Itoa(int(typed)), true
		}
	}
	return "", false
}

func appendLines(raw string, values *[]string) {
	for _, line := range strings.FieldsFunc(raw, func(r rune) bool {
		return r == '\n' || r == '\r'
	}) {
		line = strings.TrimSpace(line)
		if line != "" {
			*values = append(*values, line)
		}
	}
}

func setQuery(query url.Values, key string, value string) {
	value = strings.TrimSpace(value)
	if value != "" {
		query.Set(key, value)
	}
}

func defaultProtocol(protocol string) string {
	switch strings.TrimSpace(protocol) {
	case "socks5":
		return "socks5"
	default:
		return "http"
	}
}

func protocolEnum(protocol string) proxyruntimev1.ProxyProtocol {
	switch defaultProtocol(protocol) {
	case "socks5":
		return proxyruntimev1.ProxyProtocol_PROXY_PROTOCOL_SOCKS5
	default:
		return proxyruntimev1.ProxyProtocol_PROXY_PROTOCOL_HTTP
	}
}

func defaultStickyTTL(minutes int) int {
	if minutes < minStickyMinutes || minutes > maxStickyMinutes {
		return defaultStickyMinutes
	}
	return minutes
}

func (p *Provider) supportsCredentialSession() bool {
	return p.cfg.ProxyAddr != "" && p.cfg.Username != "" && p.cfg.Password != ""
}

func randomSessionID() (string, error) {
	var data [8]byte
	if _, err := rand.Read(data[:]); err != nil {
		return "", fmt.Errorf("generate 1024proxy session id: %w", err)
	}
	return hex.EncodeToString(data[:]), nil
}
