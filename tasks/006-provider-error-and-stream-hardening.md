# Task 006: Provider 错误与流式健壮性

## 状态

Done

## 背景

不同 OpenAI-compatible upstream 在错误格式和 SSE 细节上可能存在差异。Task 006 加固 `openai-compatible` provider，让它更稳地处理真实上游响应。

## 范围

实现：

- SSE 按事件解析，而不是逐行解析。
- 支持多行 `data:` 事件。
- 忽略 SSE comment、`event`、`id`、`retry` 行。
- malformed chunk 返回可诊断 JSON 错误。
- upstream error 保留 `message`、`type`、`param`、`code`。
- 区分 upstream `401`、`403`、`404`、`429`、`5xx`。
- 非流式 JSON 响应使用 body size limit。

暂不实现：

- stream 中途 upstream error chunk 的结构化转写。
- provider 自动重试。
- provider 熔断。
- 首 token timeout。

## 验收标准

- 多行 SSE chunk 可以被解析。
- SSE comment 和 metadata 行不会破坏解析。
- malformed SSE JSON 会返回错误。
- upstream error details 被保留到 `compat.Error`。
- upstream 5xx 映射为 gateway `502 server_error`。
- `make verify` 和 `make smoke` 在 WSL `Ubuntu-24.04` 中通过。

