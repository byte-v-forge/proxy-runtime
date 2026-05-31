package mihomo

import (
	"errors"
	"regexp"
	"strings"
	"time"
)

func renderConfig(opts renderOptions) (mihomoConfig, error) {
	host, port, err := splitEndpoint(opts.Endpoint.Addr)
	if err != nil {
		return mihomoConfig{}, err
	}
	providerMap := make(map[string]mihomoProvider, len(opts.Providers))
	providerIDs := make([]string, 0, len(opts.Providers))
	for _, item := range opts.Providers {
		id := safeID(item.ID)
		providerIDs = append(providerIDs, id)
		providerMap[id] = mihomoProvider{
			Type:     "http",
			URL:      item.URL,
			Path:     providerConfigPath(opts.ConfigDir, item, id),
			Interval: seconds(item.Interval, 3600),
			Filter:   item.Filter,
			Exclude:  item.ExcludeFilter,
			Header:   item.Headers,
			HealthCheck: &mihomoHealthCheck{
				Enable:         true,
				URL:            firstNonEmpty(item.HealthCheckURL, opts.HealthCheckURL, "https://www.gstatic.com/generate_204"),
				Interval:       seconds(item.HealthInterval, secondsDuration(opts.HealthCheckInterval, 300)),
				Timeout:        milliseconds(item.HealthTimeout, millisecondsDuration(opts.HealthCheckTimeout, 5000)),
				Lazy:           item.HealthLazy,
				ExpectedStatus: defaultExpectedStatus(item.ExpectedStatus),
			},
		}
	}
	fixedConfigs := make([]map[string]any, 0, len(opts.FixedProxies))
	fixedIDs := make([]string, 0, len(opts.FixedProxies))
	for _, item := range opts.FixedProxies {
		proxyConfig, err := renderFixedProxy(item)
		if err != nil {
			return mihomoConfig{}, err
		}
		fixedConfigs = append(fixedConfigs, proxyConfig)
		fixedIDs = append(fixedIDs, safeID(item.ID))
	}
	nodeGroups := renderNodeProxyGroups(opts.NodeListeners)
	listeners, err := renderNodeListeners(opts.NodeListeners)
	if err != nil {
		return mihomoConfig{}, err
	}
	return mihomoConfig{
		MixedPort:          port,
		BindAddress:        host,
		AllowLAN:           false,
		Mode:               "rule",
		LogLevel:           "warning",
		ExternalController: strings.TrimSpace(opts.APIAddr),
		Proxies:            fixedConfigs,
		ProxyProviders:     providerMap,
		Listeners:          listeners,
		ProxyGroups: append([]mihomoGroup{{
			Name:           groupName,
			Type:           groupStrategy(opts.GroupStrategy),
			Proxies:        fixedIDs,
			Use:            providerIDs,
			URL:            firstNonEmpty(opts.HealthCheckURL, "https://www.gstatic.com/generate_204"),
			Interval:       secondsDuration(opts.HealthCheckInterval, 300),
			Timeout:        millisecondsDuration(opts.HealthCheckTimeout, 5000),
			Lazy:           true,
			ExpectedStatus: 204,
		}}, nodeGroups...),
		Rules: []string{"MATCH," + groupName},
	}, nil
}

func renderNodeProxyGroups(bindings []nodeListener) []mihomoGroup {
	out := make([]mihomoGroup, 0, len(bindings))
	for _, binding := range bindings {
		group := mihomoGroup{
			Name:           lineGroupName(binding.SourceID, binding.NodeID),
			Type:           "select",
			URL:            "https://www.gstatic.com/generate_204",
			Interval:       300,
			Timeout:        5000,
			Lazy:           true,
			ExpectedStatus: 204,
		}
		if binding.ProviderBacked {
			group.Use = []string{binding.SourceID}
			group.Filter = exactProxyNameFilter(binding.ProxyName)
		} else {
			group.Proxies = []string{binding.ProxyName}
		}
		out = append(out, group)
	}
	return out
}

func renderNodeListeners(bindings []nodeListener) ([]mihomoListener, error) {
	out := make([]mihomoListener, 0, len(bindings))
	for _, binding := range bindings {
		host, port, err := splitEndpoint(binding.Endpoint.Addr)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(binding.ProxyName) == "" {
			return nil, errors.New("mihomo node listener proxy is required")
		}
		out = append(out, mihomoListener{
			Name:   lineListenerName(binding.SourceID, binding.NodeID),
			Type:   "mixed",
			Listen: host,
			Port:   port,
			Proxy:  lineGroupName(binding.SourceID, binding.NodeID),
			UDP:    true,
		})
	}
	return out, nil
}

func exactProxyNameFilter(name string) string {
	return "^" + regexp.QuoteMeta(strings.TrimSpace(name)) + "$"
}

func groupStrategy(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "select", "url-test", "load-balance":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "fallback"
	}
}

func seconds(value time.Duration, fallback int) int {
	if value <= 0 {
		return fallback
	}
	return int(value / time.Second)
}

func milliseconds(value time.Duration, fallback int) int {
	if value <= 0 {
		return fallback
	}
	return int(value / time.Millisecond)
}

func secondsDuration(value time.Duration, fallback int) int { return seconds(value, fallback) }

func millisecondsDuration(value time.Duration, fallback int) int {
	return milliseconds(value, fallback)
}
