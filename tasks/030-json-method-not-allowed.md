# Task 030: JSON Method Not Allowed Errors

## 状态

Done

## 背景

Task 029 已经让未知路由返回 OpenAI-compatible JSON 404。另一个相邻边界是已知路径使用错误 HTTP method 时，Go 默认会返回纯文本 405。为了让客户端统一解析错误，method not allowed 也应返回 JSON error response。

## 范围

实现：

- 已知路由使用错误 HTTP method 时返回 JSON 格式错误。
- 鉴权通过的受保护路由返回 `405 invalid_request_error`。
- 未鉴权访问受保护路由仍先返回 `401 authentication_error`。
- 公开路由的错误 method 返回 `405 invalid_request_error`。
- 设置 `Allow` header。
- 同步 API spec。

暂不实现：

- 自动从 mux pattern 反推 method 列表。
- OPTIONS/CORS 预检支持。
- 为未知路由返回 method 建议。

## 验收标准

- 已鉴权业务路由错误 method 返回 JSON `405 invalid_request_error`。
- 未鉴权业务路由错误 method 返回 JSON `401 authentication_error`。
- 公开路由错误 method 返回 JSON `405 invalid_request_error`。
- `Allow` header 正确返回支持的 method。
- `make verify`、`make build`、`make check-config-examples` 和 `make smoke` 在 WSL `Ubuntu-24.04` 中通过。
