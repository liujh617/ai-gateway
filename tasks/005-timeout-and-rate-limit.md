# Task 005: 超时与限流治理

## 状态

Done

## 背景

网关需要保护自身和上游 provider，避免普通请求无限挂起，也避免单个 API key 在短时间内过量请求。Task 005 增加普通请求超时、流式请求独立超时和简单 in-memory rate limiter。

## 范围

实现：

- `request_timeout_seconds` 配置。
- `stream_timeout_seconds` 配置。
- `rate_limit.requests_per_minute` 配置。
- 普通 chat completions provider 调用超时。
- stream chat completions 独立超时。
- 按 Bearer token 的简单 in-memory rate limiter。
- `/healthz` 跳过鉴权和限流。
- OpenAI-compatible `429 rate_limit_error`。
- 超时映射为 `504 server_error`。

暂不实现：

- 分布式限流。
- token bucket 精细平滑限流。
- 多租户配额。
- provider 级并发上限。
- stream 首 token 单独超时。

## 验收标准

- 普通 provider 调用超时返回 `504 server_error`。
- 超过每分钟请求数返回 `429 rate_limit_error`。
- `/healthz` 不受限流影响。
- stream 请求使用 `stream_timeout_seconds`，不被普通 request timeout 截断。
- `make verify` 和 `make smoke` 在 WSL `Ubuntu-24.04` 中通过。

