# Task 085 - Config schema API client descriptions

## 状态

Done.

## 背景

`schema/config.schema.json` 已经支持 `api_clients`、模型白名单和 per-client rate limit，但字段描述仍停留在早期的 auth 和 observability labels。使用支持 JSON Schema 的编辑器时，提示信息不能完整解释当前配置能力。

## 目标

- 更新 `api_clients` 描述，覆盖 auth、observability、模型白名单和限流覆盖。
- 为 `api_clients[].models` 增加说明。
- 为 per-client 和全局 `rate_limit` 增加更明确的说明。

## 验收

- Schema 字段描述与当前配置能力一致。
- WSL `Ubuntu-24.04` `make check-config-examples` 通过。
- WSL `Ubuntu-24.04` `make verify` 通过。
