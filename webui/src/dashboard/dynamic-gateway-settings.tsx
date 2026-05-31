import { ChevronDown, PlusCircle, Router, Trash2 } from 'lucide-react';
import { type MouseEvent, useState } from 'react';
import {
  Badge,
  Button,
  DashboardField,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
  Input
} from '@byte-v-forge/common-ui';
import type { Control } from '@byte-v-forge/common-ui';
import type { UseFormRegister } from 'react-hook-form';
import { useFieldArray, useWatch } from 'react-hook-form';
import type { ProxyProviderDescriptor } from '@byte-v-forge/common-ui/proto/byte/v/forge/contracts/proxyruntime/v1/proxy_runtime';
import type { DynamicProviderForm, RuntimeSettingsForm } from './settings-model';

type Props = { control: Control<RuntimeSettingsForm>; register: UseFormRegister<RuntimeSettingsForm>; providers: ProxyProviderDescriptor[] };

export function DynamicGatewaySettings({ control, register, providers }: Props) {
  const fields = useFieldArray({ control, name: 'dynamicProviders', keyName: 'fieldId' });
  const values = useWatch({ control, name: 'dynamicProviders' }) || [];
  const catalog = providers.length ? providers : fallbackProviders;
  const available = catalog.filter((provider) => !values.some((item) => item.provider_id === provider.provider_id));
  const gatewayCount = values.reduce((sum, item) => sum + (item.gateways?.length || 0), 0);
  const addProvider = (provider: ProxyProviderDescriptor) => fields.append({ provider_id: provider.provider_id, gateways: [blankGateway()] });

  return (
    <section className="grid gap-4 rounded-xl border border-[var(--border-soft)] bg-[var(--surface)] p-4 shadow-sm">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="flex min-w-0 items-start gap-3">
          <span className="grid size-10 shrink-0 place-items-center rounded-xl bg-primary/10 text-primary"><Router className="size-5" /></span>
          <div><h3 className="m-0 text-sm font-semibold">动态 IP Gateway</h3><p className="m-0 text-xs text-muted-foreground">按 Provider 配置代理入口和所在区域；session 参数由 lease 请求传入。</p></div>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <Badge variant="outline">{fields.fields.length} 个 Provider</Badge><Badge variant="outline">{gatewayCount} 个 Gateway</Badge>
          <AddGatewayProvider providers={available} onAdd={addProvider} />
        </div>
      </div>
      {fields.fields.length === 0 ? <div className="rounded-lg border border-dashed border-[var(--border-soft)] p-4 text-sm text-muted-foreground">未配置 gateway 的 provider 不会作为动态 IP lease 来源。</div> : null}
      <div className="grid gap-3 lg:grid-cols-2 2xl:grid-cols-3">
        {fields.fields.map((field, index) => (
          <ProviderGatewayGroup key={field.fieldId} control={control} displayName={providerName(catalog, field.provider_id)} index={index} onRemove={() => fields.remove(index)} register={register} />
        ))}
      </div>
    </section>
  );
}

function AddGatewayProvider({ providers, onAdd }: { providers: ProxyProviderDescriptor[]; onAdd: (provider: ProxyProviderDescriptor) => void }) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild><Button type="button" disabled={providers.length === 0}><PlusCircle />添加 Provider</Button></DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-52">
        {providers.map((provider) => <DropdownMenuItem key={provider.provider_id} onSelect={() => onAdd(provider)}>{provider.display_name || provider.provider_id}</DropdownMenuItem>)}
      </DropdownMenuContent>
    </DropdownMenu>
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
  const addGateway = (event: MouseEvent) => { event.preventDefault(); setOpen(true); gateways.append(blankGateway()); };
  const removeProvider = (event: MouseEvent) => { event.preventDefault(); onRemove(); };
  return (
    <details className="overflow-hidden rounded-xl border border-[var(--border-soft)] bg-[var(--surface-soft)] shadow-sm" onToggle={(event) => setOpen(event.currentTarget.open)} open={open}>
      <input type="hidden" {...register(`dynamicProviders.${index}.provider_id` as const)} />
      <summary className="flex cursor-pointer list-none flex-wrap items-center justify-between gap-3 p-3 [&::-webkit-details-marker]:hidden">
        <div className="flex min-w-0 items-center gap-3">
          <ChevronDown className={`size-4 shrink-0 transition-transform ${open ? 'rotate-180' : ''}`} />
          <div className="min-w-0"><h4 className="m-0 truncate text-sm font-semibold">{displayName}</h4><GatewaySummary gateways={gatewayValues} /></div>
        </div>
        <div className="flex gap-2" onClick={(event) => event.stopPropagation()}>
          <Button variant="outline" size="sm" type="button" onClick={addGateway}><PlusCircle />Gateway</Button>
          <Button variant="destructive" size="sm" type="button" onClick={removeProvider}><Trash2 />删除</Button>
        </div>
      </summary>
      <div className="grid gap-3 border-t border-[var(--border-soft)] p-3">
        {gateways.fields.map((gateway, gatewayIndex) => (
          <div key={gateway.gatewayFieldId} className="grid gap-3 rounded-lg border border-[var(--border-soft)] bg-[var(--surface)] p-3">
            <DashboardField label="Gateway ID"><Input placeholder="us" {...register(`dynamicProviders.${index}.gateways.${gatewayIndex}.gateway_id` as const)} /></DashboardField>
            <DashboardField label="代理地址"><Input placeholder="host:port" {...register(`dynamicProviders.${index}.gateways.${gatewayIndex}.addr` as const)} /></DashboardField>
            <DashboardField label="区域代码"><Input placeholder="US,HK,ANY" {...register(`dynamicProviders.${index}.gateways.${gatewayIndex}.region_codes` as const)} /></DashboardField>
            <DashboardField label="显示名"><Input placeholder="可选" {...register(`dynamicProviders.${index}.gateways.${gatewayIndex}.display_name` as const)} /></DashboardField>
            <Button className="justify-self-end" variant="destructive" size="sm" type="button" onClick={() => gateways.remove(gatewayIndex)}><Trash2 />删除 Gateway</Button>
          </div>
        ))}
      </div>
    </details>
  );
}

function GatewaySummary({ gateways }: { gateways: DynamicProviderForm['gateways'] }) {
  const regions = gateways.flatMap((gateway) => (gateway.region_codes || '').split(',').map((value) => value.trim()).filter(Boolean)).slice(0, 5);
  return <div className="mt-1 flex flex-wrap gap-1.5"><Badge variant="outline">{gateways.length} 个 gateway</Badge>{regions.map((region) => <Badge key={region} variant="secondary">{region}</Badge>)}</div>;
}

function blankGateway() { return { gateway_id: '', display_name: '', addr: '', region_codes: '' }; }

const fallbackProviders = [providerFallback('1024proxy', '1024Proxy'), providerFallback('b2proxy', 'B2Proxy'), providerFallback('cliproxy', 'Cliproxy')] as ProxyProviderDescriptor[];

function providerFallback(provider_id: string, display_name: string): ProxyProviderDescriptor {
  return { provider_id, display_name, capabilities: [], protocols: [], min_sticky_ttl: undefined, max_sticky_ttl: undefined, upstream_kinds: [], rotation_modes: [] };
}

const providerName = (providers: ProxyProviderDescriptor[], id: string) => providers.find((item) => item.provider_id === id)?.display_name || id;
