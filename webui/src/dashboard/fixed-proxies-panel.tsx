import { useState } from 'react';
import { Plus, Trash2 } from 'lucide-react';
import { Button, DashboardField, Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle, EmptyBlock, Input, Table, TableBody, TableCell, TableHead, TableHeader, TableRow, Textarea, useForm } from '@byte-v-forge/common-ui';
import type { ProxySourceDescriptor, UpsertProxyFixedSourceRequest } from '@byte-v-forge/common-ui/proto/byte/v/forge/contracts/proxyruntime/v1/proxy_runtime';

type FixedForm = { display_name: string; uri: string; region_codes: string };

export function FixedProxiesPanel({ sources, busy, onSave, onDelete }: {
  sources: ProxySourceDescriptor[]; busy?: boolean;
  onSave: (req: UpsertProxyFixedSourceRequest) => void; onDelete: (id: string) => void;
}) {
  const [open, setOpen] = useState(false);
  const fixedSources = sources.filter((item) => item.fixed_proxy && item.provider_id === 'mihomo');
  return <div className="proxyStack">
    <div className="proxyToolbar"><Button onClick={() => setOpen(true)}><Plus size={15} />导入 VLESS</Button></div>
    <FixedTable sources={fixedSources} busy={busy} onDelete={onDelete} />
    <FixedDialog open={open} busy={busy} onOpenChange={setOpen} onSubmit={onSave} />
  </div>;
}

function FixedTable({ sources, busy, onDelete }: { sources: ProxySourceDescriptor[]; busy?: boolean; onDelete: (id: string) => void }) {
  if (sources.length === 0) return <EmptyBlock text="暂无固定代理。可从 VLESS 链接导入。" />;
  return <div className="proxyTableWrap"><Table><TableHeader><TableRow><TableHead>名称</TableHead><TableHead>端点</TableHead><TableHead>状态</TableHead><TableHead /></TableRow></TableHeader><TableBody>{sources.map((item) => <TableRow key={item.source_id}>
    <TableCell>{item.display_name || item.source_id}</TableCell><TableCell>{item.fixed_proxy?.endpoint_count || 0}</TableCell><TableCell>{item.enabled ? '启用' : '停用'}</TableCell>
    <TableCell><Button disabled={busy} onClick={() => onDelete(item.source_id)}><Trash2 size={14} />删除</Button></TableCell>
  </TableRow>)}</TableBody></Table></div>;
}

function FixedDialog({ open, busy, onOpenChange, onSubmit }: { open: boolean; busy?: boolean; onOpenChange: (v: boolean) => void; onSubmit: (req: UpsertProxyFixedSourceRequest) => void }) {
  const form = useForm<FixedForm>({ defaultValues: { display_name: '', uri: '', region_codes: '' } });
  return <Dialog open={open} onOpenChange={onOpenChange}><DialogContent className="max-h-[90vh] w-[92vw] max-w-[720px] overflow-hidden p-0 sm:max-w-[720px]"><DialogHeader className="px-6 pt-6"><DialogTitle>导入固定代理</DialogTitle></DialogHeader><form className="grid min-h-0 min-w-0 gap-4 px-6 pb-6" onSubmit={form.handleSubmit((v) => { onSubmit(toFixed(v)); onOpenChange(false); form.reset(); })}>
    <div className="grid max-h-[62vh] min-h-0 min-w-0 gap-3 overflow-y-auto overflow-x-hidden pr-1">
      <DashboardField label="显示名"><Input placeholder="可选；不填使用链接名称" {...form.register('display_name')} /></DashboardField>
      <DashboardField label="区域"><Input placeholder="US,JP,AS" {...form.register('region_codes')} /></DashboardField>
      <DashboardField label="VLESS 链接"><Textarea wrap="soft" className="h-48 max-h-[48vh] min-w-0 resize-y overflow-auto break-all font-mono text-xs whitespace-pre-wrap ![field-sizing:fixed] [overflow-wrap:anywhere]" placeholder="vless://..." {...form.register('uri')} /></DashboardField>
    </div>
    <DialogFooter className="border-t pt-4"><Button disabled={busy} type="submit">导入</Button></DialogFooter>
  </form></DialogContent></Dialog>;
}

function toFixed(v: FixedForm): UpsertProxyFixedSourceRequest {
  return { source_id: '', display_name: v.display_name, enabled: true, uri: v.uri, clear_uri: false, region_codes: splitRegions(v.region_codes) };
}

function splitRegions(value: string) { return value.split(',').map((item) => item.trim().toUpperCase()).filter(Boolean); }
