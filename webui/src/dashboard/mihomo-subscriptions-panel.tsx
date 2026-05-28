import { useState } from 'react';
import { CircleCheck, CircleHelp, CircleX, Eye, Plus, Trash2 } from 'lucide-react';
import { Badge, Button, Card, CardContent, CardHeader, CardTitle, DashboardField, Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle, EmptyBlock, Input, useForm, useQuery, Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@byte-v-forge/common-ui';
import { ProxySourceNodeStatus, type ProxySourceDescriptor, type ProxySourceNode, type UpsertProxySubscriptionSourceRequest } from '@byte-v-forge/common-ui/proto/byte/v/forge/contracts/proxyruntime/v1/proxy_runtime';
import { listProxySourceNodes, proxyRuntimeKeys } from './api';
import { formatTime } from './labels';

type SourceForm = { display_name: string; url: string; region_codes: string; interval: number; health_check_url: string };

export function MihomoSubscriptionsPanel({ sources, busy, onSave, onDelete }: {
  sources: ProxySourceDescriptor[]; busy?: boolean;
  onSave: (req: UpsertProxySubscriptionSourceRequest) => void; onDelete: (id: string) => void;
}) {
  const [open, setOpen] = useState(false);
  const [nodeSource, setNodeSource] = useState<ProxySourceDescriptor | undefined>();
  const subscriptions = sources.filter((item) => item.subscription);
  return <div className="proxyStack">
    <div className="proxyToolbar"><Button onClick={() => setOpen(true)}><Plus size={15} />订阅链接</Button></div>
    <SourceTable sources={subscriptions} busy={busy} onDelete={onDelete} onViewNodes={setNodeSource} />
    <SourceNodesDialog source={nodeSource} onOpenChange={(next) => { if (!next) setNodeSource(undefined); }} />
    <SourceDialog open={open} busy={busy} onOpenChange={setOpen} onSubmit={onSave} />
  </div>;
}

function SourceTable({ sources, busy, onDelete, onViewNodes }: { sources: ProxySourceDescriptor[]; busy?: boolean; onDelete: (id: string) => void; onViewNodes: (source: ProxySourceDescriptor) => void }) {
  if (sources.length === 0) return <EmptyBlock text="暂无 Mihomo 订阅链接。" />;
  return <div className="proxyTableWrap"><Table><TableHeader><TableRow><TableHead>名称</TableHead><TableHead>URL</TableHead><TableHead>健康检查</TableHead><TableHead /></TableRow></TableHeader><TableBody>{sources.map((item) => <TableRow key={item.source_id}>
    <TableCell>{item.display_name || item.source_id}</TableCell><TableCell>{item.subscription?.url_redacted || '-'}</TableCell><TableCell>{item.subscription?.health_check_url || '-'}</TableCell>
    <TableCell className="flex justify-end gap-2"><Button disabled={busy} onClick={() => onViewNodes(item)}><Eye size={14} />查看节点</Button><Button disabled={busy} onClick={() => onDelete(item.source_id)}><Trash2 size={14} />删除</Button></TableCell>
  </TableRow>)}</TableBody></Table></div>;
}

function SourceNodesDialog({ source, onOpenChange }: { source?: ProxySourceDescriptor; onOpenChange: (open: boolean) => void }) {
  const nodesQuery = useQuery({ queryKey: [...proxyRuntimeKeys.sourceNodes, source?.source_id], queryFn: () => listProxySourceNodes(source?.source_id || ''), enabled: !!source, refetchInterval: source ? 10000 : false });
  return <Dialog open={!!source} onOpenChange={onOpenChange}><DialogContent className="w-[96vw] max-w-none sm:max-w-none xl:w-[1400px]"><DialogHeader><DialogTitle>{source?.display_name || source?.source_id || '订阅节点'}</DialogTitle></DialogHeader>
    <SourceNodeCards nodes={nodesQuery.data?.nodes || []} error={nodesQuery.error} loading={nodesQuery.isLoading} />
  </DialogContent></Dialog>;
}

function SourceNodeCards({ nodes, loading, error }: { nodes: ProxySourceNode[]; loading?: boolean; error?: unknown }) {
  if (error) return <EmptyBlock text={`Mihomo 节点读取失败：${String(error)}`} />;
  if (loading) return <EmptyBlock text="正在读取 Mihomo 订阅节点..." />;
  if (nodes.length === 0) return <EmptyBlock text="暂无可展示节点；可检查订阅 URL 或等待 Mihomo 完成拉取。" />;
  return <div className="proxyNodeGrid">{nodes.map((item) => <Card key={item.node_id} className="proxyNodeCard">
    <CardHeader className="pb-2"><CardTitle className="min-w-0 break-words text-sm leading-snug">{item.display_name}</CardTitle></CardHeader>
    <CardContent className="grid gap-2 text-xs text-muted-foreground">
      <div className="flex items-center justify-between gap-3"><Badge variant="outline">{item.node_type || 'unknown'}</Badge><div className="flex shrink-0 items-center gap-2"><NodeStatusMark status={item.status} />{item.delay_ms > 0 && <Badge variant="outline">{item.delay_ms} ms</Badge>}</div></div>
      <div>检查：{formatTime(item.checked_at)}</div>{item.error_message && <div className="break-words text-destructive">{item.error_message}</div>}
    </CardContent>
  </Card>)}</div>;
}

function NodeStatusMark({ status }: { status: ProxySourceNode['status'] }) {
  const label = nodeStatusLabel(status);
  if (status === ProxySourceNodeStatus.PROXY_SOURCE_NODE_STATUS_AVAILABLE) return <CircleCheck className="h-4 w-4 shrink-0 text-emerald-500" aria-label={label} />;
  if (status === ProxySourceNodeStatus.PROXY_SOURCE_NODE_STATUS_UNAVAILABLE) return <CircleX className="h-4 w-4 shrink-0 text-red-500" aria-label={label} />;
  return <CircleHelp className="h-4 w-4 shrink-0 text-amber-500" aria-label={label} />;
}

function nodeStatusLabel(status: ProxySourceNode['status']) {
  if (status === ProxySourceNodeStatus.PROXY_SOURCE_NODE_STATUS_AVAILABLE) return '可用';
  if (status === ProxySourceNodeStatus.PROXY_SOURCE_NODE_STATUS_UNAVAILABLE) return '不可用';
  return '未知';
}

function SourceDialog({ open, busy, onOpenChange, onSubmit }: { open: boolean; busy?: boolean; onOpenChange: (v: boolean) => void; onSubmit: (req: UpsertProxySubscriptionSourceRequest) => void }) {
  const form = useForm<SourceForm>({ defaultValues: { display_name: '', url: '', region_codes: '', interval: 3600, health_check_url: 'https://www.gstatic.com/generate_204' } });
  return <Dialog open={open} onOpenChange={onOpenChange}><DialogContent className="max-w-xl"><DialogHeader><DialogTitle>Mihomo 订阅链接</DialogTitle></DialogHeader><form className="proxyFormGrid" onSubmit={form.handleSubmit((v) => { onSubmit(toSource(v)); onOpenChange(false); form.reset(); })}>
    <DashboardField label="显示名"><Input placeholder="可选" {...form.register('display_name')} /></DashboardField>
    <DashboardField label="订阅 URL"><Input type="password" {...form.register('url')} /></DashboardField>
    <DashboardField label="区域"><Input placeholder="US,JP,AS" {...form.register('region_codes')} /></DashboardField>
    <DashboardField label="刷新间隔秒"><Input min={60} type="number" {...form.register('interval', { valueAsNumber: true })} /></DashboardField>
    <DashboardField label="健康检查 URL"><Input {...form.register('health_check_url')} /></DashboardField>
    <DialogFooter className="md:col-span-2"><Button disabled={busy} type="submit">保存订阅</Button></DialogFooter>
  </form></DialogContent></Dialog>;
}

function toSource(v: SourceForm): UpsertProxySubscriptionSourceRequest {
  return { source_id: '', display_name: v.display_name, enabled: true, url: v.url, clear_url: false, interval: `${Math.max(60, v.interval || 3600)}s`, filter: '', exclude_filter: '', health_check_url: v.health_check_url, health_interval: '300s', health_timeout: '5s', health_lazy: true, expected_status: 204, region_codes: splitRegions(v.region_codes) };
}

function splitRegions(value: string) { return value.split(',').map((item) => item.trim().toUpperCase()).filter(Boolean); }
