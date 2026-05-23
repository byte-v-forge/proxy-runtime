# AGENTS.md

本仓是 `proxy-runtime`，只承载统一出口网关、代理上游、代理池和 GOST runtime 包装能力。

- 本仓可以提供 proxy provider adapter、代理池刷新、出口路由编排、GOST 进程生命周期管理、健康检查和运行时观测接入点。
- 本仓不得承载 GPT、邮箱、短信、支付、账号注册或任何业务流程的代理选择逻辑；业务仓只通过契约、配置或 proxy ref 使用本仓能力。
- provider adapter 只表达 provider 的协议、认证、地区、会话、代理池和取号能力，不感知业务资源类型。
- `route`/`hop` 表达链路拓扑；`endpoint.upstream_kind` 表达上游资源类型。不得把 chain 建模成资源类型，因为链路中的任意 hop 都可能是简单代理、动态 IP 或代理池。
- provider 控制面访问和数据面出口分开建模；代理商 API 需要先走代理时，用 control plane route 表达，不混入业务出口链路。
- 1024Proxy 对接只封装其动态住宅代理的 HTTP(S)/SOCKS5、用户名参数和 API 取号模式；账号、密码、API 链接和白名单属于 secret/config。
- 跨仓可复用的代理模型来自本仓 `proto/byte/v/forge/contracts/proxyruntime/v1/`。
- GOST 配置由本仓统一生成；不得在业务仓手写等价 GOST 配置结构或 provider 参数拼装逻辑。
- 日志、指标和错误信息不得输出代理密码、API 链接 token、用户名中的可复用会话材料或完整代理 URL。
- 后端优先使用 Go，按 Clean Code、DI 和面向抽象设计组织代码。
- `gen/` 承载本仓 proto 生成物；proto 变更后运行生成脚本并提交生成物。
