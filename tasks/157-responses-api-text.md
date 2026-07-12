# Task 157 - Responses API 最小文本兼容

## 状态

Done.

## 目标

- 新增 `POST /v1/responses`。
- 支持字符串 input、纯文本消息数组、`instructions`、非流式 response 和 typed SSE 文本流。
- 通过 compat 转换复用现有 chat provider、fallback、熔断、超时、审计和 metrics。
- 对状态、tools、多模态和未知字段返回稳定的 `400 invalid_request_error`。
- 增加无需凭据和外网的 `make smoke-responses` 并接入 `release-check`。

## 验收

- 非流式响应包含标准 response envelope 和 output message Item。
- 流式响应使用 Responses typed SSE 事件且不发送 `[DONE]`。
- 客户端取消关闭上游 stream。
- metrics 和 audit 使用 `/v1/responses` 路径归因。
- `make verify`、`make smoke-responses` 和 `make release-check` 在 WSL `Ubuntu-24.04` 通过。
