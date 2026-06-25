# Task 042: Upstream JSON Content-Type

## 状态

Done

## 背景

Task 040 和 Task 041 已经明确了 provider 发送给上游的 `Accept` 和 `Content-Type`。成功的非流式 JSON 响应也应校验响应 `Content-Type`，避免上游以 `text/plain` 等类型返回内容时被网关当作兼容 JSON 静默接受。

## 范围

实现：

- 校验 `ListModels` 成功响应 `Content-Type` 为 `application/json`。
- 校验非流式 chat completions 成功响应 `Content-Type` 为 `application/json`。
- 校验 embeddings 成功响应 `Content-Type` 为 `application/json`。
- 允许标准参数，例如 `application/json; charset=utf-8`。
- 补充 provider 测试。
- 同步 API spec 和 architecture overview。

暂不实现：

- 校验非 2xx upstream error body 的 `Content-Type`。
- 校验 SSE stream 响应 `Content-Type`。
- 宽松接受 `application/*+json`。

## 验收标准

- 非流式 JSON 上游成功响应缺少兼容 JSON `Content-Type` 时返回 provider error。
- `application/json; charset=utf-8` 可正常解析。
- 正常上游 JSON 响应行为不变。
- `make verify`、`make build`、`make check-config-examples` 和 `make smoke` 在 WSL `Ubuntu-24.04` 中通过。
