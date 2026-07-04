# 124 OpenAI Oversized JSON Response Coverage

## 背景

OpenAI-compatible provider 使用统一的响应体大小限制读取 JSON 响应。非流式 chat completions 已有超大响应测试，models 和 embeddings 也需要直接覆盖。

## 变更

- 增加 `ListModels` 超大 JSON 响应拒绝测试。
- 增加 `CreateEmbedding` 超大 JSON 响应拒绝测试。

## 验证

- `go test ./internal/provider/openai`
- `make verify`
