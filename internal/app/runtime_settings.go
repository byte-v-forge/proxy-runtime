package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	proxyruntimev1 "github.com/byte-v-forge/common-lib/gen/go/byte/v/forge/contracts/proxyruntime/v1"
	"github.com/byte-v-forge/common-lib/protojsonx"
	"github.com/byte-v-forge/proxy-runtime/internal/ipfraud"
	"github.com/byte-v-forge/proxy-runtime/internal/provider/accountproxy"
)

type runtimeSettingsFile = proxyruntimev1.ProxyRuntimePersistentSettings

type runtimeSettingsStore struct {
	store  *PostgresStore
	logger *slog.Logger
	mu     sync.Mutex
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
	return runtimeSettingsView(settings), nil
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
	return runtimeSettingsView(settings), nil
}

func (s *runtimeSettingsStore) load() (*runtimeSettingsFile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadLocked()
}

func (s *runtimeSettingsStore) loadLocked() (*runtimeSettingsFile, error) {
	if s.store == nil {
		return normalizeRuntimeSettings(nil), nil
	}
	return s.store.LoadRuntimeSettings(context.Background())
}

func (s *runtimeSettingsStore) save(settings *runtimeSettingsFile) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveLocked(settings)
}

func (s *runtimeSettingsStore) saveLocked(settings *runtimeSettingsFile) error {
	if s.store == nil {
		return nil
	}
	return s.store.SaveRuntimeSettings(context.Background(), normalizeRuntimeSettings(settings))
}

func settingsFromRequest(req *proxyruntimev1.UpdateProxyRuntimeSettingsRequest, current *runtimeSettingsFile) (*runtimeSettingsFile, error) {
	current = normalizeRuntimeSettings(current)
	settings := &proxyruntimev1.ProxyRuntimePersistentSettings{
		EdgeCanary:         edgeCanaryFromRequest(req.GetEdgeCanary(), current.GetEdgeCanary()),
		IpFraudProviders:   make([]*proxyruntimev1.ProxyIPFraudProviderSettings, 0, len(req.GetIpFraudProviders())),
		DynamicIpProviders: make([]*proxyruntimev1.ProxyDynamicIPProviderSettings, 0, len(req.GetDynamicIpProviders())),
	}
	if edgeCanaryEnabled(settings.GetEdgeCanary()) && strings.TrimSpace(settings.GetEdgeCanary().GetUrl()) == "" {
		return nil, errors.New("edge canary url is required when enabled")
	}
	currentProviders := providerSecrets(current)
	seenProviders := map[string]struct{}{}
	for index, provider := range req.GetIpFraudProviders() {
		item := ipFraudProviderFromRequest(provider, currentProviders, index)
		if err := validateIPFraudProvider(item, index); err != nil {
			return nil, err
		}
		key := providerSecretKey(item.GetKind(), item.GetProviderId())
		if _, exists := seenProviders[key]; exists {
			return nil, fmt.Errorf("ip_fraud_providers[%d] duplicates provider %q", index, item.GetProviderId())
		}
		seenProviders[key] = struct{}{}
		settings.IpFraudProviders = append(settings.IpFraudProviders, item)
	}
	seenDynamicProviders := map[string]struct{}{}
	for index, provider := range req.GetDynamicIpProviders() {
		item := dynamicIPProviderFromProto(provider)
		if err := validateDynamicIPProvider(item, index); err != nil {
			return nil, err
		}
		if _, exists := seenDynamicProviders[item.GetProviderId()]; exists {
			return nil, fmt.Errorf("dynamic_ip_providers[%d] duplicates provider %q", index, item.GetProviderId())
		}
		seenDynamicProviders[item.GetProviderId()] = struct{}{}
		settings.DynamicIpProviders = append(settings.DynamicIpProviders, item)
	}
	return normalizeRuntimeSettings(settings), nil
}

func edgeCanaryFromRequest(req *proxyruntimev1.ProxyEdgeCanarySettings, current *proxyruntimev1.ProxyEdgeCanarySettings) *proxyruntimev1.ProxyEdgeCanarySettings {
	if req == nil {
		return cloneEdgeCanary(current)
	}
	settings := &proxyruntimev1.ProxyEdgeCanarySettings{
		Enabled: req.GetEnabled(),
		Url:     firstNonEmpty(req.GetUrl(), current.GetUrl()),
		Token:   strings.TrimSpace(req.GetToken()),
	}
	switch {
	case settings.Token != "":
	case req.GetClearToken():
		settings.Token = ""
	default:
		settings.Token = strings.TrimSpace(current.GetToken())
	}
	return settings
}

func normalizeRuntimeSettings(settings *runtimeSettingsFile) *runtimeSettingsFile {
	if settings == nil {
		settings = &proxyruntimev1.ProxyRuntimePersistentSettings{}
	}
	if settings.EdgeCanary != nil {
		settings.EdgeCanary.Url = strings.TrimSpace(settings.EdgeCanary.GetUrl())
		settings.EdgeCanary.Token = strings.TrimSpace(settings.EdgeCanary.GetToken())
	}
	for index := range settings.IpFraudProviders {
		normalizeIPFraudProvider(settings.IpFraudProviders[index], index)
	}
	settings.IpFraudProviders = supportedIPFraudProviders(settings.IpFraudProviders)
	for index := range settings.DynamicIpProviders {
		normalizeDynamicIPProvider(settings.DynamicIpProviders[index])
	}
	return settings
}

func edgeCanaryEnabled(settings *proxyruntimev1.ProxyEdgeCanarySettings) bool {
	return settings != nil && settings.GetEnabled()
}

func runtimeSettingsView(settings *runtimeSettingsFile) *proxyruntimev1.ProxyRuntimeSettings {
	settings = normalizeRuntimeSettings(settings)
	edge := settings.GetEdgeCanary()
	out := &proxyruntimev1.ProxyRuntimeSettings{
		EdgeCanary: &proxyruntimev1.ProxyEdgeCanarySettingsView{
			Url:             edge.GetUrl(),
			TokenConfigured: edge.GetToken() != "",
			Enabled:         edgeCanaryEnabled(edge),
		},
	}
	for _, provider := range settings.GetIpFraudProviders() {
		out.IpFraudProviders = append(out.IpFraudProviders, &proxyruntimev1.ProxyIPFraudProviderSettingsView{
			ProviderId:       provider.GetProviderId(),
			Weight:           provider.GetWeight(),
			Kind:             provider.GetKind(),
			Anonymous:        provider.GetAnonymous(),
			ApiKeyConfigured: len(provider.GetApiKeys()) > 0,
			ApiKeyCount:      uint32(len(provider.GetApiKeys())),
		})
	}
	for _, provider := range settings.GetDynamicIpProviders() {
		out.DynamicIpProviders = append(out.DynamicIpProviders, cloneDynamicIPProvider(provider))
	}
	return out
}

func runtimeSettingsSignature(settings *runtimeSettingsFile) string {
	data, _ := protojsonx.Marshal(normalizeRuntimeSettings(settings))
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func providerSecrets(settings *runtimeSettingsFile) map[string][]string {
	secrets := map[string][]string{}
	for _, item := range normalizeRuntimeSettings(settings).GetIpFraudProviders() {
		secrets[providerSecretKey(item.GetKind(), item.GetProviderId())] = append([]string(nil), item.GetApiKeys()...)
	}
	return secrets
}

func providerSecretKey(kind proxyruntimev1.ProxyIPFraudProviderKind, id string) string {
	return fmt.Sprintf("%d:%s", kind, strings.TrimSpace(id))
}

func ipFraudProviders(settings *runtimeSettingsFile) []ipfraud.ProviderConfig {
	items := normalizeRuntimeSettings(settings).GetIpFraudProviders()
	providers := make([]ipfraud.ProviderConfig, 0, len(items))
	for _, item := range items {
		providers = append(providers, ipfraud.ProviderConfig{
			ID:     item.GetProviderId(),
			Kind:   item.GetKind(),
			Weight: int(item.GetWeight()),
			Auth:   ipFraudAuth(item),
		})
	}
	return providers
}

func dynamicIPGatewayMap(settings *runtimeSettingsFile) map[string][]accountproxy.Gateway {
	out := map[string][]accountproxy.Gateway{}
	for _, provider := range normalizeRuntimeSettings(settings).GetDynamicIpProviders() {
		out[provider.GetProviderId()] = accountProxyGateways(provider.GetGateways())
	}
	return out
}

func dynamicIPGateways(settings *runtimeSettingsFile, providerID string) []accountproxy.Gateway {
	return dynamicIPGatewayMap(settings)[strings.TrimSpace(providerID)]
}

func ipFraudProviderFromRequest(in *proxyruntimev1.ProxyIPFraudProviderSettings, current map[string][]string, index int) *proxyruntimev1.ProxyIPFraudProviderSettings {
	if in == nil {
		return &proxyruntimev1.ProxyIPFraudProviderSettings{}
	}
	id := strings.TrimSpace(in.GetProviderId())
	if id == "" {
		id = ipfraud.DefaultProviderID(in.GetKind())
	}
	apiKeys := cleanList(in.GetApiKeys())
	if len(apiKeys) == 0 && !in.GetClearApiKeys() && !in.GetAnonymous() {
		apiKeys = current[providerSecretKey(in.GetKind(), id)]
	}
	weight := in.GetWeight()
	if weight == 0 {
		weight = providerDefaultWeight(in.GetKind(), index)
	}
	return &proxyruntimev1.ProxyIPFraudProviderSettings{ProviderId: id, Weight: weight, Kind: in.GetKind(), Anonymous: in.GetAnonymous(), ApiKeys: apiKeys}
}

func normalizeIPFraudProvider(provider *proxyruntimev1.ProxyIPFraudProviderSettings, index int) {
	if provider == nil {
		return
	}
	provider.ProviderId = strings.TrimSpace(provider.GetProviderId())
	provider.ApiKeys = cleanList(provider.GetApiKeys())
	if provider.ProviderId == "" {
		provider.ProviderId = ipfraud.DefaultProviderID(provider.GetKind())
	}
	if provider.Weight == 0 {
		provider.Weight = providerDefaultWeight(provider.GetKind(), index)
	}
}

func validateIPFraudProvider(provider *proxyruntimev1.ProxyIPFraudProviderSettings, index int) error {
	plugin, ok := ipfraud.PluginForKind(provider.GetKind())
	if !ok {
		return fmt.Errorf("ip_fraud_providers[%d].kind is required", index)
	}
	if provider.GetAnonymous() && len(provider.GetApiKeys()) > 0 {
		return fmt.Errorf("ip_fraud_providers[%d] must use anonymous or api_keys, not both", index)
	}
	if provider.GetAnonymous() && !plugin.SupportsAnonymous() {
		return fmt.Errorf("ip_fraud_providers[%d] does not support anonymous mode", index)
	}
	if !provider.GetAnonymous() && !plugin.SupportsAPIKey() {
		return fmt.Errorf("ip_fraud_providers[%d] does not support api key mode", index)
	}
	if !provider.GetAnonymous() && len(provider.GetApiKeys()) == 0 {
		return fmt.Errorf("ip_fraud_providers[%d].api_keys is required when anonymous is false", index)
	}
	return nil
}

func supportedIPFraudProviders(providers []*proxyruntimev1.ProxyIPFraudProviderSettings) []*proxyruntimev1.ProxyIPFraudProviderSettings {
	out := make([]*proxyruntimev1.ProxyIPFraudProviderSettings, 0, len(providers))
	for _, provider := range providers {
		if ipfraud.IsProviderKindSupported(provider.GetKind()) {
			out = append(out, provider)
		}
	}
	return out
}

func ipFraudAuth(provider *proxyruntimev1.ProxyIPFraudProviderSettings) ipfraud.AuthConfig {
	if provider.GetAnonymous() {
		return ipfraud.AuthConfig{Anonymous: &ipfraud.AnonymousAuthConfig{}}
	}
	plugin, ok := ipfraud.PluginForKind(provider.GetKind())
	if !ok {
		return ipfraud.AuthConfig{}
	}
	return plugin.Auth(provider.GetApiKeys(), false)
}

func dynamicIPProviderFromProto(in *proxyruntimev1.ProxyDynamicIPProviderSettings) *proxyruntimev1.ProxyDynamicIPProviderSettings {
	if in == nil {
		return &proxyruntimev1.ProxyDynamicIPProviderSettings{}
	}
	out := &proxyruntimev1.ProxyDynamicIPProviderSettings{ProviderId: strings.TrimSpace(in.GetProviderId()), Gateways: make([]*proxyruntimev1.ProxyDynamicIPGatewaySettings, 0, len(in.GetGateways()))}
	for _, gateway := range in.GetGateways() {
		out.Gateways = append(out.Gateways, dynamicIPGatewayFromProto(gateway))
	}
	normalizeDynamicIPProvider(out)
	return out
}

func dynamicIPGatewayFromProto(in *proxyruntimev1.ProxyDynamicIPGatewaySettings) *proxyruntimev1.ProxyDynamicIPGatewaySettings {
	if in == nil {
		return &proxyruntimev1.ProxyDynamicIPGatewaySettings{}
	}
	out := &proxyruntimev1.ProxyDynamicIPGatewaySettings{
		GatewayId:       strings.TrimSpace(in.GetGatewayId()),
		DisplayName:     strings.TrimSpace(in.GetDisplayName()),
		Addr:            strings.TrimSpace(in.GetAddr()),
		RegionCodes:     cleanRegionCodes(in.GetRegionCodes()),
		Protocols:       cleanProtocols(in.GetProtocols()),
		DefaultProtocol: in.GetDefaultProtocol(),
	}
	return out
}

func normalizeDynamicIPProvider(provider *proxyruntimev1.ProxyDynamicIPProviderSettings) {
	if provider == nil {
		return
	}
	provider.ProviderId = strings.TrimSpace(provider.GetProviderId())
	for index := range provider.Gateways {
		normalizeDynamicIPGateway(provider.Gateways[index], index)
	}
}

func validateDynamicIPProvider(provider *proxyruntimev1.ProxyDynamicIPProviderSettings, index int) error {
	if !accountproxy.IsSupported(provider.GetProviderId()) {
		return fmt.Errorf("dynamic_ip_providers[%d].provider_id is unsupported", index)
	}
	seen := map[string]struct{}{}
	for gatewayIndex, gateway := range provider.GetGateways() {
		if strings.TrimSpace(gateway.GetAddr()) == "" {
			return fmt.Errorf("dynamic_ip_providers[%d].gateways[%d].addr is required", index, gatewayIndex)
		}
		if _, exists := seen[gateway.GetGatewayId()]; exists {
			return fmt.Errorf("dynamic_ip_providers[%d].gateways[%d] duplicates gateway %q", index, gatewayIndex, gateway.GetGatewayId())
		}
		seen[gateway.GetGatewayId()] = struct{}{}
	}
	return nil
}

func normalizeDynamicIPGateway(gateway *proxyruntimev1.ProxyDynamicIPGatewaySettings, index int) {
	if gateway == nil {
		return
	}
	gateway.GatewayId = strings.TrimSpace(gateway.GetGatewayId())
	if gateway.GatewayId == "" {
		gateway.GatewayId = fmt.Sprintf("gateway-%d", index+1)
	}
	gateway.DisplayName = strings.TrimSpace(gateway.GetDisplayName())
	gateway.Addr = strings.TrimSpace(gateway.GetAddr())
	gateway.RegionCodes = cleanRegionCodes(gateway.GetRegionCodes())
	gateway.Protocols = cleanProtocols(gateway.GetProtocols())
}

func accountProxyGateways(gateways []*proxyruntimev1.ProxyDynamicIPGatewaySettings) []accountproxy.Gateway {
	out := make([]accountproxy.Gateway, 0, len(gateways))
	for _, gateway := range gateways {
		out = append(out, accountproxy.Gateway{
			ID:               gateway.GetGatewayId(),
			DisplayName:      gateway.GetDisplayName(),
			Addr:             gateway.GetAddr(),
			DefaultProtocol:  configuredProtocolName(gateway.GetDefaultProtocol()),
			Protocols:        protocolNames(gateway.GetProtocols()),
			PreferredRegions: gateway.GetRegionCodes(),
		})
	}
	return out
}

func cloneEdgeCanary(in *proxyruntimev1.ProxyEdgeCanarySettings) *proxyruntimev1.ProxyEdgeCanarySettings {
	if in == nil {
		return nil
	}
	return &proxyruntimev1.ProxyEdgeCanarySettings{Url: in.GetUrl(), Token: in.GetToken(), ClearToken: in.GetClearToken(), Enabled: in.GetEnabled()}
}

func cloneDynamicIPProvider(in *proxyruntimev1.ProxyDynamicIPProviderSettings) *proxyruntimev1.ProxyDynamicIPProviderSettings {
	return dynamicIPProviderFromProto(in)
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
