# Task 051: Upstream SSE Line Limit

## 状态

Done

## 背景

Task 050 为单个 SSE event 增加了大小边界，但原实现仍使用 `ReadString('\n')` 读取完整行。遇到没有换行的超长 SSE line 时，网关会先把整行读入内存，然后才进行 event 大小判断。

## 范围

实现：
- 将 SSE 行读取改为分片读取。
- 单个 SSE line 超过读取上限时立即返回 provider 解析错误。
- 保留单个 SSE event 的整体大小限制。
- 补充未结束超长 SSE line 测试。
- 同步 API spec 和 architecture overview。

暂不实现：
- 为 SSE line size 增加独立配置项。
- 限制整条 SSE stream 的总字节数。
- 支持非标准 SSE 二进制帧。

## 验收标准

- 正常 SSE 解析行为不变。
- 超长未换行 SSE line 会在读取阶段返回错误。
- 单个 SSE event 的整体大小限制仍然生效。
- `make verify`、`make build`、`make check-config-examples` 和 `make smoke` 在 WSL `Ubuntu-24.04` 中通过。
