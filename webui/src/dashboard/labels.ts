import {
  EgressHopRole,
  ProxyRouteRuntimeKind,
  ProxySourceRuntimeKind,
  ProxyDynamicLeaseStatus,
  ProxyCapability,
  ProxyProviderAccountStatus,
  ProxyProtocol,
  ProxyRotationMode,
  ProxySelectorStrategy,
  ProxySessionMode,
  ProxySourceKind,
  ProxySourceNodeStatus,
  ProxyUpstreamKind
} from '@byte-v-forge/common-ui/proto/byte/v/forge/contracts/proxyruntime/v1/proxy_runtime';

const labels: Record<string, string> = {
  [ProxyProtocol.PROXY_PROTOCOL_HTTP]: 'HTTP',
  [ProxyProtocol.PROXY_PROTOCOL_SOCKS5]: 'SOCKS5',
  [ProxyRouteRuntimeKind.PROXY_ROUTE_RUNTIME_KIND_GOST]: 'GOST',
  [ProxySourceRuntimeKind.PROXY_SOURCE_RUNTIME_KIND_NONE]: 'None',
  [ProxySourceRuntimeKind.PROXY_SOURCE_RUNTIME_KIND_MIHOMO]: 'Mihomo',
  [ProxySourceKind.PROXY_SOURCE_KIND_FIXED_PROXY]: '固定代理',
  [ProxySourceKind.PROXY_SOURCE_KIND_SUBSCRIPTION]: '订阅',
  [ProxySourceKind.PROXY_SOURCE_KIND_API_POOL]: 'API 池',
  [ProxySourceKind.PROXY_SOURCE_KIND_DYNAMIC_IP]: '动态 IP',
  [ProxyProviderAccountStatus.PROXY_PROVIDER_ACCOUNT_STATUS_ENABLED]: '启用',
  [ProxyProviderAccountStatus.PROXY_PROVIDER_ACCOUNT_STATUS_DISABLED]: '停用',
  [ProxyDynamicLeaseStatus.PROXY_DYNAMIC_LEASE_STATUS_ACTIVE]: 'Active',
  [ProxyDynamicLeaseStatus.PROXY_DYNAMIC_LEASE_STATUS_EXPIRED]: 'Expired',
  [ProxyDynamicLeaseStatus.PROXY_DYNAMIC_LEASE_STATUS_FAILED]: 'Failed',
  [ProxyDynamicLeaseStatus.PROXY_DYNAMIC_LEASE_STATUS_RELEASED]: 'Released',
  [ProxySourceNodeStatus.PROXY_SOURCE_NODE_STATUS_UNKNOWN]: 'Unknown',
  [ProxySourceNodeStatus.PROXY_SOURCE_NODE_STATUS_AVAILABLE]: 'Available',
  [ProxySourceNodeStatus.PROXY_SOURCE_NODE_STATUS_UNAVAILABLE]: 'Unavailable',
  [ProxyUpstreamKind.PROXY_UPSTREAM_KIND_SIMPLE_PROXY]: '简单代理',
  [ProxyUpstreamKind.PROXY_UPSTREAM_KIND_DYNAMIC_IP]: '动态IP',
  [ProxyUpstreamKind.PROXY_UPSTREAM_KIND_PROXY_POOL]: '代理池',
  [ProxyRotationMode.PROXY_ROTATION_MODE_NONE]: '固定',
  [ProxyRotationMode.PROXY_ROTATION_MODE_PER_REQUEST]: '请求轮换',
  [ProxyRotationMode.PROXY_ROTATION_MODE_STICKY_SESSION]: '粘性会话',
  [ProxyRotationMode.PROXY_ROTATION_MODE_SCHEDULED_POOL_REFRESH]: '定时刷新',
  [ProxySessionMode.PROXY_SESSION_MODE_STICKY]: 'Sticky',
  [ProxySessionMode.PROXY_SESSION_MODE_ROTATING]: 'Rotating',
  [EgressHopRole.EGRESS_HOP_ROLE_FORWARD]: 'Forward',
  [EgressHopRole.EGRESS_HOP_ROLE_EXIT]: 'Exit',
  [EgressHopRole.EGRESS_HOP_ROLE_CONTROL_PLANE]: 'Control',
  [ProxySelectorStrategy.PROXY_SELECTOR_STRATEGY_ROUND_ROBIN]: 'Round robin',
  [ProxySelectorStrategy.PROXY_SELECTOR_STRATEGY_RANDOM]: 'Random',
  [ProxySelectorStrategy.PROXY_SELECTOR_STRATEGY_FIFO]: 'FIFO',
  [ProxySelectorStrategy.PROXY_SELECTOR_STRATEGY_HASH_CLIENT_IP]: 'Hash client',
  [ProxySelectorStrategy.PROXY_SELECTOR_STRATEGY_HASH_TARGET_HOST]: 'Hash target',
  [ProxyCapability.PROXY_CAPABILITY_CHAINING]: 'Chaining',
  [ProxyCapability.PROXY_CAPABILITY_POOL_REFRESH]: 'Pool refresh',
  [ProxyCapability.PROXY_CAPABILITY_API_POOL]: 'API pool',
  [ProxyCapability.PROXY_CAPABILITY_STICKY_SESSION]: 'Sticky session',
  [ProxyCapability.PROXY_CAPABILITY_ACTIVE_SESSION_ROTATION]: 'Active session',
  [ProxyCapability.PROXY_CAPABILITY_USERNAME_PARAMETER_SESSION]: 'Username session',
  [ProxyCapability.PROXY_CAPABILITY_UNIFIED_EGRESS_GATEWAY]: 'Egress gateway',
  [ProxyCapability.PROXY_CAPABILITY_DYNAMIC_LEASE]: 'Dynamic lease',
  [ProxyCapability.PROXY_CAPABILITY_SUBSCRIPTION_PROVIDER]: 'Subscription'
};

export function enumLabel(value: string | undefined) {
  if (!value) return '-';
  return labels[value] || value.replace(/^[A-Z_]+_/, '').replaceAll('_', ' ').toLowerCase();
}

export function formatTime(value: string | undefined) {
  if (!value) return '-';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString();
}

export function endpointAddr(host: string, port: number) {
  return port ? `${host}:${port}` : host || '-';
}
