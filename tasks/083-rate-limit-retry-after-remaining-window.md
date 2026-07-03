# Task 083 - Rate limit Retry-After remaining window

## 状态

Done.

## 背景

Task 080 为 gateway rate limiter 的 429 响应增加了 `Retry-After`，初始实现按固定窗口长度返回 `60`。这对刚进入窗口的拒绝请求是合理的，但如果窗口即将结束，客户端会被提示等待过久。当前 rate limiter 已记录每个 bucket 的窗口开始时间，可以返回更准确的剩余秒数。

## 目标

- rate limiter 拒绝请求时计算当前固定窗口剩余时间。
- `Retry-After` 使用向上取整的剩余秒数。
- 最小返回值为 `1` 秒，避免非法或无意义的 `0`。
- 保持 JSON error body 和 rejection metrics 行为不变。

## 验收

- 同一 bucket 在窗口开始 45 秒后再次请求，返回的内部 retryAfter 为 15 秒。
- API 429 响应仍包含 `Retry-After`。
- WSL `Ubuntu-24.04` `make verify` 通过。
