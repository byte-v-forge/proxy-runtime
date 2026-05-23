import { RotateCcw } from 'lucide-react';
import type React from 'react';
import {
  Button,
  Controller,
  Input,
  Label,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
  useForm
} from '@/dashboard/module-kit';
import {
  ProxyRotationMode,
  ProxySessionMode,
  ProxyUpstreamKind,
  type CreateProxySessionRequest,
  type ProxyProviderDescriptor,
  type ProxySession
} from './proto/byte/v/forge/contracts/proxyruntime/v1/proxy_runtime';
import { enumLabel, formatTime } from './labels';

type SessionForm = {
  region: string;
  state: string;
  city: string;
  asn: string;
  ttl: number;
};

export function SessionPanel({ provider, activeSession, busy, onCreate }: {
  provider?: ProxyProviderDescriptor;
  activeSession?: ProxySession;
  busy?: boolean;
  onCreate: (req: CreateProxySessionRequest) => void;
}) {
  const form = useForm<SessionForm>({
    defaultValues: { region: '', state: '', city: '', asn: '', ttl: provider?.min_sticky_ttl_minutes || 30 }
  });
  const disabled = busy || !provider?.supports_active_new_session;
  return (
    <div className="proxySessionPane">
      <section className="proxySessionCurrent">
        <h3>当前Sticky会话</h3>
        <dl>
          <div><dt>Session ID</dt><dd className="proxyMono">{activeSession?.session_id || '-'}</dd></div>
          <div><dt>模式</dt><dd>{enumLabel(activeSession?.policy?.mode)}</dd></div>
          <div><dt>创建</dt><dd>{formatTime(activeSession?.created_at)}</dd></div>
          <div><dt>过期</dt><dd>{formatTime(activeSession?.expires_at)}</dd></div>
        </dl>
      </section>
      <form className="proxySessionForm" onSubmit={form.handleSubmit((values: SessionForm) => onCreate(requestFromForm(provider, values)))}>
        <div className="proxyFormGrid">
          <Field label="国家/地区"><Input placeholder="US" {...form.register('region')} /></Field>
          <Field label="州/省"><Input placeholder="Louisiana" {...form.register('state')} /></Field>
          <Field label="城市"><Input placeholder="New Orleans" {...form.register('city')} /></Field>
          <Field label="ASN"><Input placeholder="AS12345" {...form.register('asn')} /></Field>
          <Field label="TTL">
            <Controller control={form.control} name="ttl" render={({ field }: { field: { value: number; onChange: (value: number) => void } }) => (
              <Select value={String(field.value)} onValueChange={(value) => field.onChange(Number(value))}>
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  {[15, 30, 60, 120].map((value) => <SelectItem key={value} value={String(value)}>{value}分钟</SelectItem>)}
                </SelectContent>
              </Select>
            )} />
          </Field>
        </div>
        <Button disabled={disabled} type="submit"><RotateCcw size={15} />新建Sticky会话</Button>
      </form>
    </div>
  );
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return <label className="proxyField"><Label>{label}</Label>{children}</label>;
}

function requestFromForm(provider: ProxyProviderDescriptor | undefined, values: SessionForm): CreateProxySessionRequest {
  return {
    pool_id: 'default',
    provider_id: provider?.provider_id || '',
    force_new: true,
    policy: {
      mode: ProxySessionMode.PROXY_SESSION_MODE_STICKY,
      region: values.region,
      state: values.state,
      city: values.city,
      asn: values.asn,
      sticky_ttl_minutes: Number(values.ttl) || 30,
      labels: {},
      upstream_kind: ProxyUpstreamKind.PROXY_UPSTREAM_KIND_DYNAMIC_IP,
      rotation_mode: ProxyRotationMode.PROXY_ROTATION_MODE_STICKY_SESSION
    }
  };
}
