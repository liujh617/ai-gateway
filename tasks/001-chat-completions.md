# Task 001: 实现 `/v1/chat/completions`

## 状态

Done

## 背景

`/v1/chat/completions` 是 OpenAI-compatible 代理的第一条核心路径。完成该任务后，现有 OpenAI-compatible 客户端应能通过网关发起基础 chat completion 请求，并获得非流式或 SSE 流式响应。

相关文档：

- [README.md](../README.md)
- [OpenAI-compatible Proxy Spec](../openai-compatible-proxy-spec.md)
- [Architecture Overview](../architecture/overview.md)
- [Testing Environment](../docs/testing-environment.md)
- [ADR 0001](../docs/adr/0001-go-openai-compatible-proxy.md)

## 范围

实现：

- `POST /v1/chat/completions`
- 非流式响应
- `stream: true` 的 SSE 响应
- 基础请求校验
- OpenAI-compatible 错误响应
- 模型路由接口
- fake provider 测试

暂不实现：

- tool calling 的完整语义执行
- 多 provider fallback
- 成本统计
- 多租户配额
- 内容审计

## 接口行为

### 请求校验

必须校验：

- body 是合法 JSON。
- `model` 非空。
- `messages` 非空。
- 每个 message 的 `role` 非空。
- 每个 message 至少有可处理的 `content`。

暂不支持的字段可以保留在结构中，但不能导致请求失败，除非字段类型明显非法。

### 非流式响应

当 `stream` 为 `false` 或缺省时：

- handler 调用 provider 的非流式方法。
- 返回 `application/json`。
- 响应格式符合 `chat.completion`。
- provider 错误转换为 OpenAI-compatible error response。

### 流式响应

当 `stream` 为 `true` 时：

- handler 检查 `http.Flusher`。
- 设置 SSE headers。
- 调用 provider 的 stream 方法。
- 每个 chunk 写入 `data: <json>\n\n`。
- 每次写入后 flush。
- 正常结束时写入 `data: [DONE]\n\n`。
- request context 取消时关闭上游 stream。

## 建议实现步骤

1. 初始化 Go module。
2. 建立 `cmd/gateway/main.go`。
3. 建立 `internal/api`，注册 `/v1/chat/completions`。
4. 建立 `internal/compat`，定义 OpenAI-compatible request/response/error 类型。
5. 建立 `internal/provider`，定义 provider 接口和 fake provider。
6. 建立 `internal/router`，实现静态模型路由。
7. 实现非流式 handler。
8. 实现 SSE streaming handler。
9. 增加 middleware：request id、recovery、auth、logging。
10. 增加单元测试和 handler 测试。

## 验收标准

- 使用兼容客户端可以完成一次非流式 chat completion。
- 使用兼容客户端可以完成一次流式 chat completion。
- 无效 JSON 返回 `400 invalid_request_error`。
- 缺少 Bearer token 返回 `401 authentication_error`。
- 未配置模型返回 `404 invalid_request_error`。
- provider 超时返回 `504 server_error`。
- 测试覆盖 fake provider 的成功、错误和 streaming 场景。
- 所有最终验证命令在 WSL `Ubuntu-24.04` 中执行。

## 验证环境

标准验证环境：

- WSL distro: `Ubuntu-24.04`
- Repo path: `/mnt/e/code/open-ai-gateway`
- Shell: bash

建议验证命令：

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/open-ai-gateway -- bash -lc "go test ./..."
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/open-ai-gateway -- bash -lc "go test -race ./..."
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/open-ai-gateway -- bash -lc "go vet ./..."
```

## 测试清单

- `TestChatCompletionsNonStreamOK`
- `TestChatCompletionsStreamOK`
- `TestChatCompletionsInvalidJSON`
- `TestChatCompletionsMissingModel`
- `TestChatCompletionsMissingMessages`
- `TestChatCompletionsUnauthorized`
- `TestChatCompletionsModelNotFound`
- `TestChatCompletionsProviderError`
- `TestChatCompletionsClientCancelClosesStream`

## 完成后的文档更新

已更新：

- README 的运行方式。
- 本任务状态从 `Planned` 改为 `Done`。

待后续真实 provider 接入时继续更新：

- spec 中实际支持字段。
- architecture 中真实目录和请求流程。
