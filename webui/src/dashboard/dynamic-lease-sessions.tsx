import { Badge, EmptyBlock } from '@byte-v-forge/common-ui';
import type { ProxyDynamicLease } from '@byte-v-forge/common-ui/proto/byte/v/forge/contracts/proxyruntime/v1/proxy_runtime';
import { endpointAddr, enumLabel, formatTime } from './labels';

export function DynamicLeaseSessions({ leases }: { leases: ProxyDynamicLease[] }) {
  return leases.length === 0 ? <EmptyBlock text="暂无动态 IP lease 会话。" /> : <div className="proxyLeaseGrid">{leases.map((lease) => <LeaseCard key={lease.lease_id || lease.account_id} lease={lease} />)}</div>;
}

function LeaseCard({ lease }: { lease: ProxyDynamicLease }) {
  const plan = lease.chain_plan;
  const gateway = plan?.dynamic_gateway;
  const line = plan?.line;
  return <article className="proxyLeaseCard">
    <header className="proxyLeaseHeader">
      <div className="min-w-0"><h4>{lease.account_id}</h4><p>{lease.purpose || 'default'} · {lease.provider_account_id || '-'}</p></div>
      <Badge variant="outline">{enumLabel(lease.status)}</Badge>
    </header>
    <div className="proxyLeaseFacts">
      <LeaseFact label="Line" value={line?.display_name || 'direct gateway'} />
      <LeaseFact label="Gateway" value={gateway ? `${gateway.provider_id}/${gateway.gateway_id}` : '-'} />
      <LeaseFact label="Egress" value={endpointAddr(lease.egress?.host || '', lease.egress?.port || 0)} />
      <LeaseFact label="Expires" value={formatTime(lease.expires_at)} />
    </div>
    <div className="proxyLeaseSignals">
      {line?.region_codes?.slice(0, 4).map((region) => <Badge key={region} variant="outline">{region}</Badge>)}
      {gateway?.region_codes?.slice(0, 3).map((region) => <Badge key={`g-${region}`} variant="outline">GW {region}</Badge>)}
      <Badge variant="outline">IP {riskText(lease.ip_fraud_check?.risk_level, lease.ip_fraud_check?.risk_score)}</Badge>
      <Badge variant="outline">CF {riskText(lease.edge_access_check?.risk_level, lease.edge_access_check?.risk_score)}</Badge>
    </div>
  </article>;
}

function LeaseFact({ label, value }: { label: string; value: string }) {
  return <div><span>{label}</span><strong title={value}>{value}</strong></div>;
}

function riskText(level?: string, score?: number) {
  if (!level && !score) return '-';
  const suffix = typeof score === 'number' && score > 0 ? ` ${score}` : '';
  return `${enumLabel(level)}${suffix}`;
}
