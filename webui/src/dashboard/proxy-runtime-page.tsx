import { useEffect, useMemo, useState } from 'react';
import { Network, RefreshCw, RotateCcw } from 'lucide-react';
import {
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
  ToastMessage,
  ToolbarActionButtons,
  WorkspaceToolbar,
  useMutation,
  useQuery,
  useQueryClient,
  useToastMessage
} from '@/dashboard/module-kit';
import { createProxySession, getEgressGateway, getProxyPool, listProxyProviders, proxyRuntimeKeys, refreshProxyPool } from './api';
import type { CreateProxySessionRequest } from './proto/byte/v/forge/contracts/proxyruntime/v1/proxy_runtime';
import { GatewayOverview } from './gateway-overview';
import { EndpointTable } from './endpoint-table';
import { RouteMap } from './route-map';
import { SessionPanel } from './session-panel';

export function ProxyRuntimePage() {
  const [tab, setTab] = useState('route');
  const queryClient = useQueryClient();
  const toast = useToastMessage();
  const gatewayQuery = useQuery({ queryKey: proxyRuntimeKeys.gateway, queryFn: getEgressGateway, refetchInterval: 10000 });
  const poolQuery = useQuery({ queryKey: proxyRuntimeKeys.pool, queryFn: getProxyPool, refetchInterval: 10000 });
  const providersQuery = useQuery({ queryKey: proxyRuntimeKeys.providers, queryFn: listProxyProviders });
  const gateway = gatewayQuery.data?.gateway;
  const pool = poolQuery.data?.pool || gateway?.pool;
  const provider = providersQuery.data?.providers?.[0] || pool?.provider;

  const refreshMutation = useMutation({
    mutationFn: refreshProxyPool,
    onSuccess: async () => {
      await invalidateProxyRuntime(queryClient);
      toast.showOK('代理池已刷新');
    },
    onError: toast.showError
  });
  const sessionMutation = useMutation({
    mutationFn: (req: CreateProxySessionRequest) => createProxySession(req),
    onSuccess: async () => {
      await invalidateProxyRuntime(queryClient);
      toast.showOK('Sticky会话已切换');
    },
    onError: toast.showError
  });

  useEffect(() => {
    if (gatewayQuery.error) toast.showError(gatewayQuery.error);
  }, [gatewayQuery.error, toast.showError]);

  const meta = useMemo(() => {
    const count = pool?.endpoints?.length || 0;
    const listenerCount = gateway?.listeners?.length || 0;
    return `${count}个Endpoint · ${listenerCount}个Listener · ${provider?.display_name || 'Provider'}`;
  }, [gateway?.listeners?.length, pool?.endpoints?.length, provider?.display_name]);

  return (
    <>
      <ToastMessage toast={toast.toast} />
      <section className="workspace proxyRuntimeWorkspace">
        <div className="panel proxyRuntimePanel">
          <Tabs value={tab} onValueChange={setTab} className="flex min-h-0 flex-1 flex-col">
            <WorkspaceToolbar
              title={<span className="inline-flex items-center gap-2"><Network className="size-4" />出口网关</span>}
              meta={gatewayQuery.isLoading ? '正在加载...' : meta}
              tabs={<TabsList><TabsTrigger value="route">路由</TabsTrigger><TabsTrigger value="endpoints">端点</TabsTrigger><TabsTrigger value="session">会话</TabsTrigger></TabsList>}
              actions={<ToolbarActionButtons actions={[
                { label: '刷新代理池', icon: <RefreshCw size={15} />, disabled: refreshMutation.isPending, onClick: () => refreshMutation.mutate() },
                { label: '新Sticky会话', icon: <RotateCcw size={15} />, disabled: sessionMutation.isPending || !provider?.supports_active_new_session, tone: 'primary', onClick: () => setTab('session') }
              ]} />}
            />
            <GatewayOverview gateway={gateway} pool={pool} provider={provider} />
            <TabsContent value="route" className="mt-0 min-h-0 flex-1">
              <RouteMap gateway={gateway} />
            </TabsContent>
            <TabsContent value="endpoints" className="mt-0 min-h-0 flex-1">
              <EndpointTable endpoints={pool?.endpoints || []} />
            </TabsContent>
            <TabsContent value="session" className="mt-0 min-h-0 flex-1">
              <SessionPanel provider={provider} activeSession={pool?.active_session} busy={sessionMutation.isPending} onCreate={(req) => sessionMutation.mutate(req)} />
            </TabsContent>
          </Tabs>
        </div>
      </section>
    </>
  );
}

function invalidateProxyRuntime(queryClient: ReturnType<typeof useQueryClient>) {
  return Promise.all([
    queryClient.invalidateQueries({ queryKey: proxyRuntimeKeys.gateway }),
    queryClient.invalidateQueries({ queryKey: proxyRuntimeKeys.pool }),
    queryClient.invalidateQueries({ queryKey: proxyRuntimeKeys.providers })
  ]);
}
