# Task 047: Upstream Error Body Limit

## 状态

Done

## 背景

成功的上游 JSON 响应已经有读取大小限制和严格 JSON 校验。错误响应路径虽然也使用读取上限，但之前无法判断响应体是否被截断；如果上游错误体超过限制，网关仍可能从截断内容中解析并保留不可信的错误字段。

## 范围

实现：
- 为上游错误响应体读取增加明确的超限判断。
- 上游错误响应体超限时忽略错误体内容，回退到默认错误映射。
- 保留现有正常大小 upstream error 的字段透传行为。
- 补充 oversized upstream error 测试。
- 同步 API spec 和 architecture overview。

暂不实现：
- 对 upstream error body 做严格单 JSON 校验。
- 暴露上游错误体超限的独立错误类型。
- 调整成功响应的最大体积配置项。

## 验收标准

- 正常大小的 upstream error 仍保留 `message`、`type`、`param` 和 `code`。
- 超过读取上限的 upstream error body 不再参与错误字段映射。
- 上游 `5xx` 且错误体超限时仍映射为 `502 server_error`。
- `make verify`、`make build`、`make check-config-examples` 和 `make smoke` 在 WSL `Ubuntu-24.04` 中通过。
