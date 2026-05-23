import { api } from '@/dashboard/module-kit';
import type {
  CreateProxySessionRequest,
  CreateProxySessionResponse,
  GetEgressGatewayResponse,
  GetProxyPoolResponse,
  ListProxyProvidersResponse,
  RefreshProxyPoolResponse
} from './proto/byte/v/forge/contracts/proxyruntime/v1/proxy_runtime';

const proxyRuntimeBase = (import.meta.env.VITE_PROXY_RUNTIME_API_BASE || '/api/proxy-runtime').replace(/\/$/, '');

export const proxyRuntimeKeys = {
  gateway: ['proxy-runtime', 'gateway'] as const,
  pool: ['proxy-runtime', 'pool'] as const,
  providers: ['proxy-runtime', 'providers'] as const
};

export function getEgressGateway() {
  return api<GetEgressGatewayResponse>(`${proxyRuntimeBase}/gateway`);
}

export function getProxyPool() {
  return api<GetProxyPoolResponse>(`${proxyRuntimeBase}/pool`);
}

export function listProxyProviders() {
  return api<ListProxyProvidersResponse>(`${proxyRuntimeBase}/providers`);
}

export function refreshProxyPool() {
  return api<RefreshProxyPoolResponse>(`${proxyRuntimeBase}/refresh`, { method: 'POST', body: '{}' });
}

export function createProxySession(req: CreateProxySessionRequest) {
  return api<CreateProxySessionResponse>(`${proxyRuntimeBase}/session/new`, {
    method: 'POST',
    body: JSON.stringify(req)
  });
}
