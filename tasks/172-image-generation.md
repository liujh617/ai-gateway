# Task 172 - Image Generation

## 状态

In progress.

## 背景

网关已支持 Chat Completions、Completions、Embeddings、Responses 和 Models，但缺少图像生成能力。`POST /v1/images/generations` 允许通过文本 prompt 生成图像，是 OpenAI API 的核心端点之一。

## 变更

- 新增 `POST /v1/images/generations` 端点。
- 新增 `compat.ImageGenerationRequest` 和 `ImageGenerationResponse` 类型。
- `Provider` 接口新增 `CreateImage` 方法。
- fake、openai、azureopenai 三个 provider 实现 CreateImage。
- 新增 `images` capability。
- 复用 model router、fallback、circuit breaker、metrics、audit。

## 验收

- JSON 响应返回 OpenAI-compatible 格式。
- 缺失 model、空 prompt 返回 `400 invalid_request_error`。
- 鉴权失败返回 `401`。
- 模型无 images capability 返回 `404 model_not_found`。
- 不支持的方法返回 `405` + `Allow: POST`。
- provider fallback、circuit breaker、metrics 和 audit 在 images 路径上生效。
- WSL `make verify` 通过。
