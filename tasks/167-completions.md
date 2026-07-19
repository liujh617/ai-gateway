# Task 167 - Legacy Text Completions

## 状态

Done.

## 背景

网关已支持 Chat Completions、Responses、Embeddings、Models 等端点，但缺少 legacy `/v1/completions`。该端点被部分 OpenAPI-compatible provider 和旧客户端使用，需要作为基本能力补充。

## 变更

- 新增 `POST /v1/completions` 端点，支持非流式 JSON 和 SSE 流式两种模式。
- 新增 `compat.CompletionsRequest`、`CompletionsResponse`、`CompletionsChunk` 等类型。
- 扩展 `Provider` 接口添加 `CreateCompletion` 和 `StreamCompletion` 方法。
- 新增 `CompletionStream` 接口和 `httpx.NewCompletionStream` SSE 解析器。
- fake、openai、azureopenai 三个 provider 均实现 completions 方法。
- 新增 `completions` capability，默认启用。
- `/v1/completions` 共享 model router、provider fallback、circuit breaker、metrics 和 audit 基础设施。

## 验收

- JSON 响应返回 `text_completion` 格式的 choices。
- SSE 流式响应每行 `data:` 格式，以 `[DONE]` 结束。
- 缺失 model、空 prompt 返回 `400 invalid_request_error`。
- 鉴权失败返回 `401`。
- 模型无 completions capability 返回 `404 model_not_found`。
- 不支持的方法返回 `405` + `Allow: POST`。
- provider fallback、circuit breaker、metrics 和 audit 在 completions 路径上均生效。
- WSL `make verify` 通过。
