# proxy-runtime

`proxy-runtime` 是统一出口网关基础设施仓，负责代理路由、代理池刷新、订阅源代理面、动态 IP sticky session、GOST 路由面和 Mihomo 源代理面包装。

## 职责

- 生成 GOST v3 JSON 配置并管理 GOST 路由面子进程。
- 生成 Mihomo subscription provider 配置并管理 Mihomo 源代理面子进程；订阅源配置变更通过 external-controller 热重载。
- 暴露本地 HTTP 或 SOCKS5 代理入口。
- 支持简单代理、订阅代理池、动态 IP、代理池和多 hop 出口路由组合。
- 对接 1024Proxy 动态住宅代理的用户名参数模式和 API 取号模式。
- 支持账号级动态 IP sticky lease，通过 GOST Web API 增量更新对应 service/chain。
- 区分数据面出口路由和 provider 控制面访问路由，支持代理商 API 需要先走代理的场景。
- 提供健康检查、provider capability、出口网关和代理池观测接口。

## 模型

本仓按业界代理网关模型拆分：

- `EgressGateway`：统一出口网关，对业务服务暴露本地代理监听地址。
- `EgressListener`：数据面监听入口。监听端口不是固定能力，运行时可以通过配置或 API 增加 direct/provider route listener。
- `EgressRoute`：一条出口路线，分为 route plane route 和 control plane route。
- `EgressHop`：路由中的一跳，带 selector；一个 hop 可以有多个 endpoint。
- `ProxyEndpoint`：实际上游节点，`upstream_kind` 可为 simple proxy、dynamic IP 或 proxy pool。
- `ProxySession`：动态 IP sticky session；主动新建 session 会生成新的 provider session id。
- `ProxyProviderDescriptor`：代理商能力声明，业务侧根据 capability 使用，不硬编码代理商分支。

设计参考：GOST 的 service/chain/hop/node/selector 模型、Mihomo 的 proxy-provider/proxy-group 模型、Envoy 的 listener/cluster/dynamic forward proxy、Squid 的 upstream peer 思路。chain 只表达拓扑，不作为 endpoint 类型。

详细设计见 `docs/egress-gateway-design.md`。

## 当前实现

- Go module：`github.com/byte-v-forge/proxy-runtime`
- 服务入口：`cmd/proxy-runtime`
- 配置加载：`internal/config`
- GOST 配置与进程管理：`internal/gost`
- 源代理面抽象与 Mihomo adapter：`internal/sourceplane`
- provider 抽象和代理格式解析：`internal/provider`
- 1024Proxy adapter：`internal/provider/ten24`
- Dashboard 前端模块：`webui/src/dashboard`
- 公共契约源头：`common-lib/proto/byte/v/forge/contracts/proxyruntime/v1/proxy_runtime.proto`

## 运行方式

用户名密码模式会根据 1024Proxy 规则拼接用户名参数，并作为 GOST 最后一跳代理池节点：

```sh
PROXY_RUNTIME_1024_PROXY_ADDR=us.1024proxy.io:3000 \
PROXY_RUNTIME_1024_USERNAME='USERNAME' \
PROXY_RUNTIME_1024_PASSWORD='PASSWORD' \
PROXY_RUNTIME_LOCAL_ADDR=':1080' \
PROXY_RUNTIME_LOCAL_PROTOCOL=http \
proxy-runtime
```

本地双出口调试见 `docs/local-egress-10810.md`：`10810` 为通用 SOCKS5 出口，`10811` 为经过 `10810` 的 1024Proxy 动态 IP 出口。

生产形态不应在代码或 Helm chart 中枚举所有可能的数据面端口；使用 `PROXY_RUNTIME_LISTENERS_JSON` 声明初始 listener，或调用 listener API 动态创建。Kubernetes `Service` 只适合暴露固定控制面和少量固定入口；需要任意动态端口对外可达时，应使用 `hostNetwork`/专用网关层承载数据面。

API 模式使用 1024Proxy 控制台复制出的 API 链接作为基础地址，可通过环境变量覆盖查询参数：

```sh
PROXY_RUNTIME_1024_API_URL='https://example.invalid/api-link-from-console' \
PROXY_RUNTIME_1024_API_REGION=1 \
PROXY_RUNTIME_1024_API_NUM=20 \
PROXY_RUNTIME_1024_API_TYPE=json \
PROXY_RUNTIME_REFRESH_SECONDS=300 \
proxy-runtime
```

静态链路通过逗号分隔，按顺序生成 GOST chain hops；provider 代理池或动态 IP 作为 exit hop：

```sh
PROXY_RUNTIME_STATIC_CHAIN='socks5://user:pass@jump-a:1080,http://user:pass@jump-b:8080' \
PROXY_RUNTIME_1024_PROXY_ADDR=us.1024proxy.io:3000 \
PROXY_RUNTIME_1024_USERNAME='USERNAME' \
PROXY_RUNTIME_1024_PASSWORD='PASSWORD' \
proxy-runtime
```

当 `PROXY_RUNTIME_COMMON_EGRESS_ADDR=:10810` 且设置了 `PROXY_RUNTIME_STATIC_CHAIN` 时，动态出口的数据面路径为 `10811 -> 10810 -> static chain -> provider -> target`。这用于处理代理商要求来源 IP 加白、或访问代理商入口本身也必须先走代理的场景；如果上游 URL 含用户名密码，应按 secret 管理。

动态 listener catalog：

```sh
PROXY_RUNTIME_LISTENERS_JSON='[
  {"id":"direct-egress","addr":":10810","protocol":"socks5","route":"upstream","upstream":"tcp://192.168.122.1:10810"},
  {"id":"dynamic-provider","addr":":10811","protocol":"socks5","route":"provider"},
  {"id":"checkout-egress","addr":":10812","protocol":"socks5","route":"upstream","upstream":"tcp://192.168.122.1:10813"}
]' \
proxy-runtime
```

简单代理模式：

```sh
PROXY_RUNTIME_PROVIDER=static \
PROXY_RUNTIME_SIMPLE_PROXIES='http://user:pass@proxy-a:8080,socks5://user:pass@proxy-b:1080' \
proxy-runtime
```

代理商 API 需要通过代理访问时：

```sh
PROXY_RUNTIME_PROVIDER_HTTP_PROXY='http://user:pass@control-plane-proxy:8080' \
PROXY_RUNTIME_1024_API_URL='https://example.invalid/api-link-from-console' \
proxy-runtime
```

为业务账号分配新的动态 IP lease：

```sh
curl -X POST http://127.0.0.1:8080/proxy/leases/acquire \
  -H 'Content-Type: application/json' \
  -d '{"account_id":"account-1","purpose":"register","force_new":true,"policy":{"mode":"PROXY_SESSION_MODE_STICKY","region":"US","sticky_ttl":"1800s"}}'
```

## 配置

- `PROXY_RUNTIME_ADDR`：本服务 HTTP 地址，默认 `:8080`。
- `PROXY_RUNTIME_POSTGRES_DSN`：proxy-runtime 控制面 PG DSN。
- `PROXY_RUNTIME_ENCRYPTION_KEY`：前端配置的动态代理上游账号凭据加密密钥。
- `PROXY_RUNTIME_ROUTE_RUNTIME`：路由面 runtime，当前只支持 `gost`，默认 `gost`。
- `PROXY_RUNTIME_SOURCE_RUNTIME`：源代理面 runtime，支持 `mihomo`、`none`，默认 `mihomo`。
- `PROXY_RUNTIME_GOST_PATH`：GOST 可执行文件路径，默认 `gost`。
- `PROXY_RUNTIME_GOST_CONFIG_DIR`：生成的 GOST 配置目录，默认系统临时目录。
- `PROXY_RUNTIME_GOST_API_ADDR`：传给 GOST `-api` 的地址，默认 `127.0.0.1:18080`；动态 lease 使用该 API 增量更新，不 reload 基础配置。
- `PROXY_RUNTIME_GOST_METRICS_ADDR`：传给 GOST `-metrics` 的地址，默认关闭。
- `PROXY_RUNTIME_MIHOMO_PATH`：Mihomo 可执行文件路径，默认 `mihomo`。
- `PROXY_RUNTIME_MIHOMO_CONFIG_DIR`：Mihomo 配置和订阅 source 存储目录，默认 `/var/lib/byte-v-forge/proxy-runtime/mihomo`。
- `PROXY_RUNTIME_MIHOMO_MIXED_ADDR`：Mihomo mixed 本地入口，默认 `127.0.0.1:18900`，仅供 GOST 链接订阅源代理池。
- `PROXY_RUNTIME_MIHOMO_API_ADDR`：Mihomo external-controller 地址，默认 `127.0.0.1:18901`，用于配置热重载。
- `PROXY_RUNTIME_COMMON_EGRESS_ADDR`：通用出口监听地址，例如 `:10810`；设置后动态出口会先经过该通用出口。
- `PROXY_RUNTIME_DYNAMIC_EGRESS_ADDR`：动态出口监听地址，例如 `:10811`；未设置时回退到 `PROXY_RUNTIME_LOCAL_ADDR`。
- `PROXY_RUNTIME_LOCAL_ADDR`：兼容旧配置的本地代理监听地址，默认 `:1080`。
- `PROXY_RUNTIME_LOCAL_PROTOCOL`：本地代理协议，支持 `http`、`socks5`，默认 `http`。
- `PROXY_RUNTIME_LOCAL_USERNAME` / `PROXY_RUNTIME_LOCAL_PASSWORD`：本地代理认证，默认关闭。
- `PROXY_RUNTIME_LISTENERS_JSON`：数据面 listener catalog。每个 listener 支持 `id`、`addr`、`protocol`、`route`、`upstream`、`labels`；`route=direct` 表示直连，`route=provider` 表示走静态链路和 provider 出口，`route=upstream` 表示转发到该 listener 自己的固定 `upstream`。
- `PROXY_RUNTIME_STATIC_CHAIN`：静态 forward hops，逗号分隔，支持 `http://`、`https://`、`socks5://`。
- `PROXY_RUNTIME_SIMPLE_PROXIES`：简单代理池，`PROXY_RUNTIME_PROVIDER=static` 时必填。
- `PROXY_RUNTIME_PROVIDER_HTTP_PROXY`：provider 控制面 HTTP client 使用的代理，例如拉取代理池 API 时先走该代理。
- `PROXY_RUNTIME_PROVIDER`：provider 名称，支持 `1024proxy`、`static`、`none`，默认 `1024proxy`。
- `PROXY_RUNTIME_REFRESH_SECONDS`：代理池刷新周期，默认 `300`。
- `PROXY_RUNTIME_REQUEST_TIMEOUT_SECONDS`：provider HTTP 请求超时，默认 `10`。
- `PROXY_RUNTIME_PROXY_EXIT_GEO_URLS`：代理出口 IP 探测端点列表，默认使用多个公开出口 IP 探测服务并取最快成功结果。
- `PROXY_RUNTIME_IP_FRAUD_CACHE_TTL_SECONDS` / `PROXY_RUNTIME_IP_FRAUD_KEY_COOLDOWN_SECONDS`：IP 欺诈结果缓存和额度耗尽 key 冷却时间。

IP 欺诈 provider、Cloudflare canary 与代理出口 IP 探测超时通过 dashboard「配置」页或 `GET/PUT /proxy/settings` 管理，持久化到 proxy-runtime 控制面 PG；查询接口只返回密钥是否已配置。IP 欺诈 provider 采用添加式配置，只暴露 provider、API Key/匿名模式和权重；provider URL 属于 adapter 内部实现细节，不进入公共配置。Cloudflare canary 使用显式 `enabled` 开关；关闭只停用检测，不删除已保存 URL/token。代理出口 IP 探测超时默认 `5s`。未启用 canary、未配置 canary token 或未配置 IP 欺诈 provider 时，对应检查返回 `unsupported`。

`PUT /proxy/settings` 示例：

```json
{
  "edge_canary": {
    "enabled": true,
    "url": "https://byte-v-forge-edge-canary.example.workers.dev/edge-canary",
    "token": "change-me"
  },
  "ip_fraud_providers": [
    {"provider_id":"ipqualityscore","weight":95,"kind":"PROXY_IP_FRAUD_PROVIDER_KIND_IPQUALITYSCORE","api_keys":["key-a","key-b"]},
    {"provider_id":"ipapi","weight":100,"kind":"PROXY_IP_FRAUD_PROVIDER_KIND_IPAPI","api_keys":["key-a","key-b"]},
    {"provider_id":"abuseipdb","weight":85,"kind":"PROXY_IP_FRAUD_PROVIDER_KIND_ABUSEIPDB","api_keys":["key-a","key-b"]},
    {"provider_id":"ip2location","weight":80,"kind":"PROXY_IP_FRAUD_PROVIDER_KIND_IP2LOCATION","anonymous":true},
    {"provider_id":"ip-api-com","weight":40,"kind":"PROXY_IP_FRAUD_PROVIDER_KIND_IP_API_COM","anonymous":true}
  ],
  "check_settings": {
    "proxy_exit_ip_timeout": "5s"
  }
}
```

动态代理账号只保存 provider、显示名和账号凭证。代理入口、协议默认值和支持的 session 参数由 provider plugin 根据官方文档声明；粘性时长、国家、地区、ASN 等都属于 lease/session policy，不属于账号配置。B2Proxy 官方账密模式需要先在 dashboard 选择 hostname，API 提取模式需要生成 API URL，当前不放入账号式动态 lease，后续应作为 API extraction source 单独建模。

1024Proxy 配置：

- `PROXY_RUNTIME_1024_PROXY_ADDR`：用户名密码模式的代理地址，例如 `us.1024proxy.io:3000`。
- `PROXY_RUNTIME_1024_USERNAME` / `PROXY_RUNTIME_1024_PASSWORD`：1024Proxy 子账号认证。
- `PROXY_RUNTIME_1024_PROTOCOL`：provider 出口协议，支持 `http`、`socks5`，默认 `http`。
- `PROXY_RUNTIME_1024_SESSION_ID` / `PROXY_RUNTIME_1024_STICKY_MINUTES`：仅用于可选启动期 bootstrap；业务动态 lease 的地区、ASN 和 sticky 参数由调用方请求显式传入。
- `PROXY_RUNTIME_1024_API_URL`：API 模式链接，来自 1024Proxy 控制台。
- `PROXY_RUNTIME_1024_API_REGION` / `PROXY_RUNTIME_1024_API_FORMAT` / `PROXY_RUNTIME_1024_API_TIME` / `PROXY_RUNTIME_1024_API_NUM` / `PROXY_RUNTIME_1024_API_TYPE`：API 查询参数覆盖。

## HTTP 端点

- `GET /healthz`：进程存活检查。
- `GET /readyz`：GOST 进程可用检查。
- `GET /proxy/providers`：返回当前 provider capability descriptor。
- `GET /proxy/gateway`：返回统一出口网关、route plane route、source runtime 状态和 control plane route。
- `GET /proxy/pool`：返回 proto JSON 格式的当前代理池快照。
- `POST /proxy/refresh`：刷新 provider 池、订阅源代理面并 reload GOST 基础路由。
- `GET /proxy/provider-accounts` / `PUT /proxy/provider-accounts` / `DELETE /proxy/provider-accounts`：管理上游代理账号。
- `GET /proxy/sources` / `PUT /proxy/sources` / `DELETE /proxy/sources`：管理固定、订阅、动态 IP source 元数据。
- `GET /proxy/leases`：查看账号级动态 IP lease。
- `POST /proxy/leases/acquire`：为业务账号创建或替换动态 IP lease；只增量更新该账号对应的 GOST service/chain。
- `POST /proxy/leases/release`：释放业务账号动态 IP lease。
- `POST /proxy/proxy_exit_ip`：通过指定 listener 访问固定探测端点，返回当前代理出口 IP。
- `POST /proxy/proxy_exit_geo`：输入 `ip`，通过固定 IP 信息 API 查询国家、地区和城市；不经过代理出口。
- `POST /proxy/ip_fraud_check`：输入 `ip`，返回抽象后的欺诈风险枚举结果；不经过代理出口。
- `POST /proxy/check_cf_access_risk`：通过同一出口访问 canary，返回抽象后的边缘访问风险。
- `POST /proxy/target_connectivity_check`：通过同一出口访问 workflow 传入的目标 URL；只判断 DNS/TCP/TLS/HTTP 是否打通，任意 HTTP 响应（包括 401/403/404/5xx）都算 reachable，风控/拦截由 CF canary 与 IP fraud 单独判断。ChatGPT/OpenAI 默认用 `https://chatgpt.com/api/auth/session`、`https://auth.openai.com/`、`https://api.openai.com/v1/models` 这类轻量端点。
- `GET /proxy/settings` / `PUT /proxy/settings`：读取或更新运行时检查配置；响应不回显 token/API key。

以上代理 runtime API 同时暴露 `/api/proxy-runtime/*` 前缀，供 dashboard 模块通过同源路径访问。

## 生成

公开 proto、Go gRPC 契约和前端 TypeScript 类型都由 `common-lib` 生成；本仓前端通过 `@byte-v-forge/common-ui/proto/...` 消费公共模型。

## Cloudflare canary Worker

Worker 位于 `workers/cloudflare-edge-canary`，使用 Wrangler 部署：

```sh
cd workers/cloudflare-edge-canary
npm install
npx wrangler secret put CANARY_TOKEN
npx wrangler deploy
```

部署后把 Worker URL 和同一 token 配置给 proxy-runtime。

## 验证

```sh
go vet ./...
```

本聚合项目禁止在 Mac 本机进行业务构建和镜像构建；需要产物或部署验证时在远程宿主机环境执行。
