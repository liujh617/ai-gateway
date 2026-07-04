# 125 OpenAI JSON Charset Coverage

## 背景

OpenAI-compatible provider 允许 `application/json` 响应携带 charset 参数。chat completions 已有覆盖，models 和 embeddings 也应锁定同一兼容性。

## 变更

- 增加 `ListModels` 接受 `application/json; charset=utf-8` 的测试。
- 增加 `CreateEmbedding` 接受 `application/json; charset=utf-8` 的测试。

## 验证

- `go test ./internal/provider/openai`
- `make verify`
