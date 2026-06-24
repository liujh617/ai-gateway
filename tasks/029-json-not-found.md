# Task 029: JSON Not Found Errors

## 状态

Done

## 背景

网关的业务错误已经使用 OpenAI-compatible JSON error response。但已鉴权客户端访问未知路径时，旧行为会落到 Go 默认纯文本 404，不利于 SDK 或调用方统一解析错误。

## 范围

实现：

- 未知路由返回 JSON 格式错误。
- 鉴权通过的未知受保护路由返回 `404 invalid_request_error`。
- 未鉴权访问未知受保护路由仍先返回 `401 authentication_error`。
- 保持已有公开探针和业务路由行为不变。
- 同步 API spec。

暂不实现：

- 自定义 `405 Method Not Allowed` JSON 响应。
- 路由建议或相似路径提示。
- 为未知路由绕过鉴权。

## 验收标准

- 已鉴权未知路由返回 JSON `404 invalid_request_error`。
- 未鉴权未知路由返回 JSON `401 authentication_error`。
- 现有路由测试不受影响。
- `make verify`、`make build`、`make check-config-examples` 和 `make smoke` 在 WSL `Ubuntu-24.04` 中通过。
