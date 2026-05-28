import { api } from '@byte-v-forge/common-ui';
import type {
  AcquireProxyLeaseRequest,
  AcquireProxyLeaseResponse,
  CheckProxyEdgeAccessRequest,
  CheckProxyEdgeAccessResponse,
  CheckProxyIPFraudRequest,
  CheckProxyIPFraudResponse,
  DeleteProxyProviderAccountRequest,
  DeleteProxyProviderAccountResponse,
  DeleteProxySourceRequest,
  DeleteProxySourceResponse,
  GetEgressGatewayResponse,
  GetProxyExitGeoRequest,
  GetProxyExitGeoResponse,
  GetProxyPoolResponse,
  GetProxyRuntimeSettingsResponse,
  ListProxyDynamicLeasesResponse,
  ListProxyIPFraudProvidersResponse,
  ListProxyProviderAccountsResponse,
  ListProxyProvidersResponse,
  ListProxySourceNodesResponse,
  ListProxySourcesResponse,
  RefreshProxyPoolResponse,
  ReleaseProxyLeaseRequest,
  ReleaseProxyLeaseResponse,
  ResolveProxyChainRequest,
  ResolveProxyChainResponse,
  UpsertProxyFixedSourceRequest,
  UpsertProxyFixedSourceResponse,
  UpsertProxyProviderAccountRequest,
  UpsertProxyProviderAccountResponse,
  UpsertProxySubscriptionSourceRequest,
  UpsertProxySubscriptionSourceResponse,
  UpdateProxyRuntimeSettingsRequest,
  UpdateProxyRuntimeSettingsResponse
} from '@byte-v-forge/common-ui/proto/byte/v/forge/contracts/proxyruntime/v1/proxy_runtime';

const base = (import.meta.env.VITE_PROXY_RUNTIME_API_BASE || '/api/proxy-runtime').replace(/\/$/, '');

export const proxyRuntimeKeys = {
  accounts: ['proxy-runtime', 'accounts'] as const,
  edgeAccess: ['proxy-runtime', 'edge-access'] as const,
  gateway: ['proxy-runtime', 'gateway'] as const,
  ipFraudCheck: ['proxy-runtime', 'ip-fraud-check'] as const,
  ipFraudProviders: ['proxy-runtime', 'ip-fraud-providers'] as const,
  leases: ['proxy-runtime', 'leases'] as const,
  pool: ['proxy-runtime', 'pool'] as const,
  providers: ['proxy-runtime', 'providers'] as const,
  settings: ['proxy-runtime', 'settings'] as const,
  sourceNodes: ['proxy-runtime', 'source-nodes'] as const,
  sources: ['proxy-runtime', 'sources'] as const
};

export const getEgressGateway = () => api<GetEgressGatewayResponse>(`${base}/gateway`);
export const getProxyPool = () => api<GetProxyPoolResponse>(`${base}/pool`);
export const listProxyProviders = () => api<ListProxyProvidersResponse>(`${base}/providers`);
export const refreshProxyPool = () => api<RefreshProxyPoolResponse>(`${base}/refresh`, { method: 'POST', body: '{}' });
export const listProviderAccounts = () => api<ListProxyProviderAccountsResponse>(`${base}/provider-accounts`);
export const upsertProviderAccount = (req: UpsertProxyProviderAccountRequest) => api<UpsertProxyProviderAccountResponse>(`${base}/provider-accounts`, { method: 'PUT', body: JSON.stringify(req) });
export const deleteProviderAccount = (req: DeleteProxyProviderAccountRequest) => api<DeleteProxyProviderAccountResponse>(`${base}/provider-accounts`, { method: 'DELETE', body: JSON.stringify(req) });
export const listProxySources = () => api<ListProxySourcesResponse>(`${base}/sources`);
export const listProxySourceNodes = (sourceId = '') => api<ListProxySourceNodesResponse>(`${base}/sources/nodes${sourceId ? `?source_id=${encodeURIComponent(sourceId)}` : ''}`);
export const upsertSubscriptionSource = (req: UpsertProxySubscriptionSourceRequest) => api<UpsertProxySubscriptionSourceResponse>(`${base}/sources`, { method: 'PUT', body: JSON.stringify(req) });
export const upsertFixedSource = (req: UpsertProxyFixedSourceRequest) => api<UpsertProxyFixedSourceResponse>(`${base}/sources/fixed`, { method: 'PUT', body: JSON.stringify(req) });
export const deleteProxySource = (req: DeleteProxySourceRequest) => api<DeleteProxySourceResponse>(`${base}/sources`, { method: 'DELETE', body: JSON.stringify(req) });
export const resolveProxyChain = (req: ResolveProxyChainRequest) => api<ResolveProxyChainResponse>(`${base}/chains/resolve`, { method: 'POST', body: JSON.stringify(req) });
export const listDynamicLeases = () => api<ListProxyDynamicLeasesResponse>(`${base}/leases`);
export const acquireProxyLease = (req: AcquireProxyLeaseRequest) => api<AcquireProxyLeaseResponse>(`${base}/leases/acquire`, { method: 'POST', body: JSON.stringify(req) });
export const releaseProxyLease = (req: ReleaseProxyLeaseRequest) => api<ReleaseProxyLeaseResponse>(`${base}/leases/release`, { method: 'POST', body: JSON.stringify(req) });
export const getProxyExitGeo = (req: GetProxyExitGeoRequest) => api<GetProxyExitGeoResponse>(`${base}/proxy_exit_geo`, { method: 'POST', body: JSON.stringify(req) });
export const checkProxyIPFraud = (req: CheckProxyIPFraudRequest) => api<CheckProxyIPFraudResponse>(`${base}/ip_fraud_check`, { method: 'POST', body: JSON.stringify(req) });
export const checkProxyEdgeAccess = (req: CheckProxyEdgeAccessRequest) => api<CheckProxyEdgeAccessResponse>(`${base}/check_cf_access_risk`, { method: 'POST', body: JSON.stringify(req) });
export const getProxyRuntimeSettings = () => api<GetProxyRuntimeSettingsResponse>(`${base}/settings`);
export const listIPFraudProviders = () => api<ListProxyIPFraudProvidersResponse>(`${base}/settings/ip-fraud-providers`);
export const updateProxyRuntimeSettings = (req: UpdateProxyRuntimeSettingsRequest) => api<UpdateProxyRuntimeSettingsResponse>(`${base}/settings`, { method: 'PUT', body: JSON.stringify(req) });
