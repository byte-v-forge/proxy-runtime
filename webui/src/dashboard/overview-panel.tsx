import { Badge, EmptyBlock, MetricItem } from '@byte-v-forge/common-ui';
import type { ReactNode } from 'react';
import type { EgressGateway, ProxyDynamicLease, ProxyPoolSnapshot } from '@byte-v-forge/common-ui/proto/byte/v/forge/contracts/proxyruntime/v1/proxy_runtime';
import { DynamicLeaseSessions } from './dynamic-lease-sessions';
import { EndpointTable } from './endpoint-table';
import { enumLabel, formatTime } from './labels';
import { RouteMap } from './route-map';

export function OverviewPanel({ gateway, pool, leases }: { gateway?: EgressGateway; pool?: ProxyPoolSnapshot; leases?: ProxyDynamicLease[] }) {
  const overview = gateway?.overview;
  const sources = pool?.sources || [];
  const sessions = leases || pool?.dynamic_leases || [];
  const endpoints = pool?.endpoints || [];
  if (!gateway && !pool) return <EmptyBlock text="proxy-runtime 尚未返回网关数据。" />;
  return <div className="proxyOverviewShell">
    <section className="proxyOverviewHero">
      <div><h3>Proxy Runtime</h3><p>当前出口路径、订阅源和动态 IP lease 的运行快照。</p></div>
      <div className="proxyOverview">
        <MetricItem className="proxyMetric" label="Route Runtime" value={enumLabel(overview?.route_runtime)} detail={overview?.route_runtime_status || '-'} />
        <MetricItem className="proxyMetric" label="Source Runtime" value={enumLabel(overview?.source_runtime)} detail={overview?.source_runtime_status || '-'} />
        <MetricItem className="proxyMetric" label="Endpoints" value={String(endpoints.length)} detail="active egress endpoints" />
        <MetricItem className="proxyMetric" label="Leases" value={String(overview?.active_lease_count ?? sessions.length)} detail="account-level dynamic IP" />
        <MetricItem className="proxyMetric" label="Updated" value={formatTime(overview?.updated_at || gateway?.updated_at)} detail={formatTime(pool?.refreshed_at)} />
      </div>
    </section>
    <OverviewSection title="Sources" desc={`${overview?.source_count ?? sources.length} configured source descriptors`}><SourceTiles sources={sources} /></OverviewSection>
    <div className="proxyOverviewGrid"><OverviewSection title="Routes" desc="Data-plane and control-plane path"><RouteMap gateway={gateway} /></OverviewSection><OverviewSection title="Dynamic IP Leases" desc="Read-only workflow-created sessions"><DynamicLeaseSessions leases={sessions} /></OverviewSection></div>
    <OverviewSection title="Endpoints" desc="Current pool endpoints exposed through the gateway"><EndpointTable endpoints={endpoints} /></OverviewSection>
  </div>;
}

function OverviewSection({ title, desc, children }: { title: string; desc: string; children: ReactNode }) {
  return <section className="proxySection"><header className="proxySectionHeader"><div><h3>{title}</h3><p>{desc}</p></div></header>{children}</section>;
}

function SourceTiles({ sources }: { sources: NonNullable<ProxyPoolSnapshot['sources']> }) {
  if (sources.length === 0) return <EmptyBlock text="暂无 source。" />;
  return <div className="proxySourceGrid">{sources.map((source) => <article key={source.source_id} className="proxySourceTile">
    <div className="min-w-0"><h4 title={source.display_name || source.source_id}>{source.display_name || source.source_id}</h4><p>{source.provider_id} · {enumLabel(source.kind)}</p></div>
    <Badge variant="outline">{source.enabled ? 'Enabled' : 'Disabled'}</Badge>
    <small>{sourceSummary(source)}</small>
  </article>)}</div>;
}

function sourceSummary(source: NonNullable<ProxyPoolSnapshot['sources']>[number]) {
  if (source.subscription) return `${source.subscription.health_check_url || 'no health'} · ${source.subscription.region_codes?.join(',') || 'all regions'}`;
  if (source.fixed_proxy) return `${source.fixed_proxy.endpoint_count} endpoint · ${source.fixed_proxy.region_codes?.join(',') || 'all regions'}`;
  if (source.dynamic_ip) return `dynamic lease · ${source.dynamic_ip.min_sticky_ttl || '-'}-${source.dynamic_ip.max_sticky_ttl || '-'}`;
  return source.source_id;
}
