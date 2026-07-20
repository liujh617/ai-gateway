# Task 170 - Audit duration_ms Landing

## 状态

Done.

## 背景

`audit.Event` 结构体定义了 `DurationMS int64 json:"duration_ms,omitempty"` 字段（`internal/audit/audit.go:39`），但整个代码库中没有任何位置给它赋值。所有审计事件（request、response、stream_chunk、stream_done、error）的 `duration_ms` 都是零值，因 `omitempty` 在 JSON 中被省略。

这意味着审计日志无法回答"这次请求/流式响应耗时多久"，削弱了审计模式用于研究 agent 行为时的可观测性。access log 和 metrics 都已有 latency/duration 信息，唯独审计事件缺失。

## 变更

1. `internal/requestctx/request_id.go`：新增 `WithStartedAt(ctx, time.Time)` 和 `StartedAt(ctx) time.Time`，用独立 context key 存储请求开始时间。
2. `internal/middleware/request_id.go`：在 `RequestID` 中间件入口捕获 `started := time.Now()`，通过 `requestctx.WithStartedAt` 写入 context。选择 RequestID 是因为它是最早的"业务"中间件（在 Recovery、SecurityHeaders 之后），与 request id 同生命周期。
3. `internal/api/server.go`：`auditBaseEvent` 从 context 读取 `StartedAt`，计算 `time.Since(started).Milliseconds()` 填入 `DurationMS`。
   - `request` 事件在请求开始时记录，duration≈0，因 `omitempty` 自动省略。
   - `response`/`stream_done`/`error` 事件携带从请求开始到事件发生的耗时。
   - `stream_chunk` 事件携带从请求开始到该 chunk 的累计耗时。
4. 测试：新增 `requestctx` 单元测试 + 3 个审计 duration 集成测试（非流式 response、流式 stream_done、error 路径），用 `sleepyChatProvider` 和 `delayedStreamProvider` 保证 `DurationMS > 0` 的确定性。
5. `openai-compatible-proxy-spec.md`：审计事件说明中加入 `duration_ms` 字段描述。

## 附带修复（Task 169 遗留编译/测试错误）

验证过程中发现 Task 169 的 `completions_test.go` 从未成功编译，`make verify` 此前未真正通过：

- `completions_test.go`：`countingCompletionProvider` 存在 `calls` 字段与 `calls()` 方法同名冲突（Go 不允许），重命名方法为 `callCount()`。
- `completions_test.go`：`TestCompletionsRejectsClientDisallowedModel` 存在未使用变量 `rr`，清理为 `handler, _ :=`。
- `completions_test.go`：`TestCompletionsAllProvidersUnhealthyReturns503` 和 `TestCompletionsStreamAllProvidersUnhealthyReturns503` 错误期望两个始终失败的 provider 返回 200；修正为期望 502（circuit 未打开时）然后 503（circuit 打开后）。
- `completions.go`：流式 `[DONE]` 通过 `writeSSE` 发送，被 JSON 编码为 `data: "[DONE]"`（带引号），改为 `io.WriteString(w, "data: [DONE]\n\n")`，与 chat completions 和 OpenAI 协议一致。
- `responses_test.go`：`responseFunctionStateProvider` 和 `functionStreamProvider` 缺少 `CreateCompletion`/`StreamCompletion` 接口方法，补齐 stub。

## 验收

- 非流式成功响应的 `response` 审计事件含 `duration_ms > 0`。
- 流式成功响应的 `stream_done` 审计事件含 `duration_ms > 0`。
- 错误路径的 `error` 审计事件含 `duration_ms > 0`。
- `request` 审计事件不含 `duration_ms`（因 `omitempty`）。
- `make verify`（含 `go test`、`go test -race`、`go vet`）在 WSL `Ubuntu-24.04` 中通过。
