# Task 009: 模型能力约束

## 状态

Done

## 背景

Task 008 增加了 embeddings，但 chat 和 embeddings 暂时共用同一个模型路由。真实部署时，chat model 和 embedding model 往往不同。Task 009 增加模型能力配置，避免把错误类型的模型用于错误 API。

## 范围

实现：

- `models.<name>.capabilities` 配置。
- 支持能力：`chat`、`embeddings`。
- 未配置能力时默认兼容旧行为，同时支持 `chat` 和 `embeddings`。
- `/v1/chat/completions` 只接受支持 `chat` 的模型。
- `/v1/embeddings` 只接受支持 `embeddings` 的模型。
- 配置校验拒绝未知 capability。
- 示例配置区分 chat model 和 embedding model。

暂不实现：

- provider 级能力声明。
- 模型能力自动从 upstream `/models` 拉取。
- 多模态能力。
- image/audio API。

## 验收标准

- chat-only model 调用 embeddings 返回 `404 invalid_request_error`。
- embeddings-only model 调用 chat completions 返回 `404 invalid_request_error`。
- 未配置 capabilities 的模型保持旧兼容行为。
- 配置未知 capability 时启动失败。
- `make verify` 和 `make smoke` 在 WSL `Ubuntu-24.04` 中通过。

