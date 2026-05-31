package mihomo

import (
	"strings"
	"time"

	proxyruntimev1 "github.com/byte-v-forge/common-lib/gen/go/byte/v/forge/contracts/proxyruntime/v1"
	"github.com/byte-v-forge/proxy-runtime/internal/sourceplane"
	"google.golang.org/protobuf/types/known/durationpb"
)

func subscriptionSourceDescriptor(item sourceplane.SubscriptionProvider) *proxyruntimev1.ProxySourceDescriptor {
	return &proxyruntimev1.ProxySourceDescriptor{SourceId: item.ID, ProviderId: ProviderID, DisplayName: firstNonEmpty(item.DisplayName, item.ID), Kind: proxyruntimev1.ProxySourceKind_PROXY_SOURCE_KIND_SUBSCRIPTION, Enabled: true, Capabilities: []proxyruntimev1.ProxyCapability{proxyruntimev1.ProxyCapability_PROXY_CAPABILITY_SUBSCRIPTION_PROVIDER, proxyruntimev1.ProxyCapability_PROXY_CAPABILITY_POOL_REFRESH}, Model: &proxyruntimev1.ProxySourceDescriptor_Subscription{Subscription: &proxyruntimev1.ProxySubscriptionSourceDescriptor{UrlRedacted: redactedURL(item.URL), Interval: durationpb.New(defaultDuration(item.Interval, time.Hour)), Filter: item.Filter, ExcludeFilter: item.ExcludeFilter, HealthCheckUrl: item.HealthCheckURL, HealthInterval: durationpb.New(defaultDuration(item.HealthInterval, 300*time.Second)), HealthTimeout: durationpb.New(defaultDuration(item.HealthTimeout, 5*time.Second)), HealthLazy: item.HealthLazy, ExpectedStatus: defaultExpectedStatus(item.ExpectedStatus), RegionCodes: cleanRegionCodes(item.RegionCodes)}}}
}

func disabledSubscriptionDescriptor(id string, displayName string) *proxyruntimev1.ProxySourceDescriptor {
	return &proxyruntimev1.ProxySourceDescriptor{SourceId: id, ProviderId: ProviderID, DisplayName: firstNonEmpty(displayName, id), Kind: proxyruntimev1.ProxySourceKind_PROXY_SOURCE_KIND_SUBSCRIPTION, Enabled: false, Capabilities: []proxyruntimev1.ProxyCapability{proxyruntimev1.ProxyCapability_PROXY_CAPABILITY_SUBSCRIPTION_PROVIDER, proxyruntimev1.ProxyCapability_PROXY_CAPABILITY_POOL_REFRESH}, Model: &proxyruntimev1.ProxySourceDescriptor_Subscription{Subscription: &proxyruntimev1.ProxySubscriptionSourceDescriptor{}}}
}

func fixedSourceDescriptor(item sourceplane.FixedProxy) *proxyruntimev1.ProxySourceDescriptor {
	return &proxyruntimev1.ProxySourceDescriptor{SourceId: item.ID, ProviderId: ProviderID, DisplayName: firstNonEmpty(item.DisplayName, item.ID), Kind: proxyruntimev1.ProxySourceKind_PROXY_SOURCE_KIND_FIXED_PROXY, Enabled: true, Capabilities: []proxyruntimev1.ProxyCapability{proxyruntimev1.ProxyCapability_PROXY_CAPABILITY_CHAINING}, Model: &proxyruntimev1.ProxySourceDescriptor_FixedProxy{FixedProxy: &proxyruntimev1.ProxyFixedSourceDescriptor{EndpointCount: 1, RegionCodes: cleanRegionCodes(item.RegionCodes)}}}
}

func disabledFixedDescriptor(id string, displayName string) *proxyruntimev1.ProxySourceDescriptor {
	return &proxyruntimev1.ProxySourceDescriptor{SourceId: id, ProviderId: ProviderID, DisplayName: firstNonEmpty(displayName, id), Kind: proxyruntimev1.ProxySourceKind_PROXY_SOURCE_KIND_FIXED_PROXY, Enabled: false, Capabilities: []proxyruntimev1.ProxyCapability{proxyruntimev1.ProxyCapability_PROXY_CAPABILITY_CHAINING}, Model: &proxyruntimev1.ProxySourceDescriptor_FixedProxy{FixedProxy: &proxyruntimev1.ProxyFixedSourceDescriptor{}}}
}

func redactedURL(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	return "configured"
}

func defaultExpectedStatus(value uint32) uint32 {
	if value == 0 {
		return 204
	}
	return value
}
