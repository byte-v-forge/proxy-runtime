# Local Egress 10810

本地调试可启动两个 SOCKS5 出口：

- `127.0.0.1:10810`：通用出口，直连外部目标。
- `127.0.0.1:10811`：动态 IP 出口，数据面路径为 `10811 -> 10810 -> 1024Proxy -> target`。

如果代理商限制来源 IP，`10810` 直连可能仍会被拒绝；这时需要把 `10810` 的出口 IP 加入代理商白名单，或给 runtime 配置 `PROXY_RUNTIME_STATIC_CHAIN`，让动态出口路径变成 `10811 -> 10810 -> upstream proxy -> 1024Proxy -> target`。

`10810` 和 `10811` 只是本地验证用的初始 listener。生产环境使用 `PROXY_RUNTIME_LISTENERS_JSON` 或 `/proxy/listeners` 动态声明 listener；Kubernetes Service 不应枚举所有可能的数据面端口。

## 启动

```sh
export TEN24_PROXY_ADDR='us.1024proxy.io:3000'
export TEN24_PROXY_USERNAME='1024proxy username'
export TEN24_PROXY_PASSWORD='1024proxy password'
sh scripts/run-local-egress-10810.sh
```

可选覆盖：

```sh
export COMMON_EGRESS_ADDR=':10810'
export DYNAMIC_EGRESS_ADDR=':10811'
export COMMON_EGRESS_UPSTREAM='127.0.0.1:10810'
export GOST_PATH='gost'
```

## 验证

通用出口：

```sh
curl --socks5-hostname 127.0.0.1:10810 https://ipinfo.io
```

经过 `10810` 的动态 IP 出口：

```sh
curl --socks5-hostname 127.0.0.1:10811 https://ipinfo.io
```

查看实际 GOST 配置：

```sh
sh scripts/run-local-egress-10810.sh --print
```
