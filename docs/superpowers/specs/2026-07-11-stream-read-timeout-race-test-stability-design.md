# Stream Read Timeout Race Test Stability Design

## 背景

WSL `Ubuntu-24.04` 中执行 `make verify` 时，普通测试通过，但 race 测试可能在
`TestStreamChatCompletionReadTimeoutIsDeadlineExceeded` 失败。该测试为整个 HTTP client
设置 `10ms` timeout，同时假定 `StreamChatCompletion` 一定能在 timeout 前完成响应头阶段，
随后才在 `stream.Next` 中观察读取超时。race instrumentation 和全量测试负载会放大调度延迟，
使 timeout 偶尔在 `client.Do` 阶段先触发。

读取 SSE body 时的 transport timeout 归一化由 `internal/provider/httpx` 负责。OpenAI provider
在响应头校验通过后只把 `resp.Body` 交给 `httpx.NewChatCompletionStream`，因此 provider 层的
读取超时测试重复覆盖了 `httpx` 的职责，并引入了不必要的真实时间依赖。

## 目标

- 消除流式读取超时测试对 goroutine 调度和毫秒级真实时间窗口的依赖。
- 保留 `context.DeadlineExceeded` 错误归一化的直接覆盖。
- 保留 OpenAI provider 建连阶段 timeout 的覆盖。
- 使目标测试和完整 `make verify` 在 race 模式下稳定通过。

## 非目标

- 不修改 provider、SSE parser、HTTP client timeout 或其他生产行为。
- 不调整公开 API、配置、兼容契约或架构边界。
- 不处理 `make verify` 的格式化行为或其他测试稳定性问题。
- 不通过增大 sleep 和 timeout 来掩盖调度竞争。

## 方案

### `httpx` 层确定性测试

修改 `internal/provider/httpx/httpx_test.go`。以测试专用 `io.ReadCloser` 作为
`httpx.NewChatCompletionStream` 的输入；其 `Read` 立即返回一个实现 `net.Error`、且
`Timeout() == true` 的错误。调用 `stream.Next(context.Background())` 后，断言返回错误可由
`errors.Is(err, context.DeadlineExceeded)` 识别。

测试不启动 HTTP server、不调用 `time.Sleep`，也不依赖 `http.Client.Timeout`。它直接验证
负责该行为的组件边界：SSE stream 把底层 timeout transport error 归一化为
`context.DeadlineExceeded`。

### OpenAI provider 层去重

删除 `internal/provider/openai/openai_test.go` 中
`TestStreamChatCompletionReadTimeoutIsDeadlineExceeded`。继续保留
`TestStreamChatCompletionConnectTimeoutIsDeadlineExceeded`，验证 OpenAI provider 对
`client.Do` 阶段 timeout 的处理。

OpenAI provider 的正常 SSE 响应、content type、chunk parsing、`[DONE]`、错误响应和取消行为
仍由现有测试覆盖。本次删除不会减少 provider 专属逻辑的覆盖。

## 文件范围

- 修改 `internal/provider/httpx/httpx_test.go`：用确定性 reader 重写读取超时测试。
- 修改 `internal/provider/openai/openai_test.go`：删除重复且依赖真实时间的读取超时测试。
- 新增 `tasks/153-stream-read-timeout-race-test-stability.md`：记录问题、范围和验收标准。

不修改任何生产 Go 文件，也不要求更新外部 API spec 或 architecture 文档。

## 验证策略

所有最终验证在 WSL `Ubuntu-24.04` 中执行：

```bash
go test -race ./internal/provider/httpx -run TestStreamReadTimeoutIsDeadlineExceeded -count=100
go test -race ./internal/provider/openai -run TestStreamChatCompletionConnectTimeoutIsDeadlineExceeded -count=20
make verify
```

验收条件：

- `httpx` 读取超时测试不含 wall-clock timeout、sleep 或 test HTTP server。
- OpenAI provider 建连 timeout 测试继续通过。
- 生产代码无变更。
- 上述三个验证命令全部通过。

## 风险与控制

删除 provider 层重复测试可能被误解为降低覆盖。控制方式是明确按职责分层：provider 层覆盖
建连和 provider 专属请求/响应行为，`httpx` 层覆盖 stream body 的读取及 timeout 归一化。
实施时通过现有正常 SSE 测试确认 OpenAI provider 仍把 response body 接入共享 stream。
