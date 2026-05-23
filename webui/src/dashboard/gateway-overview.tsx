import { Badge } from '@/dashboard/module-kit';
import type { EgressGateway, ProxyPoolSnapshot, ProxyProviderDescriptor } from './proto/byte/v/forge/contracts/proxyruntime/v1/proxy_runtime';
import { endpointAddr, enumLabel, formatTime } from './labels';

export function GatewayOverview({ gateway, pool, provider }: {
  gateway?: EgressGateway;
  pool?: ProxyPoolSnapshot;
  provider?: ProxyProviderDescriptor;
}) {
  const dataHops = gateway?.data_plane_route?.hops?.length || 0;
  const controlHops = gateway?.control_plane_route?.hops?.length || 0;
  const endpointCount = pool?.endpoints?.length || 0;
  const listeners = gateway?.listeners || [];
  const primary = listeners[0];
  return (
    <div className="proxyOverview">
      <Metric label="监听" value={primary ? endpointAddr(primary.listen_addr, 0) : '-'} detail={`${listeners.length} listeners`} />
      <Metric label="出口端点" value={String(endpointCount)} detail={`${dataHops} hop data plane`} />
      <Metric label="控制面" value={gateway?.provider_control_plane?.uses_proxy ? '代理访问' : '直连'} detail={`${controlHops} hop control plane`} />
      <Metric label="Provider" value={provider?.display_name || '-'} detail={formatTime(pool?.refreshed_at)} />
      {listeners.slice(0, 4).map((listener) => (
        <Badge key={listener.listener_id} variant="outline">
          {listener.listener_id} · {endpointAddr(listener.listen_addr, 0)} · {enumLabel(listener.kind)}
        </Badge>
      ))}
      <div className="proxyCapabilities">
        {(provider?.capabilities || []).map((capability) => (
          <Badge key={capability} variant="outline">{enumLabel(capability)}</Badge>
        ))}
      </div>
    </div>
  );
}

function Metric({ label, value, detail }: { label: string; value: string; detail: string }) {
  return (
    <div className="proxyMetric">
      <span>{label}</span>
      <strong>{value}</strong>
      <small>{detail}</small>
    </div>
  );
}
