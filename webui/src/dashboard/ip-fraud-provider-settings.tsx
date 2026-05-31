import { type ReactNode } from 'react';
import { KeyRound, PlusCircle, ShieldCheck, Trash2 } from 'lucide-react';
import {
  Badge,
  Button,
  Controller,
  DashboardField,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
  Input,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
  Textarea
} from '@byte-v-forge/common-ui';
import type { Control } from '@byte-v-forge/common-ui';
import type { UseFormRegister } from 'react-hook-form';
import type { ProxyIPFraudProviderDescriptor } from '@byte-v-forge/common-ui/proto/byte/v/forge/contracts/proxyruntime/v1/proxy_runtime';
import { providerDefaults, type ProviderForm, type ProviderMode, type RuntimeSettingsForm } from './settings-model';

type SettingsView = { ip_fraud_providers?: Array<{ kind: string; anonymous: boolean; api_key_configured: boolean; api_key_count: number }> };

type Props = {
  catalog: ProxyIPFraudProviderDescriptor[];
  control: Control<RuntimeSettingsForm>;
  fields: Array<ProviderForm & { fieldId: string }>;
  providers: ProviderForm[];
  register: UseFormRegister<RuntimeSettingsForm>;
  settings?: SettingsView;
  onAdd: (provider: ProviderForm) => void;
  onRemove: (index: number) => void;
};

export function IPFraudProviderSettings({ catalog, control, fields, providers, register, settings, onAdd, onRemove }: Props) {
  const available = catalog.filter((item) => !providers.some((provider) => provider.kind === item.kind));
  return (
    <section className="grid gap-4 rounded-xl border border-[var(--border-soft)] bg-[var(--surface)] p-4 shadow-sm">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <SectionTitle icon={<ShieldCheck />} title="IP 欺诈 Provider" description="添加式配置；只暴露 Provider、API Key/匿名模式和权重，URL 由后端插件维护。" />
        <AddProviderButton catalog={catalog} providers={available} onAdd={onAdd} />
      </div>
      {fields.length === 0 ? <EmptyNotice /> : null}
      <div className="grid gap-3 lg:grid-cols-2 2xl:grid-cols-3">
        {fields.map((field, index) => {
          const descriptor = catalog.find((item) => item.kind === field.kind);
          return (
            <ProviderRow
              key={field.fieldId}
              control={control}
              index={index}
              label={descriptor?.display_name || field.id}
              provider={field}
              register={register}
              keyStatus={providerKeyStatus(settings, field.kind)}
              supportsAnonymous={descriptor?.supports_anonymous ?? true}
              supportsApiKey={descriptor?.supports_api_key ?? true}
              onRemove={() => onRemove(index)}
            />
          );
        })}
      </div>
    </section>
  );
}

function AddProviderButton({ catalog, providers, onAdd }: { catalog: ProxyIPFraudProviderDescriptor[]; providers: ProxyIPFraudProviderDescriptor[]; onAdd: (provider: ProviderForm) => void }) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button type="button" disabled={providers.length === 0}><PlusCircle />添加 Provider</Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-56">
        {providers.map((provider) => (
          <DropdownMenuItem key={provider.provider_id} onSelect={() => onAdd(providerDefaults(provider.kind, catalog))}>
            {provider.display_name || provider.provider_id}
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

function ProviderRow({ control, index, keyStatus, label, provider, register, supportsAnonymous, supportsApiKey, onRemove }: {
  control: Control<RuntimeSettingsForm>;
  index: number;
  keyStatus: ProviderKeyStatus;
  label: string;
  provider: ProviderForm & { fieldId: string };
  register: UseFormRegister<RuntimeSettingsForm>;
  supportsAnonymous: boolean;
  supportsApiKey: boolean;
  onRemove: () => void;
}) {
  return (
    <div className="grid content-start gap-3 rounded-xl border border-[var(--border-soft)] bg-[var(--surface-soft)] p-3 shadow-sm">
      <input type="hidden" {...register(`providers.${index}.id` as const)} />
      <input type="hidden" {...register(`providers.${index}.kind` as const)} />
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex min-w-0 items-center gap-3">
          <span className="grid size-9 shrink-0 place-items-center rounded-lg bg-primary/10 text-primary"><KeyRound className="size-4" /></span>
          <div className="min-w-0">
            <h4 className="m-0 truncate text-sm font-semibold">{label}</h4>
            <div className="mt-1 flex flex-wrap gap-1.5"><SecretBadge status={keyStatus} /><Badge variant="outline">权重 {provider.weight || 0}</Badge></div>
          </div>
        </div>
        <Button variant="destructive" size="sm" type="button" onClick={onRemove}><Trash2 />删除</Button>
      </div>
      <div className="grid gap-3">
        <DashboardField label="模式">
          <Controller control={control} name={`providers.${index}.mode` as const} render={({ field }) => <ModeSelect supportsAnonymous={supportsAnonymous} supportsApiKey={supportsApiKey} value={field.value} onChange={field.onChange} />} />
        </DashboardField>
        <DashboardField label="权重"><Input min={1} type="number" {...register(`providers.${index}.weight` as const, { valueAsNumber: true })} /></DashboardField>
        {supportsApiKey ? <DashboardField label="API Keys"><Textarea placeholder="每行一个 key；留空保留已配置密钥" rows={3} {...register(`providers.${index}.keys` as const)} /></DashboardField> : null}
      </div>
    </div>
  );
}

function SectionTitle({ icon, title, description }: { icon: ReactNode; title: string; description: string }) {
  return <div className="flex min-w-0 items-start gap-3"><span className="grid size-10 shrink-0 place-items-center rounded-xl bg-primary/10 text-primary [&_svg]:size-5">{icon}</span><div><h3 className="m-0 text-sm font-semibold">{title}</h3><p className="m-0 text-xs text-muted-foreground">{description}</p></div></div>;
}

function EmptyNotice() {
  return <div className="rounded-lg border border-dashed border-[var(--border-soft)] p-4 text-sm text-muted-foreground">未添加 IP 欺诈 provider；IP 欺诈检查会返回 unsupported。</div>;
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

type ProviderKeyStatus = { label: string; tone: 'secondary' | 'outline' | 'destructive' };
function providerKeyStatus(settings: SettingsView | undefined, kind: string): ProviderKeyStatus {
  const provider = settings?.ip_fraud_providers?.find((item) => item.kind === kind);
  if (!provider) return { label: '未配置', tone: 'destructive' };
  if (provider.anonymous) return { label: '匿名调用', tone: 'secondary' };
  return provider.api_key_configured ? { label: `已配置 ${provider.api_key_count} 个 key`, tone: 'secondary' } : { label: '未配置 key', tone: 'destructive' };
}

function SecretBadge({ status }: { status: ProviderKeyStatus }) {
  return <Badge variant={status.tone}>{status.label}</Badge>;
}
