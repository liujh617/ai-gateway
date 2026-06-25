# Task 055: SSE CR Line Ending

## 状态

Done

## 背景

SSE 规则允许 line ending 使用 LF、CRLF 或 CR。当前 provider parser 主要按 LF 读取行，CR-only 上游流会被误认为没有事件结束空行，进而触发未结束 event 错误。

## 范围

实现：
- SSE line reader 支持 LF、CRLF 和 CR。
- 保留单行大小限制。
- 补充 CR-only SSE stream 测试。
- 同步 API spec 和 architecture overview。

暂不实现：
- 支持非标准二进制帧。
- 改变 SSE event 必须以空行结束的规则。
- 暴露 line ending 类型给上层。

## 验收标准

- 使用 CR-only line ending 的 `data` event 能正常解析。
- 使用 CR-only line ending 的 `[DONE]` 能正常结束流。
- 现有 LF/CRLF、大小限制、BOM 和终止规则不变。
- `make verify`、`make build`、`make check-config-examples` 和 `make smoke` 在 WSL `Ubuntu-24.04` 中通过。
