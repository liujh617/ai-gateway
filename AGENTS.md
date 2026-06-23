# AGENTS.md

本文件为参与 `open-ai-gateway` 的 coding agent 和自动化协作者提供工程约束。

## 项目方向

这是一个基于 Go 的 OpenAI-compatible API 代理。优先实现小而清晰的网关核心，而不是一次性覆盖全部 OpenAI API。

当前推进顺序：

1. 完成项目说明和兼容契约。
2. 完成架构概览。
3. 实现 `/v1/chat/completions`。
4. 增加 provider adapter 和集成测试。
5. 扩展 `/v1/models`、embeddings、限流和观测能力。

## 编码原则

- 优先使用 Go 标准库。
- HTTP server 默认使用 `net/http`。
- JSON 默认使用 `encoding/json`。
- 使用 `context.Context` 贯穿请求、上游调用和流式响应。
- 保持 handler、compat、router、provider 的边界清晰。
- 不在 handler 中写 provider 专属逻辑。
- 不在 provider adapter 中写外部 API 鉴权逻辑。

## 文档同步

涉及外部 API 契约的变更，必须同步更新：

- [openai-compatible-proxy-spec.md](openai-compatible-proxy-spec.md)
- [architecture/overview.md](architecture/overview.md)
- 相关 task 文档
- 必要时新增或更新 ADR

## 测试要求

标准测试与验证环境是 WSL 中的 `Ubuntu-24.04`。除非用户明确要求，最终验证命令必须在该环境中执行。

推荐命令格式：

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/open-ai-gateway -- bash -lc "<command>"
```

详细约定见 [Testing Environment](docs/testing-environment.md)。

新增行为至少覆盖：

- 正常 JSON 响应。
- SSE streaming 响应。
- 无效请求。
- 鉴权失败。
- 模型不存在。
- 上游错误。
- context cancellation。

对流式响应的测试必须验证：

- `Content-Type: text/event-stream`。
- 每个事件使用 `data:`。
- 正常结束发送 `[DONE]`。
- 客户端取消时上游 stream 被关闭。

## 安全约束

- 不提交真实 API key。
- 不在日志中输出 Authorization header。
- 不默认记录完整 prompt 或 completion。
- provider API key 只从服务端配置读取。
- 示例配置使用明显的占位值。

## 风格约束

- 文档使用中文，保留必要英文协议名和字段名。
- Go package 命名保持短小、语义明确。
- 错误信息面向调用方时保持稳定，内部细节写入日志。
- 新增依赖前先确认标准库无法合理解决。
