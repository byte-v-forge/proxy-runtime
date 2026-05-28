import { useState } from 'react';
import { Plus, Trash2 } from 'lucide-react';
import { Button, Controller, DashboardField, Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle, EmptyBlock, Input, Select, SelectContent, SelectItem, SelectTrigger, SelectValue, Table, TableBody, TableCell, TableHead, TableHeader, TableRow, useForm } from '@byte-v-forge/common-ui';
import { ProxyUpstreamKind, type ProxyProviderAccount, type ProxyProviderDescriptor, type UpsertProxyProviderAccountRequest } from '@byte-v-forge/common-ui/proto/byte/v/forge/contracts/proxyruntime/v1/proxy_runtime';

type AccountForm = { provider_id: string; display_name: string; username: string; password: string };

export function DynamicProxyAccountsPanel({ accounts, providers, busy, onSave, onDelete }: {
  accounts: ProxyProviderAccount[]; providers: ProxyProviderDescriptor[]; busy?: boolean;
  onSave: (req: UpsertProxyProviderAccountRequest) => void; onDelete: (id: string) => void;
}) {
  const [open, setOpen] = useState(false);
  return <div className="proxyStack">
    <div className="proxyToolbar"><Button onClick={() => setOpen(true)}><Plus size={15} />动态代理账号</Button></div>
    <AccountTable accounts={accounts} providers={providers} busy={busy} onDelete={onDelete} />
    <AccountDialog open={open} providers={providers} busy={busy} onOpenChange={setOpen} onSubmit={onSave} />
  </div>;
}

function AccountTable({ accounts, providers, busy, onDelete }: { accounts: ProxyProviderAccount[]; providers: ProxyProviderDescriptor[]; busy?: boolean; onDelete: (id: string) => void }) {
  if (accounts.length === 0) return <EmptyBlock text="暂无动态代理账号。动态 IP lease 需要先配置账号。" />;
  return <div className="proxyTableWrap"><Table><TableHeader><TableRow><TableHead>名称</TableHead><TableHead>Provider</TableHead><TableHead>凭证</TableHead><TableHead /></TableRow></TableHeader><TableBody>{accounts.map((item) => <TableRow key={item.account_id}>
    <TableCell>{item.display_name || item.account_id}</TableCell><TableCell>{providerName(providers, item.provider_id)}</TableCell><TableCell>{item.credential_configured ? '已配置' : '未配置'}</TableCell>
    <TableCell><Button disabled={busy} onClick={() => onDelete(item.account_id)}><Trash2 size={14} />删除</Button></TableCell>
  </TableRow>)}</TableBody></Table></div>;
}

function AccountDialog({ open, providers, busy, onOpenChange, onSubmit }: { open: boolean; providers: ProxyProviderDescriptor[]; busy?: boolean; onOpenChange: (v: boolean) => void; onSubmit: (req: UpsertProxyProviderAccountRequest) => void }) {
  const dynamic = dynamicProviders(providers);
  const options = providers.length ? dynamic : fallbackProviders;
  const form = useForm<AccountForm>({ defaultValues: defaultAccount(options[0]) });
  return <Dialog open={open} onOpenChange={onOpenChange}><DialogContent className="max-w-xl"><DialogHeader><DialogTitle>动态代理账号</DialogTitle></DialogHeader>{options.length === 0 ? <p className="m-0 text-sm text-muted-foreground">请先在「通用配置」里为动态 IP provider 配置 gateway。</p> : <form className="proxyFormGrid" onSubmit={form.handleSubmit((v) => { onSubmit(toAccount(v)); onOpenChange(false); form.reset(defaultAccount(options[0])); })}>
    <DashboardField label="Provider"><Controller control={form.control} name="provider_id" render={({ field }) => <Select value={field.value} onValueChange={field.onChange}><SelectTrigger><SelectValue /></SelectTrigger><SelectContent>{options.map((item) => <SelectItem key={item.provider_id} value={item.provider_id}>{item.display_name || item.provider_id}</SelectItem>)}</SelectContent></Select>} /></DashboardField>
    <DashboardField label="显示名"><Input placeholder="可选" {...form.register('display_name')} /></DashboardField>
    <DashboardField label="用户名"><Input autoComplete="username" {...form.register('username')} /></DashboardField>
    <DashboardField label="密码"><Input autoComplete="current-password" type="password" {...form.register('password')} /></DashboardField>
    <p className="m-0 text-xs text-muted-foreground md:col-span-2">代理入口和协议由 provider adapter 按官方能力选择；粘性时长在业务发起 lease/session 时传入。</p>
    <DialogFooter className="md:col-span-2"><Button disabled={busy} type="submit">保存账号</Button></DialogFooter>
  </form>}</DialogContent></Dialog>;
}

function toAccount(v: AccountForm): UpsertProxyProviderAccountRequest {
  return { account_id: '', provider_id: v.provider_id, display_name: v.display_name, enabled: true, username: v.username, password: v.password, clear_password: false };
}

const fallbackProviders: ProxyProviderDescriptor[] = [
  fallbackProvider('1024proxy', '1024Proxy'),
  fallbackProvider('cliproxy', 'Cliproxy')
];
function fallbackProvider(provider_id: string, display_name: string): ProxyProviderDescriptor {
  return { provider_id, display_name, capabilities: [], protocols: [], min_sticky_ttl: '60s', max_sticky_ttl: '7200s', upstream_kinds: [ProxyUpstreamKind.PROXY_UPSTREAM_KIND_DYNAMIC_IP], rotation_modes: [] };
}
const dynamicProviders = (items: ProxyProviderDescriptor[]) => items.filter((item) => item.upstream_kinds?.includes(ProxyUpstreamKind.PROXY_UPSTREAM_KIND_DYNAMIC_IP));
const defaultAccount = (provider?: ProxyProviderDescriptor): AccountForm => ({ provider_id: provider?.provider_id || '1024proxy', display_name: '', username: '', password: '' });
const providerName = (providers: ProxyProviderDescriptor[], id: string) => providers.find((item) => item.provider_id === id)?.display_name || id;
