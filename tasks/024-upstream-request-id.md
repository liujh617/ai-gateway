# Task 024: Upstream Request ID Propagation

## 状态

Done

## 背景

Task 023 已经让网关生成、校验并回写最终 request id。为了把客户端、网关和上游 provider 的日志串起来，OpenAI-compatible provider 调用上游时也应携带同一个 `X-Request-Id`。

## 范围

实现：

- 新增 `internal/requestctx`，集中保存 request id context key 和 header 名。
- middleware 生成或复用 request id 后写入 `requestctx`。
- access log 继续读取同一个最终 request id。
- OpenAI-compatible provider 发起上游请求时转发 `X-Request-Id`。
- 覆盖 chat completions、stream chat completions 和 embeddings 上游请求。
- 同步 API spec、架构说明和部署文档。

暂不实现：

- W3C `traceparent` 转发。
- OpenTelemetry trace/span 集成。
- 对 fake provider 暴露 request id 行为。

## 验收标准

- 非流式 chat 上游请求包含最终 `X-Request-Id`。
- 流式 chat 上游请求包含最终 `X-Request-Id`。
- embeddings 上游请求包含最终 `X-Request-Id`。
- `make verify`、`make build`、`make check-config-examples` 和 `make smoke` 在 WSL `Ubuntu-24.04` 中通过。
