# Contributing

- 保持本仓只承载通用代理基础设施能力。
- provider 差异通过 adapter、配置和契约表达，不写入业务分支。
- secret、token、代理密码和可复用会话材料不得写入日志、示例或提交历史。
- 修改 proto 后运行 `sh scripts/generate-proto.sh` 并提交生成物。
