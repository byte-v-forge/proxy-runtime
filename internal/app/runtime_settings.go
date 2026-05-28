package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	proxyruntimev1 "github.com/byte-v-forge/common-lib/gen/go/byte/v/forge/contracts/proxyruntime/v1"
	"github.com/byte-v-forge/proxy-runtime/internal/ipfraud"
	"github.com/byte-v-forge/proxy-runtime/internal/provider/accountproxy"
)

type runtimeSettingsStore struct {
	store  *PostgresStore
	logger *slog.Logger
	mu     sync.Mutex
}

type runtimeSettingsFile struct {
	EdgeCanary         edgeCanarySettings         `json:"edge_canary"`
	IPFraudProviders   []ipFraudProviderSetting   `json:"ip_fraud_providers"`
	DynamicIPProviders []dynamicIPProviderSetting `json:"dynamic_ip_providers"`
}

type edgeCanarySettings struct {
	Enabled *bool  `json:"enabled,omitempty"`
	URL     string `json:"url"`
	Token   string `json:"token"`
}

type ipFraudProviderSetting struct {
	ID        string                                  `json:"id"`
	Weight    uint32                                  `json:"weight"`
	Kind      proxyruntimev1.ProxyIPFraudProviderKind `json:"kind"`
	Anonymous bool                                    `json:"anonymous"`
	APIKeys   []string                                `json:"api_keys"`
}

type dynamicIPProviderSetting struct {
	ProviderID string                    `json:"provider_id"`
	Gateways   []dynamicIPGatewaySetting `json:"gateways"`
}

type dynamicIPGatewaySetting struct {
	GatewayID       string                         `json:"gateway_id"`
	DisplayName     string                         `json:"display_name"`
	Addr            string                         `json:"addr"`
	RegionCodes     []string                       `json:"region_codes"`
	Protocols       []proxyruntimev1.ProxyProtocol `json:"protocols"`
	DefaultProtocol proxyruntimev1.ProxyProtocol   `json:"default_protocol"`
}

func newRuntimeSettingsStore(store *PostgresStore, logger *slog.Logger) *runtimeSettingsStore {
	if logger == nil {
		logger = slog.Default()
	}
	return &runtimeSettingsStore{store: store, logger: logger}
}

func (s *runtimeSettingsStore) view() (*proxyruntimev1.ProxyRuntimeSettings, error) {
	settings, err := s.load()
	if err != nil {
		return nil, err
	}
	return settings.view(), nil
}

func (s *runtimeSettingsStore) update(req *proxyruntimev1.UpdateProxyRuntimeSettingsRequest) (*proxyruntimev1.ProxyRuntimeSettings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	current, err := s.loadLocked()
	if err != nil {
		return nil, err
	}
	settings, err := settingsFromRequest(req, current)
	if err != nil {
		return nil, err
	}
	if err := s.saveLocked(settings); err != nil {
		return nil, err
	}
	return settings.view(), nil
}

func (s *runtimeSettingsStore) load() (runtimeSettingsFile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadLocked()
}

func (s *runtimeSettingsStore) loadLocked() (runtimeSettingsFile, error) {
	if s.store == nil {
		return runtimeSettingsFile{}, nil
	}
	return s.store.LoadRuntimeSettings(context.Background())
}

func (s *runtimeSettingsStore) save(settings runtimeSettingsFile) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveLocked(settings)
}

func (s *runtimeSettingsStore) saveLocked(settings runtimeSettingsFile) error {
	if s.store == nil {
		return nil
	}
	settings.normalize()
	return s.store.SaveRuntimeSettings(context.Background(), settings)
}

func settingsFromRequest(req *proxyruntimev1.UpdateProxyRuntimeSettingsRequest, current runtimeSettingsFile) (runtimeSettingsFile, error) {
	current.normalize()
	edgeCanary := edgeCanaryFromRequest(req.GetEdgeCanary(), current.EdgeCanary)
	settings := runtimeSettingsFile{
		EdgeCanary:         edgeCanary,
		IPFraudProviders:   make([]ipFraudProviderSetting, 0, len(req.GetIpFraudProviders())),
		DynamicIPProviders: make([]dynamicIPProviderSetting, 0, len(req.GetDynamicIpProviders())),
	}
	if settings.EdgeCanary.enabled() && settings.EdgeCanary.URL == "" {
		return runtimeSettingsFile{}, errors.New("edge canary url is required when enabled")
	}
	currentProviders := current.providerSecrets()
	seenProviders := map[string]struct{}{}
	for index, provider := range req.GetIpFraudProviders() {
		kind := provider.GetKind()
		id := strings.TrimSpace(provider.GetProviderId())
		if id == "" {
			id = ipfraud.DefaultProviderID(kind)
		}
		apiKeys := cleanList(provider.GetApiKeys())
		if len(apiKeys) == 0 && !provider.GetClearApiKeys() && !provider.GetAnonymous() {
			apiKeys = currentProviders[providerSecretKey(kind, id)]
		}
		item := ipFraudProviderSetting{
			ID:        id,
			Weight:    provider.GetWeight(),
			Kind:      kind,
			Anonymous: provider.GetAnonymous(),
			APIKeys:   apiKeys,
		}
		if item.Weight == 0 {
			item.Weight = providerDefaultWeight(item.Kind, index)
		}
		if err := item.validate(index); err != nil {
			return runtimeSettingsFile{}, err
		}
		key := providerSecretKey(item.Kind, item.ID)
		if _, exists := seenProviders[key]; exists {
			return runtimeSettingsFile{}, fmt.Errorf("ip_fraud_providers[%d] duplicates provider %q", index, item.ID)
		}
		seenProviders[key] = struct{}{}
		settings.IPFraudProviders = append(settings.IPFraudProviders, item)
	}
	seenDynamicProviders := map[string]struct{}{}
	for index, provider := range req.GetDynamicIpProviders() {
		item := dynamicIPProviderFromProto(provider)
		if err := item.validate(index); err != nil {
			return runtimeSettingsFile{}, err
		}
		if _, exists := seenDynamicProviders[item.ProviderID]; exists {
			return runtimeSettingsFile{}, fmt.Errorf("dynamic_ip_providers[%d] duplicates provider %q", index, item.ProviderID)
		}
		seenDynamicProviders[item.ProviderID] = struct{}{}
		settings.DynamicIPProviders = append(settings.DynamicIPProviders, item)
	}
	settings.normalize()
	return settings, nil
}

func edgeCanaryFromRequest(
	req *proxyruntimev1.ProxyEdgeCanarySettings,
	current edgeCanarySettings,
) edgeCanarySettings {
	if req == nil {
		return current
	}
	enabled := req.GetEnabled()
	settings := edgeCanarySettings{
		Enabled: &enabled,
		URL:     firstNonEmpty(req.GetUrl(), current.URL),
		Token:   strings.TrimSpace(req.GetToken()),
	}
	switch {
	case settings.Token != "":
	case req.GetClearToken():
		settings.Token = ""
	default:
		settings.Token = strings.TrimSpace(current.Token)
	}
	return settings
}

func (s *runtimeSettingsFile) normalize() {
	s.EdgeCanary.URL = strings.TrimSpace(s.EdgeCanary.URL)
	s.EdgeCanary.Token = strings.TrimSpace(s.EdgeCanary.Token)
	for index := range s.IPFraudProviders {
		s.IPFraudProviders[index].normalize()
		if s.IPFraudProviders[index].Weight == 0 {
			s.IPFraudProviders[index].Weight = providerDefaultWeight(s.IPFraudProviders[index].Kind, index)
		}
	}
	for index := range s.DynamicIPProviders {
		s.DynamicIPProviders[index].normalize()
	}
}

func (s edgeCanarySettings) enabled() bool {
	if s.Enabled != nil {
		return *s.Enabled
	}
	return strings.TrimSpace(s.URL) != "" && strings.TrimSpace(s.Token) != ""
}

func (s runtimeSettingsFile) view() *proxyruntimev1.ProxyRuntimeSettings {
	out := &proxyruntimev1.ProxyRuntimeSettings{
		EdgeCanary: &proxyruntimev1.ProxyEdgeCanarySettingsView{
			Url:             s.EdgeCanary.URL,
			TokenConfigured: s.EdgeCanary.Token != "",
			Enabled:         s.EdgeCanary.enabled(),
		},
	}
	for _, provider := range s.IPFraudProviders {
		out.IpFraudProviders = append(out.IpFraudProviders, &proxyruntimev1.ProxyIPFraudProviderSettingsView{
			ProviderId:       provider.ID,
			Weight:           provider.Weight,
			Kind:             provider.Kind,
			Anonymous:        provider.Anonymous,
			ApiKeyConfigured: len(provider.APIKeys) > 0,
			ApiKeyCount:      uint32(len(provider.APIKeys)),
		})
	}
	for _, provider := range s.DynamicIPProviders {
		out.DynamicIpProviders = append(out.DynamicIpProviders, provider.toProto())
	}
	return out
}

func (s runtimeSettingsFile) signature() string {
	data, _ := json.Marshal(s)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func (s runtimeSettingsFile) providerSecrets() map[string][]string {
	secrets := map[string][]string{}
	for _, item := range s.IPFraudProviders {
		secrets[providerSecretKey(item.Kind, item.ID)] = append([]string(nil), item.APIKeys...)
	}
	return secrets
}

func providerSecretKey(kind proxyruntimev1.ProxyIPFraudProviderKind, id string) string {
	return fmt.Sprintf("%d:%s", kind, strings.TrimSpace(id))
}

func (s runtimeSettingsFile) ipFraudProviders() []ipfraud.ProviderConfig {
	providers := make([]ipfraud.ProviderConfig, 0, len(s.IPFraudProviders))
	for _, item := range s.IPFraudProviders {
		provider := ipfraud.ProviderConfig{
			ID:     item.ID,
			Kind:   item.Kind,
			Weight: int(item.Weight),
			Auth:   item.ipFraudAuth(),
		}
		providers = append(providers, provider)
	}
	return providers
}

func (s runtimeSettingsFile) dynamicIPGatewayMap() map[string][]accountproxy.Gateway {
	out := map[string][]accountproxy.Gateway{}
	for _, provider := range s.DynamicIPProviders {
		out[provider.ProviderID] = provider.gateways()
	}
	return out
}

func (s runtimeSettingsFile) dynamicIPGateways(providerID string) []accountproxy.Gateway {
	return s.dynamicIPGatewayMap()[strings.TrimSpace(providerID)]
}

func (p *ipFraudProviderSetting) normalize() {
	p.ID = strings.TrimSpace(p.ID)
	p.APIKeys = cleanList(p.APIKeys)
	if p.ID == "" {
		p.ID = ipfraud.DefaultProviderID(p.Kind)
	}
}

func (p ipFraudProviderSetting) validate(index int) error {
	if !ipfraud.IsProviderKindSupported(p.Kind) {
		return fmt.Errorf("ip_fraud_providers[%d].kind is required", index)
	}
	if p.Anonymous && len(p.APIKeys) > 0 {
		return fmt.Errorf("ip_fraud_providers[%d] must use anonymous or api_keys, not both", index)
	}
	if !p.Anonymous && len(p.APIKeys) == 0 {
		return fmt.Errorf("ip_fraud_providers[%d].api_keys is required when anonymous is false", index)
	}
	return nil
}

func (p ipFraudProviderSetting) ipFraudAuth() ipfraud.AuthConfig {
	if p.Anonymous {
		return ipfraud.AuthConfig{Anonymous: &ipfraud.AnonymousAuthConfig{}}
	}
	plugin, ok := ipfraud.PluginForKind(p.Kind)
	if !ok {
		return ipfraud.AuthConfig{}
	}
	return plugin.Auth(p.APIKeys, false)
}

func dynamicIPProviderFromProto(in *proxyruntimev1.ProxyDynamicIPProviderSettings) dynamicIPProviderSetting {
	if in == nil {
		return dynamicIPProviderSetting{}
	}
	item := dynamicIPProviderSetting{ProviderID: strings.TrimSpace(in.GetProviderId()), Gateways: make([]dynamicIPGatewaySetting, 0, len(in.GetGateways()))}
	for _, gateway := range in.GetGateways() {
		item.Gateways = append(item.Gateways, dynamicIPGatewayFromProto(gateway))
	}
	return item
}

func dynamicIPGatewayFromProto(in *proxyruntimev1.ProxyDynamicIPGatewaySettings) dynamicIPGatewaySetting {
	if in == nil {
		return dynamicIPGatewaySetting{}
	}
	return dynamicIPGatewaySetting{
		GatewayID:       strings.TrimSpace(in.GetGatewayId()),
		DisplayName:     strings.TrimSpace(in.GetDisplayName()),
		Addr:            strings.TrimSpace(in.GetAddr()),
		RegionCodes:     cleanRegionCodes(in.GetRegionCodes()),
		Protocols:       cleanProtocols(in.GetProtocols()),
		DefaultProtocol: in.GetDefaultProtocol(),
	}
}

func (p *dynamicIPProviderSetting) normalize() {
	p.ProviderID = strings.TrimSpace(p.ProviderID)
	for index := range p.Gateways {
		p.Gateways[index].normalize(index)
	}
}

func (p dynamicIPProviderSetting) validate(index int) error {
	if !accountproxy.IsSupported(p.ProviderID) {
		return fmt.Errorf("dynamic_ip_providers[%d].provider_id is unsupported", index)
	}
	seen := map[string]struct{}{}
	for gatewayIndex, gateway := range p.Gateways {
		if gateway.Addr == "" {
			return fmt.Errorf("dynamic_ip_providers[%d].gateways[%d].addr is required", index, gatewayIndex)
		}
		if _, exists := seen[gateway.GatewayID]; exists {
			return fmt.Errorf("dynamic_ip_providers[%d].gateways[%d] duplicates gateway %q", index, gatewayIndex, gateway.GatewayID)
		}
		seen[gateway.GatewayID] = struct{}{}
	}
	return nil
}

func (p dynamicIPProviderSetting) toProto() *proxyruntimev1.ProxyDynamicIPProviderSettings {
	out := &proxyruntimev1.ProxyDynamicIPProviderSettings{ProviderId: p.ProviderID}
	for _, gateway := range p.Gateways {
		out.Gateways = append(out.Gateways, gateway.toProto())
	}
	return out
}

func (p dynamicIPProviderSetting) gateways() []accountproxy.Gateway {
	out := make([]accountproxy.Gateway, 0, len(p.Gateways))
	for _, gateway := range p.Gateways {
		out = append(out, gateway.gateway())
	}
	return out
}

func (g *dynamicIPGatewaySetting) normalize(index int) {
	g.GatewayID = strings.TrimSpace(g.GatewayID)
	if g.GatewayID == "" {
		g.GatewayID = fmt.Sprintf("gateway-%d", index+1)
	}
	g.DisplayName = strings.TrimSpace(g.DisplayName)
	g.Addr = strings.TrimSpace(g.Addr)
	g.RegionCodes = cleanRegionCodes(g.RegionCodes)
	g.Protocols = cleanProtocols(g.Protocols)
}

func (g dynamicIPGatewaySetting) toProto() *proxyruntimev1.ProxyDynamicIPGatewaySettings {
	return &proxyruntimev1.ProxyDynamicIPGatewaySettings{
		GatewayId:       g.GatewayID,
		DisplayName:     g.DisplayName,
		Addr:            g.Addr,
		RegionCodes:     g.RegionCodes,
		Protocols:       g.Protocols,
		DefaultProtocol: g.DefaultProtocol,
	}
}

func (g dynamicIPGatewaySetting) gateway() accountproxy.Gateway {
	return accountproxy.Gateway{
		ID:               g.GatewayID,
		DisplayName:      g.DisplayName,
		Addr:             g.Addr,
		DefaultProtocol:  configuredProtocolName(g.DefaultProtocol),
		Protocols:        protocolNames(g.Protocols),
		PreferredRegions: g.RegionCodes,
	}
}

func cleanList(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func cleanRegionCodes(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.ToUpper(strings.TrimSpace(value))
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func cleanProtocols(values []proxyruntimev1.ProxyProtocol) []proxyruntimev1.ProxyProtocol {
	out := make([]proxyruntimev1.ProxyProtocol, 0, len(values))
	seen := map[proxyruntimev1.ProxyProtocol]struct{}{}
	for _, value := range values {
		if value == proxyruntimev1.ProxyProtocol_PROXY_PROTOCOL_UNSPECIFIED {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func protocolNames(values []proxyruntimev1.ProxyProtocol) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if name := configuredProtocolName(value); name != "" {
			out = append(out, name)
		}
	}
	return out
}

func configuredProtocolName(value proxyruntimev1.ProxyProtocol) string {
	switch value {
	case proxyruntimev1.ProxyProtocol_PROXY_PROTOCOL_HTTP:
		return "http"
	case proxyruntimev1.ProxyProtocol_PROXY_PROTOCOL_SOCKS5:
		return "socks5"
	default:
		return ""
	}
}

func defaultProviderWeight(index int) uint32 {
	if index < 0 {
		return 100
	}
	if index > 9 {
		return 10
	}
	return uint32(100 - index*10)
}

func providerDefaultWeight(kind proxyruntimev1.ProxyIPFraudProviderKind, index int) uint32 {
	if plugin, ok := ipfraud.PluginForKind(kind); ok {
		return plugin.DefaultWeight()
	}
	return defaultProviderWeight(index)
}
