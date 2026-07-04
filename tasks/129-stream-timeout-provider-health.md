# 129 Stream Timeout Provider Health

## 背景

流式 chat completions 在响应开始后不会切换 fallback provider，避免混合两个上游的 SSE 响应。但如果 stream 读取阶段因 timeout 结束，该 provider 仍应记录一次失败，使后续请求可以按 circuit breaker 跳过 unhealthy provider。

客户端主动取消仍不应计为 provider 失败。

## 变更

- stream `Next` 返回 `context.DeadlineExceeded` 时标记 provider failure 并刷新 provider health metrics。
- 增加回归测试：首次 stream timeout 后，后续 stream 请求跳过 primary provider 并使用 fallback。

## 验证

- `go test ./internal/api -run TestChatCompletionsStreamTimeoutMarksProviderFailure -count=1`
- `go test ./internal/api`
- `make verify`
