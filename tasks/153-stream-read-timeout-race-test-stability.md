# Task 153 - Stream Read Timeout Race Test Stability

## 状态

Done.

## 背景

流式读取超时测试使用 `10ms` HTTP client timeout，并假定响应头阶段必定在 timeout 前完成。
在 `go test -race ./...` 的 instrumentation 和全量负载下，timeout 可能在
`StreamChatCompletion` 返回 stream 之前触发，导致测试偶发失败。

## 变更

- 在 `internal/provider/httpx` 使用立即返回 timeout error 的测试 reader，确定性验证
  stream 读取错误归一化为 `context.DeadlineExceeded`。
- 删除 OpenAI provider 中重复且依赖真实时间窗口的 stream read timeout 测试。
- 保留 OpenAI provider 建连 timeout 和现有 SSE 行为覆盖。
- 不修改生产代码。

## 验收

- `go test -race ./internal/provider/httpx -run TestStreamReadTimeoutIsDeadlineExceeded -count=100`
- `go test -race ./internal/provider/openai -run TestStreamChatCompletionConnectTimeoutIsDeadlineExceeded -count=20`
- `make verify`
