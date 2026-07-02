# Task 071 - Gateway client rate limit overrides

## 背景

Task 070 已经引入带非敏感 `name` 的 `api_clients`，日志和 metrics 可以按 gateway client 归因。当前限流仍然只有全局 `rate_limit.requests_per_minute`，无法对不同 client 设置不同调用速率。

## 目标

- 在 `api_clients` 中支持可选 `rate_limit.requests_per_minute`。
- 未配置 client 覆盖时继承全局 `rate_limit.requests_per_minute`。
- client 覆盖显式配置为 `0` 时，关闭该 client 限流。
- 限流 key 优先使用鉴权后的 gateway client name，避免依赖或记录 Bearer token。
- 配置校验、JSON Schema 和文档同步更新。

## 验收

- 全局限流仍兼容既有 `api_key` 和 `api_keys` 配置。
- `api_clients[].rate_limit.requests_per_minute` 可覆盖单个 client。
- 负数 client 限流配置被拒绝。
- `/healthz` 等公开路由仍不参与限流。
- 超限仍返回 OpenAI-compatible `429 rate_limit_error`。

## 状态

Done.
