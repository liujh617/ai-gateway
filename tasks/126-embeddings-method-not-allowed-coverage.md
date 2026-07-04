# 126 Embeddings Method Not Allowed Coverage

## 背景

已知受保护路由使用错误 HTTP method 时，应先执行鉴权；鉴权通过后返回 JSON `405 invalid_request_error` 并设置 `Allow` header。chat completions 已有覆盖，embeddings 路径也需要直接锁定。

## 变更

- 增加 `GET /v1/embeddings` 鉴权通过后的 `405` 测试。
- 增加 `GET /v1/embeddings` 未鉴权时优先返回 `401` 的测试。

## 验证

- `go test ./internal/api`
- `make verify`
