# Task 043: Upstream SSE Content-Type

## 状态

Done

## 背景

Task 042 校验了非流式 JSON 上游成功响应的 `Content-Type`。流式 chat completions 请求已经通过 `Accept: text/event-stream` 明确要求 SSE 响应，因此成功响应也应校验 `Content-Type`，避免把非 SSE 响应当作 event stream 解析。

## 范围

实现：

- 校验流式 chat completions 成功响应 `Content-Type` 为 `text/event-stream`。
- 允许标准参数，例如 `text/event-stream; charset=utf-8`。
- 保持非 2xx upstream error mapping 行为不变。
- 复用 JSON/SSE content type 判断 helper。
- 补充 provider 测试。
- 同步 API spec 和 architecture overview。

暂不实现：

- 校验非 2xx upstream error body 的 `Content-Type`。
- 对 stream payload 增加额外 JSON framing 限制。
- SSE content negotiation fallback。

## 验收标准

- 流式上游成功响应缺少兼容 SSE `Content-Type` 时返回 provider error。
- `text/event-stream; charset=utf-8` 可正常解析。
- 正常 SSE 上游响应行为不变。
- `make verify`、`make build`、`make check-config-examples` 和 `make smoke` 在 WSL `Ubuntu-24.04` 中通过。
