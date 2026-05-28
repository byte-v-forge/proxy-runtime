package ipfraud

import (
	"sort"

	proxyruntimev1 "github.com/byte-v-forge/common-lib/gen/go/byte/v/forge/contracts/proxyruntime/v1"
)

var plugins = map[proxyruntimev1.ProxyIPFraudProviderKind]Plugin{}

func Register(plugin Plugin) {
	if plugin == nil || plugin.Kind() == proxyruntimev1.ProxyIPFraudProviderKind_PROXY_IP_FRAUD_PROVIDER_KIND_UNSPECIFIED {
		panic("IP fraud provider plugin kind is required")
	}
	plugins[plugin.Kind()] = plugin
}

func PluginForKind(kind proxyruntimev1.ProxyIPFraudProviderKind) (Plugin, bool) {
	plugin, ok := plugins[kind]
	return plugin, ok
}

func IsProviderKindSupported(kind proxyruntimev1.ProxyIPFraudProviderKind) bool {
	_, ok := PluginForKind(kind)
	return ok
}

func DefaultProviderID(kind proxyruntimev1.ProxyIPFraudProviderKind) string {
	plugin, ok := PluginForKind(kind)
	if !ok {
		return ""
	}
	return plugin.ProviderID()
}

func ProviderDescriptors() []*proxyruntimev1.ProxyIPFraudProviderDescriptor {
	kinds := make([]proxyruntimev1.ProxyIPFraudProviderKind, 0, len(plugins))
	for kind := range plugins {
		kinds = append(kinds, kind)
	}
	sort.Slice(kinds, func(i, j int) bool { return plugins[kinds[i]].DefaultWeight() > plugins[kinds[j]].DefaultWeight() })
	out := make([]*proxyruntimev1.ProxyIPFraudProviderDescriptor, 0, len(kinds))
	for _, kind := range kinds {
		plugin := plugins[kind]
		out = append(out, &proxyruntimev1.ProxyIPFraudProviderDescriptor{ProviderId: plugin.ProviderID(), DisplayName: plugin.DisplayName(), DefaultWeight: plugin.DefaultWeight(), Kind: kind, SupportsAnonymous: plugin.SupportsAnonymous(), SupportsApiKey: plugin.SupportsAPIKey()})
	}
	return out
}
