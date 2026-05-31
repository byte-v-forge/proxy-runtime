import { useEffect, useMemo, type ReactNode } from 'react';
import { Cloud, Gauge, Save } from 'lucide-react';
import {
  Badge,
  Button,
  Controller,
  DashboardField,
  Input,
  Switch,
  useForm,
  useMutation,
  useQuery,
  useQueryClient
} from '@byte-v-forge/common-ui';
import { useFieldArray } from 'react-hook-form';
import { getProxyRuntimeSettings, listIPFraudProviders, listProxyProviders, proxyRuntimeKeys, updateProxyRuntimeSettings } from './api';
import { DynamicGatewaySettings } from './dynamic-gateway-settings';
import { IPFraudProviderSettings } from './ip-fraud-provider-settings';
import { defaultSettingsForm, formFromSettings, providerCatalogFrom, requestFromSettingsForm, type RuntimeSettingsForm } from './settings-model';

export function RuntimeSettingsPanel({ onSaved, onError }: { onSaved?: () => void; onError?: (error: unknown) => void }) {
  const queryClient = useQueryClient();
  const query = useQuery({ queryKey: proxyRuntimeKeys.settings, queryFn: getProxyRuntimeSettings });
  const providersQuery = useQuery({ queryKey: proxyRuntimeKeys.ipFraudProviders, queryFn: listIPFraudProviders });
  const proxyProvidersQuery = useQuery({ queryKey: proxyRuntimeKeys.providers, queryFn: listProxyProviders });
  const form = useForm<RuntimeSettingsForm>({ defaultValues: defaultSettingsForm });
  const providerFields = useFieldArray({ control: form.control, name: 'providers', keyName: 'fieldId' });
  const providers = form.watch('providers');
  const edgeEnabled = form.watch('edgeEnabled');
  const catalog = useMemo(() => providerCatalogFrom(providersQuery.data?.providers), [providersQuery.data?.providers]);
  const mutation = useMutation({
    mutationFn: (values: RuntimeSettingsForm) => updateProxyRuntimeSettings(requestFromSettingsForm(values)),
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: proxyRuntimeKeys.settings }),
        queryClient.invalidateQueries({ queryKey: proxyRuntimeKeys.providers }),
        queryClient.invalidateQueries({ queryKey: proxyRuntimeKeys.sources }),
        queryClient.invalidateQueries({ queryKey: proxyRuntimeKeys.pool }),
        queryClient.invalidateQueries({ queryKey: proxyRuntimeKeys.ipFraudCheck }),
        queryClient.invalidateQueries({ queryKey: proxyRuntimeKeys.edgeAccess })
      ]);
      onSaved?.();
    },
    onError
  });

  useEffect(() => {
    if (query.data?.settings) form.reset(formFromSettings(query.data.settings, catalog));
  }, [catalog, form, query.data?.settings]);
  useEffect(() => {
    if (query.error) onError?.(query.error);
  }, [onError, query.error]);

  const edge = query.data?.settings?.edge_canary;
  const saving = mutation.isPending || query.isLoading;

  return (
    <form className="grid h-full min-h-0 grid-rows-[1fr_auto] overflow-hidden bg-[var(--surface-soft)]" onSubmit={form.handleSubmit((values) => mutation.mutate(values))}>
      <div className="grid min-h-0 content-start gap-4 overflow-y-auto overflow-x-hidden p-4">
        <div className="rounded-xl border border-[var(--border-soft)] bg-[var(--surface)] p-4 shadow-sm">
          <div className="flex flex-wrap items-start justify-between gap-3">
            <div><h2 className="m-0 text-base font-semibold">Proxy Runtime 配置</h2><p className="m-0 text-xs text-muted-foreground">统一维护可热更新的检测、风控和动态代理网关配置。</p></div>
            <div className="flex flex-wrap gap-2"><StatusBadge label="Canary" enabled={!!edge?.enabled} /><StatusBadge label="IP Fraud" enabled={(query.data?.settings?.ip_fraud_providers?.length || 0) > 0} /></div>
          </div>
        </div>

        <div className="grid gap-4 xl:grid-cols-[minmax(0,1.25fr)_minmax(300px,0.75fr)]">
          <section className="grid gap-3 rounded-xl border border-[var(--border-soft)] bg-[var(--surface)] p-4 shadow-sm">
            <SectionTitle icon={<Cloud />} title="Cloudflare Canary" description="显式启用后才执行；未启用、URL 或 token 未配置时返回 unsupported。" />
            <div className="flex items-center justify-between gap-3 rounded-lg border border-[var(--border-soft)] bg-[var(--surface-soft)] p-3">
              <div><p className="m-0 text-sm font-medium">启用 Canary</p><p className="m-0 text-xs text-muted-foreground">关闭只停用检测，不删除已保存配置。</p></div>
              <Controller control={form.control} name="edgeEnabled" render={({ field }) => <Switch checked={field.value} onCheckedChange={field.onChange} />} />
            </div>
            <div className="grid gap-3 md:grid-cols-2">
              <DashboardField label="Canary URL"><Input disabled={!edgeEnabled} placeholder="https://.../edge-canary" {...form.register('edgeUrl')} /></DashboardField>
              <DashboardField label="Canary Token"><Input autoComplete="new-password" disabled={!edgeEnabled} placeholder={edge?.token_configured ? '留空保留已配置 token' : '未配置'} type="password" {...form.register('edgeToken')} /></DashboardField>
            </div>
            <div className="flex flex-wrap gap-2"><StatusBadge label="启用状态" enabled={!!edge?.enabled} /><StatusBadge label="Token" enabled={!!edge?.token_configured} /></div>
          </section>

          <section className="grid content-start gap-3 rounded-xl border border-[var(--border-soft)] bg-[var(--surface)] p-4 shadow-sm">
            <SectionTitle icon={<Gauge />} title="运行时检查" description="控制代理出口 IP 探测等待时间；保存后新请求立即使用。" />
            <DashboardField label="出口 IP 探测超时（秒）">
              <Input min={1} step={1} type="number" {...form.register('proxyExitIpTimeoutSeconds', { valueAsNumber: true })} />
            </DashboardField>
          </section>
        </div>

        <DynamicGatewaySettings control={form.control} providers={proxyProvidersQuery.data?.providers || []} register={form.register} />
        <IPFraudProviderSettings catalog={catalog} control={form.control} fields={providerFields.fields} providers={providers} register={form.register} settings={query.data?.settings} onAdd={providerFields.append} onRemove={providerFields.remove} />
      </div>

      <div className="flex items-center justify-between gap-3 border-t border-[var(--border-soft)] bg-[var(--surface)] px-4 py-3">
        <p className="m-0 text-xs text-muted-foreground">API Key 留空会保留现有密钥；删除 Provider 行会移除该 Provider 配置。</p>
        <Button disabled={saving} type="submit"><Save />保存配置</Button>
      </div>
    </form>
  );
}

function SectionTitle({ icon, title, description }: { icon: ReactNode; title: string; description: string }) {
  return <div className="flex min-w-0 items-start gap-3"><span className="grid size-10 shrink-0 place-items-center rounded-xl bg-primary/10 text-primary [&_svg]:size-5">{icon}</span><div><h3 className="m-0 text-sm font-semibold">{title}</h3><p className="m-0 text-xs text-muted-foreground">{description}</p></div></div>;
}

function StatusBadge({ label, enabled }: { label: string; enabled: boolean }) {
  return <Badge variant={enabled ? 'secondary' : 'outline'}>{label}：{enabled ? '已配置' : '未配置'}</Badge>;
}
