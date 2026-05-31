package accountproxy

import (
	"fmt"
	"net/http"
	"sort"
	"strings"

	proxyruntimev1 "github.com/byte-v-forge/common-lib/gen/go/byte/v/forge/contracts/proxyruntime/v1"
	"github.com/byte-v-forge/proxy-runtime/internal/provider"
)

var registry = map[string]Plugin{}

func Register(plugin Plugin) {
	if plugin == nil || strings.TrimSpace(plugin.ID()) == "" {
		panic("proxy account plugin id is required")
	}
	registry[plugin.ID()] = plugin
}

func Get(providerID string) (Plugin, bool) {
	plugin, ok := registry[strings.TrimSpace(providerID)]
	return plugin, ok
}

func IsSupported(providerID string) bool {
	_, ok := Get(providerID)
	return ok
}

func SupportsRuntimeGeoTargeting(providerID string) bool {
	plugin, ok := Get(providerID)
	return ok && plugin.SupportsRuntimeGeoTargeting()
}

func Descriptors(gateways map[string][]Gateway) []*proxyruntimev1.ProxyProviderDescriptor {
	ids := providerIDs()
	out := make([]*proxyruntimev1.ProxyProviderDescriptor, 0, len(ids))
	for _, id := range ids {
		out = append(out, registry[id].Descriptor(gateways[id]))
	}
	return out
}

func New(cfg Config, client *http.Client) (provider.Provider, error) {
	plugin, ok := Get(cfg.ProviderID)
	if !ok {
		return nil, fmt.Errorf("unsupported provider_id %q", cfg.ProviderID)
	}
	return plugin.NewProvider(cfg, client)
}

func providerIDs() []string {
	ids := make([]string, 0, len(registry))
	for id := range registry {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
