# Task 164 - 查询已保存的 Response

## 状态

Done.

## 背景

网关可以保存 Responses API 的对话上下文，但调用方还不能按 response ID 读取创建时的完整响应。

## 变更

- 新增需要鉴权的 `GET` 和 `HEAD /v1/responses/{response_id}`。
- 同时保存 continuation transcript 和最终 Response JSON，并纳入现有 TTL、LRU 和 byte 限制。
- 对未知、未保存、过期、淘汰和其他 client 的 ID 返回统一 404。
- 对具体 response ID 路径进行日志和 metrics 规范化，读取不调用 provider。

## 验收

- 普通及流式 `store: true` Response 均可读取，字段和值与创建时一致。
- `store: false`、store 禁用和跨 client 读取返回 `404 invalid_request_error`。
- WSL `make verify` 通过。
