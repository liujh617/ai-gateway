# Task 059: Stream Connect Timeout Coverage

## 状态

Done

## 背景

Task 058 明确了 transport timeout 规范化覆盖 models、chat completions、embeddings 和 stream 建连阶段。前三类已经有测试覆盖；流式 chat completions 建连阶段也需要显式测试，避免后续改动让 stream 建连 timeout 回退成普通 provider error。

## 范围

实现：
- 补充 `StreamChatCompletion` 建连阶段 transport timeout 测试。
- 确认错误能被 `errors.Is(err, context.DeadlineExceeded)` 识别。
- 同步 API spec 和 architecture overview，使描述更明确地区分非流式和流式 chat completions。

暂不实现：
- 增加 stream 读取阶段独立 timeout 类型。
- 改变 HTTP API 的流式错误响应策略。
- 调整 provider timeout 配置项。

## 验收标准

- Stream chat completions 建连阶段 timeout 能被识别为 `context.DeadlineExceeded`。
- Models、非流式 chat completions 和 embeddings 的 timeout 覆盖保持不变。
- HTTP API 仍可将 provider timeout 映射为 `504 server_error`。
- `make verify`、`make build`、`make check-config-examples` 和 `make smoke` 在 WSL `Ubuntu-24.04` 中通过。
