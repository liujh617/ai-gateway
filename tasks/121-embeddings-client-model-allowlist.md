# 121 Embeddings Client Model Allowlist

## 背景

网关已支持按 client 限制可访问模型，并覆盖了 `/v1/models` 和 `/v1/chat/completions` 的行为。

## 变更

- 增加 `/v1/embeddings` 在 client 模型白名单下拒绝未授权模型的测试。
- 验证被拒绝时不会调用 provider adapter。

## 验证

- `go test ./internal/api`
- `make verify`
