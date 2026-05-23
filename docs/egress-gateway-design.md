# Egress Gateway Design

`proxy-runtime` 按统一出口网关建模，不按单个代理商建模。

## 设计依据

- GOST v3 的配置核心是 `service -> handler/listener -> chain -> hop -> node`，并通过 selector 在 hop 内选择 node。对应本仓的 `EgressGateway`、`EgressRoute`、`EgressHop`、`ProxyEndpoint` 和 `ProxySelectorPolicy`。
- Envoy 的动态转发代理把 listener、cluster、DNS cache 和 upstream 发现分开，说明“动态解析/动态转发”是能力和发现方式，不是链路拓扑本身。
- Squid 的 `cache_peer` 模型把上游代理作为 peer，路由策略在 peer 之外表达。

参考链接：

- https://v3.gost.run/en/concepts/selector/
- https://www.envoyproxy.io/docs/envoy/latest/configuration/http/http_filters/dynamic_forward_proxy_filter
- https://www.squid-cache.org/Doc/config/cache_peer/

## 核心边界

- `EgressGateway`：业务服务看到的统一本地出口。
- `EgressRoute`：一条可执行路线，分 data plane route 和 control plane route。
- `EgressHop`：路线中的一跳；hop 是拓扑位置，不是代理类型。
- `ProxyEndpoint`：真正的代理上游；上游类型通过 `upstream_kind` 表达。
- `ProxyProviderDescriptor`：代理商 capability registry。
- `ProxySession`：动态 IP/sticky IP 的会话身份。

## 上游类型

- `SIMPLE_PROXY`：固定 HTTP/SOCKS5 代理，通常不主动轮换。
- `DYNAMIC_IP`：同一个 provider 接入点通过 session、用户名参数、token 或 API 控制出口 IP。
- `PROXY_POOL`：provider API 返回多个可选代理节点，由 selector 选择。

`CHAIN` 不作为上游类型。链式代理是 route/hop 拓扑；链路里的任意 hop 都可以是 simple proxy、dynamic IP 或 proxy pool。

## 控制面与数据面

有些代理商 API 需要先通过代理才能访问。这个路径属于 control plane route，只影响取号、刷新池、创建 session 等 provider 操作。

业务实际流量走 data plane route，由 GOST chain/hop/node 执行。两条路由分开，避免把“如何访问代理商 API”和“业务流量从哪里出网”混成同一条链路。
