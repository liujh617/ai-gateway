# Task 049: Upstream Error Content-Type

## 状态

Done

## 背景

Task 048 已经要求上游错误响应体必须是单个 JSON 值后才会透传错误字段。但如果上游错误响应没有声明 JSON `Content-Type`，即使 body 看起来像 JSON，也不应把其中字段当作可信 OpenAI-compatible error。

## 范围

实现：
- 仅当 upstream error response 的 `Content-Type` 是 `application/json` 时解析错误体。
- 保留 `application/json; charset=utf-8` 这类标准参数兼容。
- 非 JSON Content-Type 的 upstream error 回退到默认错误映射。
- 补充非 JSON Content-Type 伪装错误 JSON 的测试。
- 同步 API spec 和 architecture overview。

暂不实现：
- 要求 upstream error 必须返回 JSON Content-Type。
- 将非 JSON upstream error Content-Type 暴露为独立错误类型。
- 解析 `text/plain` 或其他媒体类型中的 JSON-like body。

## 验收标准

- JSON Content-Type 的合法 upstream error 仍保留 `message`、`type`、`param` 和 `code`。
- 非 JSON Content-Type 的 upstream error 不再透传 body 中的错误字段。
- 非 JSON Content-Type 的上游 `5xx` 仍映射为 `502 server_error`。
- `make verify`、`make build`、`make check-config-examples` 和 `make smoke` 在 WSL `Ubuntu-24.04` 中通过。
