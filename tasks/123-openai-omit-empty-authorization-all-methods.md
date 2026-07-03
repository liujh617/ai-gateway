# 123 OpenAI Empty Authorization All Methods

## 背景

OpenAI-compatible provider 在 upstream API key 为空时不应发送空的 `Authorization` header。已有测试覆盖了 `ListModels`，但其他调用入口也需要锁定同一约束。

## 变更

- 将空 API key 的 Authorization 省略测试扩展到：
  - `ListModels`
  - 非流式 chat completions
  - 流式 chat completions
  - embeddings

## 验证

- `go test ./internal/provider/openai`
- `make verify`
