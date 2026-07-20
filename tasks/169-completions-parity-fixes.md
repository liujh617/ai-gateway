# Task 169 - Completions Audit & Fallback Parity Fixes

## 状态

In progress.

## 背景

Task 167 引入的 `/v1/completions` 端点与 `chat_completions.go` / `embeddings.go` 在三个维度上存在对等缺口，最严重的是潜在 nil panic：

1. **nil panic 风险**：`createCompletionWithFallback` 和 `openCompletionStreamWithFallback` 在循环结束后直接返回 `lastErr`，缺少 `providerUnavailableError()` 守卫。当某模型所有 provider 都因 circuit breaker 处于 unhealthy 被跳过时，`lastErr` 保持 `nil`，返回 `(nil, "", "", nil)`，回到 handler 会解引用 nil `resp` → panic。
2. **审计路径不完整**：3 处错误路径（validation 失败、model not found、route resolve 失败）使用 `writeError` 而非 `writeAuditedError`，未记录 audit error 事件，与 chat / embeddings handler 不一致。
3. **缺少 warn/error 日志**：circuit open、provider failed、stream failed 等路径缺少 `s.logger.Warn` / `s.logger.Error` 日志，与 chat handler 不一致。
4. **缺少专属测试文件**：`internal/api/` 下没有 `completions_test.go`，AGENTS.md 要求的 7 类场景没有专属覆盖，上述 nil panic 本应被测试拦住。
5. **spec 文档缺口**：audit event 说明只写了 "chat completions 或 embeddings"，未提及 completions。

## 变更

- `internal/api/completions.go`：
  - 3 处错误路径 `writeError` → `writeAuditedError`（validation / model not found / route resolve）。
  - `createCompletionWithFallback` 和 `openCompletionStreamWithFallback` 末尾补 `if skippedFrom != "" { return ..., providerUnavailableError() }` 守卫。
  - 补齐 circuit-open / provider-failed / stream-failed 的 `s.logger.Warn` / `s.logger.Error` 日志。
- `internal/api/completions_test.go`（新增）：覆盖 AGENTS.md 要求的 7 类场景 + 所有 provider unhealthy 的 503 路径（非流式 + 流式）。
- `openai-compatible-proxy-spec.md`：audit event 说明加入 completions。

## 验收

- 所有 provider unhealthy 时返回 `503 server_error`，不 panic。
- validation / model not found / route resolve 错误路径记录 audit error 事件。
- circuit open / provider failed / stream failed 路径输出结构化日志。
- `make verify` 在 WSL `Ubuntu-24.04` 中通过。
