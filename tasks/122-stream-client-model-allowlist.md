# 122 Stream Client Model Allowlist

## 背景

`/v1/chat/completions` 的非流式请求已经覆盖 client 模型白名单拒绝逻辑。流式请求虽然共享同一段前置校验，但需要单独测试锁定。

## 变更

- 增加流式 chat completions 请求访问 client 不可见模型的 API 测试。
- 验证拒绝发生在打开 provider stream 之前。

## 验证

- `go test ./internal/api`
- `make verify`
