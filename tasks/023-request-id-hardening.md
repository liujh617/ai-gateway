# Task 023: Request ID Hardening

## 状态

Done

## 背景

网关会把 request id 写入响应头和 access log，方便客户端、代理层和服务端日志关联。同一请求链路中复用客户端传入的 `X-Request-Id` 很有用，但不能无条件信任外部 header，否则可能引入日志污染、过长字段或不可见字符。

## 范围

实现：

- 继续支持客户端传入 `X-Request-Id`。
- 去掉 request id 首尾空白。
- 拒绝空值、超过 128 字节、包含空白/控制字符或非 ASCII 可见字符的 request id。
- 缺失或非法时生成新的 16 字节 hex request id。
- 响应头始终返回最终使用的 `X-Request-Id`。
- context 和 access log 使用同一个最终 request id。
- 同步 API spec、架构说明和部署文档。

暂不实现：

- W3C `traceparent` 解析。
- OpenTelemetry trace/span 集成。
- request id 传递到上游 provider 请求。

## 验收标准

- 合法 `X-Request-Id` 会被复用并写回响应。
- 首尾空白会被 trim。
- 非法 `X-Request-Id` 不会进入 context 或响应。
- `make verify`、`make build`、`make check-config-examples` 和 `make smoke` 在 WSL `Ubuntu-24.04` 中通过。
