# Task 072 - Gateway client model allowlists

## 背景

Task 070/071 已经让 gateway client 成为日志、metrics 和限流的稳定归因维度。下一步需要支持按 client 控制可见模型，避免不同调用方默认看到或调用全部模型。

## 目标

- 在 `api_clients` 中支持可选 `models` 白名单。
- `models` 未配置或为空时，该 client 可访问全部模型。
- `GET /v1/models` 只返回当前 client 可见的模型。
- `/v1/chat/completions` 和 `/v1/embeddings` 访问不可见模型时返回与模型不存在一致的 `404 invalid_request_error`。
- provider adapter 不感知 client 权限；权限检查保留在 API 层。
- 配置校验、JSON Schema 和文档同步更新。

## 验收

- 有白名单的 client 只能看到白名单模型。
- 无白名单的 client 仍可看到全部模型。
- 访问不在白名单内的模型不会调用 provider。
- `api_clients[].models` 引用不存在模型、重复模型或空模型名时配置校验失败。

## 状态

Done.
