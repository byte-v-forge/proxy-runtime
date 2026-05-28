# cloudflare-edge-canary

轻量 Cloudflare Worker canary，用于 proxy-runtime 判断当前出口是否被边缘侧 challenge、block 或 rate limit。

## 部署

```sh
npm install
npx wrangler login
npx wrangler secret put CANARY_TOKEN
npx wrangler deploy
```

默认路径为 `/edge-canary`。部署后在 proxy-runtime dashboard「配置」页写入 Worker URL 和同一个 token；proxy-runtime 不再通过环境变量读取 canary token。

如需绑定自有域名，在 `wrangler.jsonc` 增加 `routes` 或 `custom_domain` 配置后重新 `npx wrangler deploy`。

## 约束

- Worker 只返回 `{ "ok": true }`，不回传 Cloudflare、Bot、WAF 或请求原始细节。
- 如果 Cloudflare 在 Worker 前拦截，请求不会进入 Worker；proxy-runtime 会根据状态码、`cf-mitigated` 和 challenge 页面特征输出抽象风险。
