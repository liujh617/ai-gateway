# Task 056: SSE CR Line Nonblocking

## 状态

Done

## 背景

Task 055 支持了 CR-only SSE line ending，但实现中在读到 `\r` 后会尝试查看下一个字节，以判断是否为 CRLF。对于真正使用 CR-only 且上游 flush 后暂停的流，这会让 parser 等待后续字节，不能在 CR 到达时立即完成当前行。

## 范围

实现：
- 读到 `\r` 时立即结束当前 SSE line。
- 如果后续字节是 CRLF 中的 `\n`，下一次读取时跳过它。
- 保留 LF、CRLF 和 CR 三种 line ending 兼容。
- 补充 CR-terminated event flush 后不等待后续字节的测试。
- 同步 API spec 和 architecture overview。

暂不实现：
- 暴露 line ending 类型给上层。
- 调整 SSE event 必须以空行结束的规则。
- 为 line reader 增加独立超时。

## 验收标准

- CR-only 完整 event 在空行 CR 到达后立即返回 chunk。
- CRLF 不会被误判为额外空行。
- 现有 LF、CR-only、大小限制、BOM 和终止规则不变。
- `make verify`、`make build`、`make check-config-examples` 和 `make smoke` 在 WSL `Ubuntu-24.04` 中通过。
