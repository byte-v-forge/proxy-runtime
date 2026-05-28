import { ChevronDown, Plus, Trash2 } from 'lucide-react';
import { type MouseEvent, useState } from 'react';
import {
  Button,
  DashboardField,
  Input
} from '@byte-v-forge/common-ui';
import type { Control } from '@byte-v-forge/common-ui';
import type { UseFormRegister } from 'react-hook-form';
import { useFieldArray, useWatch } from 'react-hook-form';
import type { ProxyProviderDescriptor } from '@byte-v-forge/common-ui/proto/byte/v/forge/contracts/proxyruntime/v1/proxy_runtime';
import type { DynamicProviderForm, RuntimeSettingsForm } from './settings-model';

type Props = {
  control: Control<RuntimeSettingsForm>;
  register: UseFormRegister<RuntimeSettingsForm>;
  providers: ProxyProviderDescriptor[];
};

export function DynamicGatewaySettings({ control, register, providers }: Props) {
  const fields = useFieldArray({ control, name: 'dynamicProviders', keyName: 'fieldId' });
  const values = useWatch({ control, name: 'dynamicProviders' }) || [];
  const catalog = providers.length ? providers : fallbackProviders;
  const available = catalog.filter((provider) => !values.some((item) => item.provider_id === provider.provider_id));
  const addProvider = (provider: ProxyProviderDescriptor) => fields.append({ provider_id: provider.provider_id, gateways: [blankGateway()] });

  return (
    <section className="grid gap-3 rounded-lg border border-[var(--border-soft)] bg-[var(--surface)] p-4">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h3 className="m-0 text-sm font-semibold">动态 IP Gateway</h3>
          <p className="m-0 text-xs text-muted-foreground">按 provider 配置代理入口和所在区域；账号只保存凭证，session 参数由 lease 请求传入。</p>
        </div>
        <div className="flex flex-wrap gap-2">
          {available.map((provider) => <Button key={provider.provider_id} type="button" onClick={() => addProvider(provider)}>添加 {provider.display_name || provider.provider_id}</Button>)}
        </div>
      </div>
      {fields.fields.length === 0 ? <p className="m-0 text-sm text-muted-foreground">未配置 gateway 的 provider 不会作为动态 IP lease 来源。</p> : null}
      {fields.fields.map((field, index) => (
        <ProviderGatewayGroup
          key={field.fieldId}
          control={control}
          displayName={providerName(catalog, field.provider_id)}
          index={index}
          onRemove={() => fields.remove(index)}
          register={register}
        />
      ))}
    </section>
  );
}

function ProviderGatewayGroup({ control, displayName, index, onRemove, register }: {
  control: Control<RuntimeSettingsForm>;
  displayName: string;
  index: number;
  onRemove: () => void;
  register: UseFormRegister<RuntimeSettingsForm>;
}) {
  const gateways = useFieldArray({ control, name: `dynamicProviders.${index}.gateways` as const, keyName: 'gatewayFieldId' });
  const gatewayValues = useWatch({ control, name: `dynamicProviders.${index}.gateways` as const }) || [];
  const [open, setOpen] = useState(false);
  const addGateway = (event: MouseEvent) => {
    event.preventDefault();
    setOpen(true);
    gateways.append(blankGateway());
  };
  const removeProvider = (event: MouseEvent) => {
    event.preventDefault();
    onRemove();
  };
  return (
    <details className="rounded-md border border-[var(--border-soft)] bg-[var(--surface)]" onToggle={(event) => setOpen(event.currentTarget.open)} open={open}>
      <input type="hidden" {...register(`dynamicProviders.${index}.provider_id` as const)} />
      <summary className="flex cursor-pointer list-none flex-wrap items-center justify-between gap-3 p-3 [&::-webkit-details-marker]:hidden">
        <div className="flex min-w-0 items-center gap-2">
          <ChevronDown className={`size-4 shrink-0 transition-transform ${open ? 'rotate-180' : ''}`} />
          <div className="min-w-0">
            <h4 className="m-0 truncate text-sm font-semibold">{displayName}</h4>
            <p className="m-0 text-xs text-muted-foreground">{gatewaySummary(gatewayValues)}</p>
          </div>
        </div>
        <div className="flex gap-2" onClick={(event) => event.stopPropagation()}>
          <Button type="button" onClick={addGateway}><Plus size={14} />Gateway</Button>
          <Button type="button" onClick={removeProvider}><Trash2 size={14} />删除 Provider</Button>
        </div>
      </summary>
      <div className="grid gap-3 border-t border-[var(--border-soft)] p-3">
        {gateways.fields.map((gateway, gatewayIndex) => (
          <div key={gateway.gatewayFieldId} className="grid gap-3 rounded border border-[var(--border-soft)] p-3 md:grid-cols-[120px_1fr_1fr_1fr_auto]">
            <DashboardField label="Gateway ID"><Input placeholder="us" {...register(`dynamicProviders.${index}.gateways.${gatewayIndex}.gateway_id` as const)} /></DashboardField>
            <DashboardField label="地址"><Input placeholder="host:port" {...register(`dynamicProviders.${index}.gateways.${gatewayIndex}.addr` as const)} /></DashboardField>
            <DashboardField label="区域"><Input placeholder="US,HK,ANY" {...register(`dynamicProviders.${index}.gateways.${gatewayIndex}.region_codes` as const)} /></DashboardField>
            <DashboardField label="显示名"><Input placeholder="可选" {...register(`dynamicProviders.${index}.gateways.${gatewayIndex}.display_name` as const)} /></DashboardField>
            <Button className="self-end" type="button" onClick={() => gateways.remove(gatewayIndex)}><Trash2 size={14} /></Button>
          </div>
        ))}
      </div>
    </details>
  );
}

function blankGateway() {
  return { gateway_id: '', display_name: '', addr: '', region_codes: '' };
}

function gatewaySummary(gateways: DynamicProviderForm['gateways']) {
  const count = gateways.length;
  const regions = gateways.flatMap((gateway) => (gateway.region_codes || '').split(',').map((value) => value.trim()).filter(Boolean)).slice(0, 6);
  const suffix = regions.length ? `；区域 ${regions.join(', ')}` : '';
  return `${count} 个 gateway${suffix}`;
}

const fallbackProviders = [
  providerFallback('1024proxy', '1024Proxy'),
  providerFallback('b2proxy', 'B2Proxy'),
  providerFallback('cliproxy', 'Cliproxy')
] as ProxyProviderDescriptor[];

function providerFallback(provider_id: string, display_name: string): ProxyProviderDescriptor {
  return { provider_id, display_name, capabilities: [], protocols: [], min_sticky_ttl: undefined, max_sticky_ttl: undefined, upstream_kinds: [], rotation_modes: [] };
}

const providerName = (providers: ProxyProviderDescriptor[], id: string) => providers.find((item) => item.provider_id === id)?.display_name || id;
