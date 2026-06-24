# Task 021: Request Field Passthrough

## 状态

Done

## 背景

真实 OpenAI-compatible 客户端会在基础字段之外传递高级参数，例如 chat completions 的 `tools`、`tool_choice`、`response_format`，以及 embeddings 的 `dimensions`。如果网关只解析固定字段，上游 provider 就收不到这些参数，会降低兼容性。

## 范围

实现：

- chat completions 请求保留未知 JSON 字段。
- embeddings 请求保留未知 JSON 字段。
- OpenAI-compatible provider 转发请求时合并未知字段。
- 网关仍然覆盖 `model` 为路由后的上游模型名。
- 非流式 chat provider 调用仍然清除 `stream=true`。
- 补充 API 层和 provider 层测试。
- 同步 README 和 API spec。

暂不实现：

- 对所有 OpenAI 高级字段做语义校验。
- provider 级字段白名单或黑名单。
- endpoint 级透传策略配置。

## 验收标准

- chat completions 的 `tools`、`tool_choice`、`response_format` 可以透传到 provider。
- embeddings 的 `dimensions` 可以透传到 provider。
- 已知字段优先于透传字段，避免客户端覆盖网关路由后的 `model` 或 provider 调用的 `stream` 行为。
- `make verify`、`make build`、`make check-config-examples` 和 `make smoke` 在 WSL `Ubuntu-24.04` 中通过。
