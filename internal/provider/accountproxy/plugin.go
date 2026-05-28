package accountproxy

import (
	"net/http"

	proxyruntimev1 "github.com/byte-v-forge/common-lib/gen/go/byte/v/forge/contracts/proxyruntime/v1"
	"github.com/byte-v-forge/proxy-runtime/internal/provider"
)

type definitionPlugin struct{ definition Definition }

func RegisterDefinition(definition Definition) {
	Register(definitionPlugin{definition: definition})
}

func (p definitionPlugin) ID() string { return p.definition.ProviderID }

func (p definitionPlugin) DisplayName() string { return p.definition.DisplayName }

func (p definitionPlugin) Descriptor(gateways []Gateway) *proxyruntimev1.ProxyProviderDescriptor {
	return descriptor(p.definition, gateways)
}

func (p definitionPlugin) DynamicSource(accountID string, displayName string, gateways []Gateway) *proxyruntimev1.ProxySourceDescriptor {
	return dynamicSource(p.definition, accountID, displayName, gateways)
}

func (p definitionPlugin) NewProvider(cfg Config, client *http.Client) (provider.Provider, error) {
	cfg.ProviderID = p.definition.ProviderID
	if err := p.Validate(cfg); err != nil {
		return nil, err
	}
	if len(cfg.Gateways) == 0 {
		return nil, provider.ErrUnsupportedCapability
	}
	definition := p.definition
	definition.Gateways = cfg.Gateways
	return NewCredentialProvider(cfg, definition, client), nil
}

func (p definitionPlugin) Validate(cfg Config) error {
	cfg.ProviderID = p.definition.ProviderID
	return validateConfig(cfg, p.definition)
}
