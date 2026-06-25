# Task 040: Upstream Accept Header

## 状态

Done

## 背景

OpenAI-compatible provider 已经设置稳定的 `User-Agent`、`Content-Type` 和 request id。上游响应类型也应该通过 `Accept` header 明确协商：普通 JSON 调用期望 JSON，流式 chat completions 期望 SSE。

## 范围

实现：

- 非流式 JSON 上游请求发送 `Accept: application/json`。
- `ListModels`、非流式 chat completions 和 embeddings 使用 JSON Accept。
- 流式 chat completions 使用 `Accept: text/event-stream`。
- 补充 provider 请求 header 测试。
- 同步 API spec 和 architecture overview。

暂不实现：

- 校验上游响应 `Content-Type`。
- 为不同 provider 类型配置 Accept header。
- SSE content negotiation fallback。

## 验收标准

- JSON 上游请求发送 `Accept: application/json`。
- SSE 上游请求发送 `Accept: text/event-stream`。
- 现有 provider 行为不变。
- `make verify`、`make build`、`make check-config-examples` 和 `make smoke` 在 WSL `Ubuntu-24.04` 中通过。
