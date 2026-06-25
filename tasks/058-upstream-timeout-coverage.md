# Task 058: Upstream Timeout Coverage

## 状态

Done

## 背景

Task 057 已经让 OpenAI-compatible provider 将上游 transport timeout 规范化为 `context.DeadlineExceeded`。该逻辑由 provider 内部共用，但目前测试只覆盖 chat completions；models 和 embeddings 也需要明确锁定同一语义。

## 范围

实现：
- 补充 embeddings transport timeout 测试。
- 补充 list models transport timeout 测试。
- 确认两条路径都能被 `errors.Is(err, context.DeadlineExceeded)` 识别。
- 同步 API spec 和 architecture overview，明确 timeout 规范化覆盖的上游调用类型。

暂不实现：
- 改变现有 timeout 映射响应格式。
- 为不同 endpoint 设置不同 timeout。
- 增加 stream 读取阶段的独立 timeout 类型。

## 验收标准

- Chat completions、embeddings 和 list models 的 transport timeout 都能被识别为 `context.DeadlineExceeded`。
- HTTP API 仍可将 provider timeout 映射为 `504 server_error`。
- 非 timeout transport error 行为不变。
- `make verify`、`make build`、`make check-config-examples` 和 `make smoke` 在 WSL `Ubuntu-24.04` 中通过。
