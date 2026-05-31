package mihomo

import (
	"errors"
	"strings"
	"time"

	proxyruntimev1 "github.com/byte-v-forge/common-lib/gen/go/byte/v/forge/contracts/proxyruntime/v1"
	"github.com/byte-v-forge/proxy-runtime/internal/sourceplane"
	"google.golang.org/protobuf/types/known/durationpb"
)

func upsertProvider(providers []sourceplane.SubscriptionProvider, req *proxyruntimev1.UpsertProxySubscriptionSourceRequest) (sourceplane.SubscriptionProvider, []sourceplane.SubscriptionProvider, error) {
	id, err := requestID(req.GetSourceId(), "sub", providerIDs(providers))
	if err != nil {
		return sourceplane.SubscriptionProvider{}, nil, err
	}
	current := sourceplane.SubscriptionProvider{ID: id}
	found := false
	for _, item := range providers {
		if item.ID == id {
			current = item
			found = true
			break
		}
	}
	current.DisplayName = firstNonEmpty(req.GetDisplayName(), current.DisplayName, "Subscription")
	if !req.GetEnabled() {
		return current, deleteProvider(providers, id), nil
	}
	if req.GetClearUrl() {
		current.URL = ""
	}
	if strings.TrimSpace(req.GetUrl()) != "" {
		current.URL = strings.TrimSpace(req.GetUrl())
	}
	if current.URL == "" {
		return sourceplane.SubscriptionProvider{}, nil, errors.New("enabled subscription requires url")
	}
	current.Interval = protoDuration(req.GetInterval(), defaultDuration(current.Interval, time.Hour))
	current.Filter = strings.TrimSpace(req.GetFilter())
	current.ExcludeFilter = strings.TrimSpace(req.GetExcludeFilter())
	current.HealthCheckURL = strings.TrimSpace(req.GetHealthCheckUrl())
	current.HealthInterval = protoDuration(req.GetHealthInterval(), defaultDuration(current.HealthInterval, 300*time.Second))
	current.HealthTimeout = protoDuration(req.GetHealthTimeout(), defaultDuration(current.HealthTimeout, 5*time.Second))
	current.HealthLazy = req.GetHealthLazy()
	current.ExpectedStatus = req.GetExpectedStatus()
	current.RegionCodes = cleanRegionCodes(req.GetRegionCodes())
	if current.ExpectedStatus == 0 {
		current.ExpectedStatus = 204
	}
	return current, replaceProvider(providers, current, found), nil
}

func upsertFixedProxy(fixedProxies []sourceplane.FixedProxy, req *proxyruntimev1.UpsertProxyFixedSourceRequest) (sourceplane.FixedProxy, []sourceplane.FixedProxy, error) {
	id, err := requestID(req.GetSourceId(), "fixed", fixedIDs(fixedProxies))
	if err != nil {
		return sourceplane.FixedProxy{}, nil, err
	}
	current := sourceplane.FixedProxy{ID: id}
	found := false
	for _, item := range fixedProxies {
		if item.ID == id {
			current = item
			found = true
			break
		}
	}
	current.DisplayName = firstNonEmpty(req.GetDisplayName(), current.DisplayName, fixedName(req.GetUri()), "Fixed proxy")
	if !req.GetEnabled() {
		return current, deleteFixedProxy(fixedProxies, id), nil
	}
	if req.GetClearUri() {
		current.URI = ""
	}
	if strings.TrimSpace(req.GetUri()) != "" {
		current.URI = strings.TrimSpace(req.GetUri())
	}
	if current.URI == "" {
		return sourceplane.FixedProxy{}, nil, errors.New("enabled fixed proxy requires uri")
	}
	if _, err := renderFixedProxy(current); err != nil {
		return sourceplane.FixedProxy{}, nil, err
	}
	current.RegionCodes = cleanRegionCodes(req.GetRegionCodes())
	return current, replaceFixedProxy(fixedProxies, current, found), nil
}

func replaceProvider(providers []sourceplane.SubscriptionProvider, current sourceplane.SubscriptionProvider, found bool) []sourceplane.SubscriptionProvider {
	out := make([]sourceplane.SubscriptionProvider, 0, len(providers)+1)
	if !found {
		out = append(out, current)
	} else {
		for _, item := range providers {
			if item.ID == current.ID {
				out = append(out, current)
				continue
			}
			out = append(out, item)
		}
	}
	return cleanProviders(out)
}

func replaceFixedProxy(fixedProxies []sourceplane.FixedProxy, current sourceplane.FixedProxy, found bool) []sourceplane.FixedProxy {
	out := make([]sourceplane.FixedProxy, 0, len(fixedProxies)+1)
	if !found {
		out = append(out, current)
	} else {
		for _, item := range fixedProxies {
			if item.ID == current.ID {
				out = append(out, current)
				continue
			}
			out = append(out, item)
		}
	}
	return cleanFixedProxies(out)
}

func deleteProvider(providers []sourceplane.SubscriptionProvider, sourceID string) []sourceplane.SubscriptionProvider {
	id := safeID(sourceID)
	out := make([]sourceplane.SubscriptionProvider, 0, len(providers))
	for _, item := range providers {
		if item.ID != id {
			out = append(out, item)
		}
	}
	return cleanProviders(out)
}

func deleteFixedProxy(fixedProxies []sourceplane.FixedProxy, sourceID string) []sourceplane.FixedProxy {
	id := safeID(sourceID)
	out := make([]sourceplane.FixedProxy, 0, len(fixedProxies))
	for _, item := range fixedProxies {
		if item.ID != id {
			out = append(out, item)
		}
	}
	return cleanFixedProxies(out)
}

func protoDuration(value *durationpb.Duration, fallback time.Duration) time.Duration {
	if value == nil || value.AsDuration() <= 0 {
		return fallback
	}
	return value.AsDuration()
}

func defaultDuration(value time.Duration, fallback time.Duration) time.Duration {
	if value <= 0 {
		return fallback
	}
	return value
}
