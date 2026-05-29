import { useEffect, useMemo } from 'react';
import {
  Button,
  Controller,
  DashboardField,
  Input,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
  Switch,
  Textarea,
  useForm,
  useMutation,
  useQuery,
  useQueryClient
} from '@byte-v-forge/common-ui';
import type { Control } from '@byte-v-forge/common-ui';
import type { UseFormRegister } from 'react-hook-form';
import { useFieldArray } from 'react-hook-form';
import { getProxyRuntimeSettings, listIPFraudProviders, listProxyProviders, proxyRuntimeKeys, updateProxyRuntimeSettings } from './api';
import { DynamicGatewaySettings } from './dynamic-gateway-settings';
import {
  defaultSettingsForm,
  formFromSettings,
  providerCatalogFrom,
  providerDefaults,
  requestFromSettingsForm,
  type ProviderForm,
  type ProviderMode,
  type RuntimeSettingsForm
} from './settings-model';

export function RuntimeSettingsPanel({ onSaved, onError }: { onSaved?: () => void; onError?: (error: unknown) => void }) {
  const queryClient = useQueryClient();
  const query = useQuery({ queryKey: proxyRuntimeKeys.settings, queryFn: getProxyRuntimeSettings });
  const providersQuery = useQuery({ queryKey: proxyRuntimeKeys.ipFraudProviders, queryFn: listIPFraudProviders });
  const proxyProvidersQuery = useQuery({ queryKey: proxyRuntimeKeys.providers, queryFn: listProxyProviders });
  const form = useForm<RuntimeSettingsForm>({ defaultValues: defaultSettingsForm });
  const fieldArray = useFieldArray({ control: form.control, name: 'providers', keyName: 'fieldId' });
  const providers = form.watch('providers');
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
  const edgeEnabled = form.watch('edgeEnabled');
  const available = catalog.filter((item) => !providers.some((provider) => provider.kind === item.kind));
  const addProvider = (provider: (typeof catalog)[number]) => fieldArray.append(providerDefaults(provider.kind, catalog));

  return (
    <form className="grid h-full min-h-0 content-start gap-4 overflow-y-auto overflow-x-hidden bg-[var(--surface-soft)] p-4" onSubmit={form.handleSubmit((values) => mutation.mutate(values))}>
      <section className="grid gap-3 rounded-lg border border-[var(--border-soft)] bg-[var(--surface)] p-4">
        <div>
          <h3 className="m-0 text-sm font-semibold">Cloudflare Canary</h3>
          <p className="m-0 text-xs text-muted-foreground">显式启用后才执行；未启用、URL 或 token 未配置时检测结果返回 unsupported。</p>
        </div>
        <div className="flex items-center justify-between gap-3 rounded-md border border-[var(--border-soft)] p-3">
          <div><p className="m-0 text-sm font-medium">启用 Canary</p><p className="m-0 text-xs text-muted-foreground">关闭只停用检测，不删除已保存配置。</p></div>
          <Controller control={form.control} name="edgeEnabled" render={({ field }) => <Switch checked={field.value} onCheckedChange={field.onChange} />} />
        </div>
        <div className="grid gap-3 md:grid-cols-2">
          <DashboardField label="Canary URL"><Input disabled={!edgeEnabled} placeholder="https://.../edge-canary" {...form.register('edgeUrl')} /></DashboardField>
          <DashboardField label="Canary Token"><Input autoComplete="new-password" disabled={!edgeEnabled} placeholder={edge?.token_configured ? '留空保留已配置 token' : '未配置'} type="password" {...form.register('edgeToken')} /></DashboardField>
        </div>
        <p className="m-0 text-xs text-muted-foreground">当前状态：{edge?.enabled ? '启用' : '停用'}；token：{edge?.token_configured ? '已配置' : '未配置'}。</p>
      </section>

      <DynamicGatewaySettings control={form.control} providers={proxyProvidersQuery.data?.providers || []} register={form.register} />

      <section className="grid gap-3 rounded-lg border border-[var(--border-soft)] bg-[var(--surface)] p-4">
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div>
            <h3 className="m-0 text-sm font-semibold">IP 欺诈 Provider</h3>
            <p className="m-0 text-xs text-muted-foreground">添加式配置；只填写 API Key 与权重，provider URL 由后端 adapter 自己维护。</p>
          </div>
          <div className="flex flex-wrap gap-2">
            {available.map((item) => <Button key={item.provider_id} type="button" onClick={() => addProvider(item)}>添加 {item.display_name || item.provider_id}</Button>)}
          </div>
        </div>
        {fieldArray.fields.length === 0 ? <p className="m-0 text-sm text-muted-foreground">未添加 IP 欺诈 provider；IP 欺诈检查会返回 unsupported。</p> : null}
        {fieldArray.fields.map((field, index) => (
          <ProviderRow
            key={field.fieldId}
            control={form.control}
            register={form.register}
            provider={field}
            keyStatus={providerKeyStatus(query.data?.settings, field.kind)}
            label={catalog.find((item) => item.kind === field.kind)?.display_name || field.id}
            supportsAnonymous={catalog.find((item) => item.kind === field.kind)?.supports_anonymous ?? true}
            supportsApiKey={catalog.find((item) => item.kind === field.kind)?.supports_api_key ?? true}
            onRemove={() => fieldArray.remove(index)}
            index={index}
          />
        ))}
      </section>

      <div className="flex items-center justify-between gap-3">
        <p className="m-0 text-xs text-muted-foreground">API Key 留空会保留现有密钥；删除 provider 行会移除该 provider 配置。</p>
        <Button disabled={mutation.isPending || query.isLoading} type="submit">保存配置</Button>
      </div>
    </form>
  );
}

type ProviderRowProps = {
  control: Control<RuntimeSettingsForm>;
  register: UseFormRegister<RuntimeSettingsForm>;
  provider: ProviderForm & { fieldId: string };
  keyStatus: string;
  label: string;
  supportsAnonymous: boolean;
  supportsApiKey: boolean;
  onRemove: () => void;
  index: number;
};

function ProviderRow({ control, register, provider, keyStatus, label, supportsAnonymous, supportsApiKey, onRemove, index }: ProviderRowProps) {
  return (
    <div className="grid gap-3 rounded-md border border-[var(--border-soft)] p-3">
      <input type="hidden" {...register(`providers.${index}.id` as const)} />
      <input type="hidden" {...register(`providers.${index}.kind` as const)} />
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h4 className="m-0 text-sm font-semibold">{label}</h4>
          <p className="m-0 text-xs text-muted-foreground">当前密钥：{keyStatus}</p>
        </div>
        <Button type="button" onClick={onRemove}>删除</Button>
      </div>
      <div className={supportsApiKey ? 'grid gap-3 md:grid-cols-[180px_120px_1fr]' : 'grid gap-3 md:grid-cols-[180px_120px]'}>
        <DashboardField label="模式">
          <Controller control={control} name={`providers.${index}.mode` as const} render={({ field }) => <ModeSelect supportsAnonymous={supportsAnonymous} supportsApiKey={supportsApiKey} value={field.value} onChange={field.onChange} />} />
        </DashboardField>
        <DashboardField label="权重"><Input min={1} type="number" {...register(`providers.${index}.weight` as const, { valueAsNumber: true })} /></DashboardField>
        {supportsApiKey ? <DashboardField label="API Keys"><Textarea placeholder="每行一个 key；留空保留已配置密钥" rows={3} {...register(`providers.${index}.keys` as const)} /></DashboardField> : null}
      </div>
    </div>
  );
}

function ModeSelect({ supportsAnonymous, supportsApiKey, value, onChange }: { supportsAnonymous: boolean; supportsApiKey: boolean; value: ProviderMode; onChange: (value: ProviderMode) => void }) {
  return (
    <Select value={value} onValueChange={(next) => onChange(next as ProviderMode)}>
      <SelectTrigger><SelectValue /></SelectTrigger>
      <SelectContent>
        {supportsApiKey ? <SelectItem value="api_keys">API Keys</SelectItem> : null}
        {supportsAnonymous ? <SelectItem value="anonymous">匿名</SelectItem> : null}
      </SelectContent>
    </Select>
  );
}

function providerKeyStatus(settings: { ip_fraud_providers?: Array<{ kind: string; anonymous: boolean; api_key_configured: boolean; api_key_count: number }> } | undefined, kind: string) {
  const provider = settings?.ip_fraud_providers?.find((item) => item.kind === kind);
  if (!provider) return '未配置';
  if (provider.anonymous) return '匿名调用';
  return provider.api_key_configured ? `已配置 ${provider.api_key_count} 个` : '未配置';
}
