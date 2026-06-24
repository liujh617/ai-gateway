# Task 008: Embeddings API

## 状态

Done

## 背景

项目已经支持 chat completions 和真实 OpenAI-compatible upstream。Task 008 增加 `/v1/embeddings`，让网关覆盖另一个高频 OpenAI-compatible 基础接口。

## 范围

实现：

- `POST /v1/embeddings`。
- OpenAI-compatible embeddings request/response 类型。
- provider interface 增加 `CreateEmbedding`。
- fake provider 返回稳定 embedding。
- openai-compatible provider 转发 `/embeddings`。
- 复用现有模型路由、鉴权、超时、限流、错误响应和结构化日志。
- smoke test 覆盖 embeddings。

暂不实现：

- base64 embedding 解码。
- token 数组的深度校验。
- embeddings 专用 provider 能力声明。
- provider 级模型类型约束。

## 验收标准

- fake provider embeddings 请求返回 `object=list`。
- 缺少 `input` 返回 `400 invalid_request_error`。
- 模型不存在返回 `404 invalid_request_error`。
- provider error 返回 `502 server_error`。
- openai-compatible provider 正确转发到 `/embeddings`。
- `make verify` 和 `make smoke` 在 WSL `Ubuntu-24.04` 中通过。

