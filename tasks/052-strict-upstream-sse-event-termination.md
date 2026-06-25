# Task 052: Strict Upstream SSE Event Termination

## 状态

Done

## 背景

SSE event 按协议以空行结束。当前 provider parser 在 EOF 时会把已经读取到的 `data:` 内容作为一个完整事件返回，这会让上游中途断开的半截 event 被当作合法 chunk。

## 范围

实现：
- 上游 SSE event 必须以空行结束。
- EOF 发生在 event 中途时返回 provider 解析错误。
- 保留空流 EOF 和正常 `[DONE]\n\n` 行为。
- 补充未结束 SSE event 测试。
- 同步 API spec 和 architecture overview。

暂不实现：
- 自动补全缺失空行。
- 将中途 EOF 映射成独立错误类型。
- 调整 handler 层的流式错误 chunk 策略。

## 验收标准

- 正常以空行结束的 SSE event 行为不变。
- `data: {...}` 后直接 EOF 会返回错误。
- 空流 EOF 仍返回 `io.EOF`。
- `make verify`、`make build`、`make check-config-examples` 和 `make smoke` 在 WSL `Ubuntu-24.04` 中通过。
