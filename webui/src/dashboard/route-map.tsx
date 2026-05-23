import { ArrowRight, Route } from 'lucide-react';
import { Badge, EmptyBlock } from '@/dashboard/module-kit';
import type { EgressGateway, EgressHop, EgressRoute } from './proto/byte/v/forge/contracts/proxyruntime/v1/proxy_runtime';
import { endpointAddr, enumLabel } from './labels';

export function RouteMap({ gateway }: { gateway?: EgressGateway }) {
  if (!gateway) return <EmptyBlock text="暂无网关数据。proxy-runtime 尚未返回出口网关快照。" />;
  return (
    <div className="proxyRoutePane">
      <RouteSection title="Data plane" route={gateway.data_plane_route} />
      <RouteSection title="Control plane" route={gateway.control_plane_route} />
    </div>
  );
}

function RouteSection({ title, route }: { title: string; route?: EgressRoute }) {
  const hops = route?.hops || [];
  return (
    <section className="proxyRouteSection">
      <div className="proxyRouteTitle"><Route size={15} />{title}<span>{route?.route_id || '-'}</span></div>
      {hops.length === 0 ? (
        <EmptyBlock text="未配置路由。当前路径没有 hop。" />
      ) : (
        <div className="proxyHopRail">
          {hops.map((hop, index) => (
            <div className="proxyHopStep" key={hop.hop_id}>
              <HopBlock hop={hop} />
              {index < hops.length - 1 && <ArrowRight className="proxyHopArrow" size={18} />}
            </div>
          ))}
        </div>
      )}
    </section>
  );
}

function HopBlock({ hop }: { hop: EgressHop }) {
  return (
    <div className="proxyHop">
      <div className="proxyHopHeader">
        <strong>{hop.hop_id}</strong>
        <Badge variant="outline">{enumLabel(hop.role)}</Badge>
      </div>
      <div className="proxyHopMeta">
        <span>{enumLabel(hop.selector?.strategy)}</span>
        <span>{hop.endpoints.length} endpoints</span>
      </div>
      <div className="proxyHopEndpoints">
        {hop.endpoints.slice(0, 3).map((endpoint) => (
          <span key={endpoint.id}>{endpointAddr(endpoint.host, endpoint.port)} · {enumLabel(endpoint.upstream_kind)}</span>
        ))}
      </div>
    </div>
  );
}
