# Task 078 - Rate limit rejection metrics

## 状态

Done.

## 背景

现有 HTTP metrics 可以通过 `status="429"` 看到返回了 429 的请求，但无法区分这些 429 是 gateway rate limiter 主动拒绝，还是未来其他组件或上游 provider 映射产生。Task 071 已经让限流按 gateway client 归因，下一步需要暴露专用指标，便于观察各 client 的限流命中情况。

## 目标

- 新增 `open_ai_gateway_rate_limit_rejections_total` counter。
- 仅在 gateway 自身 rate limiter 拒绝请求时递增。
- 指标标签只包含规范化 `path` 和非敏感 `client`。
- 不记录 API key、Authorization header、token、remote address 等高敏感或高基数字段。

## 验收

- 受保护路由超限时返回 `429 rate_limit_error`，并递增 `open_ai_gateway_rate_limit_rejections_total{path,client}`。
- 普通 HTTP 429 总量指标仍保留原有语义。
- WSL `Ubuntu-24.04` `make verify` 通过。
