import {
  ProxyIPFraudProviderKind,
  ProxyProtocol,
  type ProxyDynamicIPGatewaySettings,
  type ProxyDynamicIPProviderSettings,
  type ProxyIPFraudProviderDescriptor,
  type ProxyIPFraudProviderSettings,
  type ProxyRuntimeSettings,
  type UpdateProxyRuntimeSettingsRequest
} from '@byte-v-forge/common-ui/proto/byte/v/forge/contracts/proxyruntime/v1/proxy_runtime';

export type ProviderMode = 'anonymous' | 'api_keys';
export type ProviderForm = { id: string; kind: ProxyIPFraudProviderKind; mode: ProviderMode; weight: number; keys: string };
export type DynamicGatewayForm = {
  gateway_id: string;
  display_name: string;
  addr: string;
  region_codes: string;
  protocols?: ProxyProtocol[];
  default_protocol?: ProxyProtocol;
};
export type DynamicProviderForm = { provider_id: string; gateways: DynamicGatewayForm[] };
export type RuntimeSettingsForm = { edgeEnabled: boolean; edgeUrl: string; edgeToken: string; providers: ProviderForm[]; dynamicProviders: DynamicProviderForm[] };

const ffraudKind = ProxyIPFraudProviderKind.PROXY_IP_FRAUD_PROVIDER_KIND_FFRAUD;
const ipapiKind = ProxyIPFraudProviderKind.PROXY_IP_FRAUD_PROVIDER_KIND_IPAPI;

export const fallbackProviderCatalog: ProxyIPFraudProviderDescriptor[] = [
  providerDescriptor('ffraud', 'FFraud', ffraudKind, 100),
  providerDescriptor('ipapi', 'ipapi.is', ipapiKind, 90)
];

export const defaultSettingsForm: RuntimeSettingsForm = { edgeEnabled: false, edgeUrl: '', edgeToken: '', providers: [], dynamicProviders: [] };
export const providerCatalogFrom = (providers?: ProxyIPFraudProviderDescriptor[]) => providers?.length ? providers : fallbackProviderCatalog;
export function providerDefaults(kind: ProxyIPFraudProviderKind, catalog = fallbackProviderCatalog): ProviderForm {
  const item = catalog.find((provider) => provider.kind === kind) || catalog[0];
  return { id: item.provider_id, kind: item.kind, mode: 'api_keys', weight: item.default_weight || 100, keys: '' };
}

export function formFromSettings(settings: ProxyRuntimeSettings | undefined, catalog = fallbackProviderCatalog): RuntimeSettingsForm {
  return { edgeEnabled: !!settings?.edge_canary?.enabled, edgeUrl: settings?.edge_canary?.url || '', edgeToken: '', providers: (settings?.ip_fraud_providers || []).map((provider) => ({
    id: provider.provider_id || providerDefaults(provider.kind, catalog).id,
    kind: provider.kind,
    mode: provider.anonymous ? 'anonymous' : 'api_keys',
    weight: provider.weight || providerDefaults(provider.kind, catalog).weight,
    keys: ''
  })), dynamicProviders: (settings?.dynamic_ip_providers || []).map(dynamicProviderForm) };
}

export function requestFromSettingsForm(values: RuntimeSettingsForm): UpdateProxyRuntimeSettingsRequest {
  return { edge_canary: { enabled: values.edgeEnabled, url: values.edgeUrl.trim(), token: values.edgeToken.trim(), clear_token: false }, ip_fraud_providers: values.providers.map(providerRequest), dynamic_ip_providers: values.dynamicProviders.map(dynamicProviderRequest) };
}

function providerRequest(provider: ProviderForm): ProxyIPFraudProviderSettings {
  return { provider_id: provider.id, weight: Number(provider.weight) || 100, kind: provider.kind, anonymous: provider.mode === 'anonymous', api_keys: provider.mode === 'api_keys' ? splitKeys(provider.keys) : [], clear_api_keys: provider.mode !== 'api_keys' };
}

function providerDescriptor(provider_id: string, display_name: string, kind: ProxyIPFraudProviderKind, default_weight: number): ProxyIPFraudProviderDescriptor {
  return { provider_id, display_name, kind, default_weight, supports_anonymous: true, supports_api_key: true };
}

function splitKeys(value: string) {
  return value.split(/[\n,]+/).map((item) => item.trim()).filter(Boolean);
}

function dynamicProviderForm(provider: ProxyDynamicIPProviderSettings): DynamicProviderForm {
  return { provider_id: provider.provider_id, gateways: (provider.gateways || []).map(dynamicGatewayForm) };
}

function dynamicGatewayForm(gateway: ProxyDynamicIPGatewaySettings): DynamicGatewayForm {
  return { gateway_id: gateway.gateway_id, display_name: gateway.display_name, addr: gateway.addr, region_codes: (gateway.region_codes || []).join(','), protocols: gateway.protocols || [], default_protocol: gateway.default_protocol };
}

function dynamicProviderRequest(provider: DynamicProviderForm): ProxyDynamicIPProviderSettings {
  return { provider_id: provider.provider_id, gateways: provider.gateways.map(dynamicGatewayRequest).filter((gateway) => gateway.addr) };
}

function dynamicGatewayRequest(gateway: DynamicGatewayForm): ProxyDynamicIPGatewaySettings {
  return { gateway_id: gateway.gateway_id.trim(), display_name: gateway.display_name.trim(), addr: gateway.addr.trim(), region_codes: splitRegionCodes(gateway.region_codes), protocols: gateway.protocols || [], default_protocol: gateway.default_protocol || ProxyProtocol.PROXY_PROTOCOL_UNSPECIFIED };
}

function splitRegionCodes(value: string) {
  return value.split(/[\n,，\\s]+/).map((item) => item.trim().toUpperCase()).filter(Boolean);
}
