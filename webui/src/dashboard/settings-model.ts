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
export type RuntimeSettingsForm = { edgeEnabled: boolean; edgeUrl: string; edgeToken: string; proxyExitIpTimeoutSeconds: number; providers: ProviderForm[]; dynamicProviders: DynamicProviderForm[] };

const ipapiKind = ProxyIPFraudProviderKind.PROXY_IP_FRAUD_PROVIDER_KIND_IPAPI;
const ipinfoKind = ProxyIPFraudProviderKind.PROXY_IP_FRAUD_PROVIDER_KIND_IPINFO;
const ip2LocationKind = ProxyIPFraudProviderKind.PROXY_IP_FRAUD_PROVIDER_KIND_IP2LOCATION;
const ipAPIComKind = ProxyIPFraudProviderKind.PROXY_IP_FRAUD_PROVIDER_KIND_IP_API_COM;
const ipQualityScoreKind = ProxyIPFraudProviderKind.PROXY_IP_FRAUD_PROVIDER_KIND_IPQUALITYSCORE;
const abuseIPDBKind = ProxyIPFraudProviderKind.PROXY_IP_FRAUD_PROVIDER_KIND_ABUSEIPDB;

export const fallbackProviderCatalog: ProxyIPFraudProviderDescriptor[] = [
  providerDescriptor('ipqualityscore', 'IPQualityScore', ipQualityScoreKind, 95, false, true),
  providerDescriptor('ipapi', 'ipapi.is', ipapiKind, 100, true, true),
  providerDescriptor('ipinfo', 'IPinfo', ipinfoKind, 90, false, true),
  providerDescriptor('abuseipdb', 'AbuseIPDB', abuseIPDBKind, 85, false, true),
  providerDescriptor('ip2location', 'IP2Location.io', ip2LocationKind, 80, true, true),
  providerDescriptor('ip-api-com', 'IP-API.com', ipAPIComKind, 40, true, false)
];

export const defaultSettingsForm: RuntimeSettingsForm = { edgeEnabled: false, edgeUrl: '', edgeToken: '', proxyExitIpTimeoutSeconds: 5, providers: [], dynamicProviders: [] };
export const providerCatalogFrom = (providers?: ProxyIPFraudProviderDescriptor[]) => providers?.length ? providers : fallbackProviderCatalog;
export function providerDefaults(kind: ProxyIPFraudProviderKind, catalog = fallbackProviderCatalog): ProviderForm {
  const item = catalog.find((provider) => provider.kind === kind) || catalog[0];
  return { id: item.provider_id, kind: item.kind, mode: item.supports_api_key ? 'api_keys' : 'anonymous', weight: item.default_weight || 100, keys: '' };
}

export function formFromSettings(settings: ProxyRuntimeSettings | undefined, catalog = fallbackProviderCatalog): RuntimeSettingsForm {
  return { edgeEnabled: !!settings?.edge_canary?.enabled, edgeUrl: settings?.edge_canary?.url || '', edgeToken: '', proxyExitIpTimeoutSeconds: durationSeconds(settings?.check_settings?.proxy_exit_ip_timeout), providers: (settings?.ip_fraud_providers || []).map((provider) => ({
    id: provider.provider_id || providerDefaults(provider.kind, catalog).id,
    kind: provider.kind,
    mode: provider.anonymous ? 'anonymous' : 'api_keys',
    weight: provider.weight || providerDefaults(provider.kind, catalog).weight,
    keys: ''
  })), dynamicProviders: (settings?.dynamic_ip_providers || []).map(dynamicProviderForm) };
}

export function requestFromSettingsForm(values: RuntimeSettingsForm): UpdateProxyRuntimeSettingsRequest {
  return { edge_canary: { enabled: values.edgeEnabled, url: values.edgeUrl.trim(), token: values.edgeToken.trim(), clear_token: false }, ip_fraud_providers: values.providers.map(providerRequest), dynamic_ip_providers: values.dynamicProviders.map(dynamicProviderRequest), check_settings: { proxy_exit_ip_timeout: `${positiveSeconds(values.proxyExitIpTimeoutSeconds)}s` } };
}

function providerRequest(provider: ProviderForm): ProxyIPFraudProviderSettings {
  return { provider_id: provider.id, weight: Number(provider.weight) || 100, kind: provider.kind, anonymous: provider.mode === 'anonymous', api_keys: provider.mode === 'api_keys' ? splitKeys(provider.keys) : [], clear_api_keys: provider.mode !== 'api_keys' };
}

function providerDescriptor(provider_id: string, display_name: string, kind: ProxyIPFraudProviderKind, default_weight: number, supports_anonymous: boolean, supports_api_key: boolean): ProxyIPFraudProviderDescriptor {
  return { provider_id, display_name, kind, default_weight, supports_anonymous, supports_api_key };
}

function splitKeys(value: string) {
  return value.split(/[\n,]+/).map((item) => item.trim()).filter(Boolean);
}

function durationSeconds(value: string | undefined) {
  const match = String(value || '').match(/^(\d+(?:\.\d+)?)s$/);
  return positiveSeconds(match ? Number(match[1]) : 5);
}

function positiveSeconds(value: number) {
  return Math.max(1, Math.round(Number(value) || 5));
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
