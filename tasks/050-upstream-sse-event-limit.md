# Task 050: Upstream SSE Event Limit

## 状态

Done

## 背景

非流式上游 JSON 响应已经有读取大小限制。流式响应可以持续很久，不能限制整条流的总大小，但单个 SSE event 如果异常巨大，会让网关在解析事件时占用过多内存。

## 范围

实现：
- 为单个上游 SSE event 增加大小边界。
- 超过边界时返回 provider 解析错误。
- 保持长时间多事件流式响应不受总大小限制影响。
- 补充 oversized SSE event 测试。
- 同步 API spec 和 architecture overview。

暂不实现：
- 为 SSE event size 增加独立配置项。
- 限制整条 SSE stream 的总字节数。
- 改写 SSE parser 为低层分片读取器。

## 验收标准

- 正常 SSE event 行为不变。
- 单个 SSE event 超过响应体读取上限时，`Next` 返回错误。
- 多行 `data:`、comment 和 metadata 行仍按既有规则处理。
- `make verify`、`make build`、`make check-config-examples` 和 `make smoke` 在 WSL `Ubuntu-24.04` 中通过。
