# Task 048: Strict Upstream Error JSON

## 状态

Done

## 背景

成功的上游 JSON 响应已经要求响应体只包含一个 JSON 值。Task 047 为上游错误响应补齐了大小边界，但错误响应仍需要复用同一套“单 JSON 值”解析规则，避免从拼接了额外 JSON/token 的响应中透传不可信错误字段。

## 范围

实现：
- 上游错误响应复用严格 JSON 响应解析逻辑。
- 上游错误响应体不是单个 JSON 值时，忽略其错误字段并回退到默认错误映射。
- 保留合法 upstream error 的 `message`、`type`、`param` 和 `code` 透传。
- 补充 trailing JSON upstream error 测试。
- 同步 API spec 和 architecture overview。

暂不实现：
- 将 malformed upstream error 暴露为独立错误类型。
- 校验 upstream error 响应的 `Content-Type`。
- 调整成功响应或错误响应的最大体积配置项。

## 验收标准

- 合法 upstream error JSON 仍正常映射。
- `{"error":...}{}` 这类 trailing JSON upstream error 不再透传上游错误字段。
- 上游错误响应仍遵守 10MiB 读取上限。
- `make verify`、`make build`、`make check-config-examples` 和 `make smoke` 在 WSL `Ubuntu-24.04` 中通过。
