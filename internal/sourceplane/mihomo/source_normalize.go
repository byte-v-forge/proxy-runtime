package mihomo

import (
	"strings"

	"github.com/byte-v-forge/proxy-runtime/internal/sourceplane"
)

func cleanSourceFile(file sourceFile) sourceFile {
	return sourceFile{Subscriptions: cleanProviders(file.Subscriptions), FixedProxies: cleanFixedProxies(file.FixedProxies)}
}

func cleanProviders(in []sourceplane.SubscriptionProvider) []sourceplane.SubscriptionProvider {
	seen := map[string]struct{}{}
	out := make([]sourceplane.SubscriptionProvider, 0, len(in))
	for _, item := range in {
		item.ID = safeID(item.ID)
		item.DisplayName = strings.TrimSpace(item.DisplayName)
		item.URL = strings.TrimSpace(item.URL)
		item.Path = strings.TrimSpace(item.Path)
		item.Filter = strings.TrimSpace(item.Filter)
		item.ExcludeFilter = strings.TrimSpace(item.ExcludeFilter)
		item.HealthCheckURL = strings.TrimSpace(item.HealthCheckURL)
		item.RegionCodes = cleanRegionCodes(item.RegionCodes)
		if item.ID == "" || item.URL == "" {
			continue
		}
		if _, exists := seen[item.ID]; exists {
			continue
		}
		seen[item.ID] = struct{}{}
		out = append(out, item)
	}
	return out
}

func cleanFixedProxies(in []sourceplane.FixedProxy) []sourceplane.FixedProxy {
	seen := map[string]struct{}{}
	out := make([]sourceplane.FixedProxy, 0, len(in))
	for _, item := range in {
		item.ID = safeID(item.ID)
		item.DisplayName = strings.TrimSpace(item.DisplayName)
		item.URI = strings.TrimSpace(item.URI)
		item.RegionCodes = cleanRegionCodes(item.RegionCodes)
		if item.ID == "" || item.URI == "" {
			continue
		}
		if _, exists := seen[item.ID]; exists {
			continue
		}
		seen[item.ID] = struct{}{}
		out = append(out, item)
	}
	return out
}

func cleanRegionCodes(in []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, value := range in {
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

func enabledProviders(in []sourceplane.SubscriptionProvider) []sourceplane.SubscriptionProvider {
	out := make([]sourceplane.SubscriptionProvider, 0, len(in))
	for _, item := range in {
		if strings.TrimSpace(item.ID) == "" || strings.TrimSpace(item.URL) == "" {
			continue
		}
		out = append(out, item)
	}
	return out
}

func enabledFixedProxies(in []sourceplane.FixedProxy) []sourceplane.FixedProxy {
	out := make([]sourceplane.FixedProxy, 0, len(in))
	for _, item := range in {
		if strings.TrimSpace(item.ID) == "" || strings.TrimSpace(item.URI) == "" {
			continue
		}
		out = append(out, item)
	}
	return out
}
