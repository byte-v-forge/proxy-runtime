import { useMemo, useState } from 'react';
import { Network, RefreshCw } from 'lucide-react';
import { ToastMessage, ToolbarActionButtons, WorkspaceTabbedPanel, useMutation, useQuery, useQueryClient, useToastMessage } from '@byte-v-forge/common-ui';
import { deleteProviderAccount, deleteProxySource, getEgressGateway, getProxyPool, listDynamicLeases, listProviderAccounts, listProxyProviders, listProxySources, proxyRuntimeKeys, refreshProxyPool, upsertFixedSource, upsertProviderAccount, upsertSubscriptionSource } from './api';
import { DynamicProxyAccountsPanel } from './dynamic-proxy-accounts-panel';
import { FixedProxiesPanel } from './fixed-proxies-panel';
import { MihomoSubscriptionsPanel } from './mihomo-subscriptions-panel';
import { OverviewPanel } from './overview-panel';
import { RuntimeSettingsPanel } from './runtime-settings-panel';

export function ProxyRuntimePage() {
  const [tab, setTab] = useState('overview');
  const qc = useQueryClient();
  const toast = useToastMessage();
  const gatewayQuery = useQuery({ queryKey: proxyRuntimeKeys.gateway, queryFn: getEgressGateway, refetchInterval: 10000 });
  const providersQuery = useQuery({ queryKey: proxyRuntimeKeys.providers, queryFn: listProxyProviders });
  const poolQuery = useQuery({ queryKey: proxyRuntimeKeys.pool, queryFn: getProxyPool, refetchInterval: 10000 });
  const accountsQuery = useQuery({ queryKey: proxyRuntimeKeys.accounts, queryFn: listProviderAccounts });
  const sourcesQuery = useQuery({ queryKey: proxyRuntimeKeys.sources, queryFn: listProxySources });
  const leasesQuery = useQuery({ queryKey: proxyRuntimeKeys.leases, queryFn: listDynamicLeases, refetchInterval: 10000 });
  const gateway = gatewayQuery.data?.gateway;
  const pool = poolQuery.data?.pool || gateway?.pool;
  const accounts = accountsQuery.data?.accounts || [];
  const providers = providersQuery.data?.providers || [];
  const sources = sourcesQuery.data?.sources || pool?.sources || [];
  const leases = leasesQuery.data?.leases || pool?.dynamic_leases || [];

  const refreshMutation = useMutation({ mutationFn: refreshProxyPool, onSuccess: () => done(qc, toast, '代理基础池已刷新'), onError: toast.showError });
  const accountMutation = useMutation({ mutationFn: upsertProviderAccount, onSuccess: () => done(qc, toast, '动态代理账号已保存'), onError: toast.showError });
  const accountDelete = useMutation({ mutationFn: deleteProviderAccount, onSuccess: () => done(qc, toast, '动态代理账号已删除'), onError: toast.showError });
  const sourceMutation = useMutation({ mutationFn: upsertSubscriptionSource, onSuccess: () => done(qc, toast, '订阅源已保存'), onError: toast.showError });
  const fixedMutation = useMutation({ mutationFn: upsertFixedSource, onSuccess: () => done(qc, toast, '固定代理已导入'), onError: toast.showError });
  const sourceDelete = useMutation({ mutationFn: deleteProxySource, onSuccess: () => done(qc, toast, '订阅源已删除'), onError: toast.showError });

  const meta = useMemo(() => `${pool?.endpoints?.length || 0} endpoints · ${leases.length} leases · ${accounts.length} accounts`, [accounts.length, leases.length, pool?.endpoints?.length]);
  const busy = accountMutation.isPending || sourceMutation.isPending || fixedMutation.isPending;

  return <><ToastMessage toast={toast.toast} /><WorkspaceTabbedPanel value={tab} onValueChange={setTab} title={<span className="inline-flex items-center gap-2"><Network className="size-4" />Proxy Runtime</span>} meta={gatewayQuery.isLoading ? '加载中...' : meta} actions={<ToolbarActionButtons actions={[{ label: '刷新基础池', icon: <RefreshCw size={15} />, disabled: refreshMutation.isPending, onClick: () => refreshMutation.mutate() }]} />} tabs={[
    { value: 'overview', label: '概览', content: <OverviewPanel gateway={gateway} pool={pool} leases={leases} /> },
    { value: 'dynamic-accounts', label: '动态代理账号', content: <DynamicProxyAccountsPanel accounts={accounts} providers={providers} busy={busy} onSave={(req) => accountMutation.mutate(req)} onDelete={(account_id) => accountDelete.mutate({ account_id })} /> },
    { value: 'mihomo-subscriptions', label: 'Mihomo 订阅', content: <MihomoSubscriptionsPanel sources={sources} busy={busy} onSave={(req) => sourceMutation.mutate(req)} onDelete={(source_id) => sourceDelete.mutate({ source_id })} /> },
    { value: 'fixed-proxies', label: '固定代理', content: <FixedProxiesPanel sources={sources} busy={busy} onSave={(req) => fixedMutation.mutate(req)} onDelete={(source_id) => sourceDelete.mutate({ source_id })} /> },
    { value: 'settings', label: '通用配置', content: <RuntimeSettingsPanel onSaved={() => toast.showOK('通用配置已保存')} onError={toast.showError} /> }
  ]} /></>;
}

function done(queryClient: ReturnType<typeof useQueryClient>, toast: ReturnType<typeof useToastMessage>, message: string) {
  toast.showOK(message);
  return Promise.all(Object.values(proxyRuntimeKeys).map((queryKey) => queryClient.invalidateQueries({ queryKey })));
}
