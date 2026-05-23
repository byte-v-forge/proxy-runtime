import { Badge, EmptyBlock, Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/dashboard/module-kit';
import type { ProxyEndpoint } from './proto/byte/v/forge/contracts/proxyruntime/v1/proxy_runtime';
import { endpointAddr, enumLabel } from './labels';

export function EndpointTable({ endpoints }: { endpoints: ProxyEndpoint[] }) {
  if (endpoints.length === 0) return <EmptyBlock text="暂无端点。代理池刷新后会在这里显示出口端点。" />;
  return (
    <div className="proxyTableWrap">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>ID</TableHead>
            <TableHead>地址</TableHead>
            <TableHead>类型</TableHead>
            <TableHead>轮换</TableHead>
            <TableHead>协议</TableHead>
            <TableHead>会话</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {endpoints.map((endpoint) => (
            <TableRow key={endpoint.id}>
              <TableCell className="font-medium">{endpoint.id}</TableCell>
              <TableCell>{endpointAddr(endpoint.host, endpoint.port)}</TableCell>
              <TableCell><Badge variant="outline">{enumLabel(endpoint.upstream_kind)}</Badge></TableCell>
              <TableCell>{enumLabel(endpoint.rotation_mode)}</TableCell>
              <TableCell>{enumLabel(endpoint.protocol)}</TableCell>
              <TableCell className="proxyMono">{endpoint.session_id || '-'}</TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  );
}
