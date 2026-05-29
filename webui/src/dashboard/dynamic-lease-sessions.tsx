import { Badge, EmptyBlock } from '@byte-v-forge/common-ui';
import type { ProxyDynamicLease } from '@byte-v-forge/common-ui/proto/byte/v/forge/contracts/proxyruntime/v1/proxy_runtime';
import { endpointAddr, enumLabel, formatTime } from './labels';

export function DynamicLeaseSessions({ leases }: { leases: ProxyDynamicLease[] }) {
  return leases.length === 0 ? <EmptyBlock text="暂无动态 IP lease 会话。" /> : <div className="proxyLeaseGrid">{leases.map((lease) => <LeaseCard key={lease.lease_id || lease.account_id} lease={lease} />)}</div>;
}

function LeaseCard({ lease }: { lease: ProxyDynamicLease }) {
  const plan = lease.chain_plan;
  const gateway = plan?.dynamic_gateway;
  const hops = [...(plan?.hops || [])].sort((a, b) => (a.order || 0) - (b.order || 0));
  return <article className="proxyLeaseCard">
    <header className="proxyLeaseHeader">
      <div className="min-w-0"><h4>{lease.account_id}</h4><p>{lease.purpose || 'default'}</p></div>
      <Badge variant="outline">{enumLabel(lease.status)}</Badge>
    </header>
    <div className="proxyLeaseFacts">
      <LeaseFact label="Chain" value={chainText(hops)} />
      <LeaseFact label="Gateway" value={gateway ? `${gateway.provider_id}/${gateway.gateway_id}` : '-'} />
      <LeaseFact label="Egress" value={endpointAddr(lease.egress?.host || '', lease.egress?.port || 0)} />
      <LeaseFact label="Expires" value={formatTime(lease.expires_at)} />
    </div>
    <div className="proxyLeaseSignals">
      {hops.slice(0, 4).map((hop) => <Badge key={hop.hop_id || `${hop.role}-${hop.order}`} variant="outline">{hopBadge(hop)}</Badge>)}
      {gateway?.region_codes?.slice(0, 3).map((region) => <Badge key={`g-${region}`} variant="outline">GW {region}</Badge>)}
      <Badge variant="outline">IP {riskText(lease.ip_fraud_check?.risk_level, lease.ip_fraud_check?.risk_score)}</Badge>
      <Badge variant="outline">CF {riskText(lease.edge_access_check?.risk_level, lease.edge_access_check?.risk_score)}</Badge>
    </div>
  </article>;
}

function chainText(hops: NonNullable<ProxyDynamicLease['chain_plan']>['hops']) {
  if (!hops.length) return 'direct gateway';
  return hops.map((hop) => hopName(hop)).filter(Boolean).join(' → ') || 'direct gateway';
}

function hopBadge(hop: NonNullable<ProxyDynamicLease['chain_plan']>['hops'][number]) {
  const geo = [hop.country_code, hop.region].filter(Boolean).join('/');
  return [hopName(hop), hop.observed_ip, geo].filter(Boolean).join(' · ') || enumLabel(hop.role);
}

function hopName(hop: NonNullable<ProxyDynamicLease['chain_plan']>['hops'][number]) {
  if (hop.role === 'PROXY_CHAIN_HOP_ROLE_DYNAMIC_GATEWAY') return hop.gateway_display_name || hop.gateway_id || hop.provider_id;
  if (hop.role === 'PROXY_CHAIN_HOP_ROLE_DYNAMIC_EXIT') return 'Exit';
  return hop.node_display_name || hop.source_display_name || hop.node_id || hop.source_id;
}

function LeaseFact({ label, value }: { label: string; value: string }) {
  return <div><span>{label}</span><strong title={value}>{value}</strong></div>;
}

function riskText(level?: string, score?: number) {
  if (!level && !score) return '-';
  const suffix = typeof score === 'number' && score > 0 ? ` ${score}` : '';
  return `${enumLabel(level)}${suffix}`;
}
