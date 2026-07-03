# Task 080 - Rate limit Retry-After header

## 状态

Done.

## 背景

Gateway rate limiter 当前在超限时返回 OpenAI-compatible `429 rate_limit_error`，并暴露专用 rejection metrics。客户端收到 429 后仍缺少标准重试提示，只能自行猜测等待时间。因此可以在 gateway 自身限流拒绝时返回稳定的 `Retry-After` header。具体取值已由 Task 083 精细化为当前窗口剩余秒数。

## 目标

- Gateway 自身 rate limiter 拒绝请求时设置 `Retry-After`。
- 仅影响 gateway rate limiter 产生的 `429`。
- 保持现有 JSON error body 和 metrics 行为不变。

## 验收

- 受保护路由超限时返回 `429 rate_limit_error`。
- 响应包含 `Retry-After`。
- `open_ai_gateway_rate_limit_rejections_total` 仍正常递增。
- WSL `Ubuntu-24.04` `make verify` 通过。
