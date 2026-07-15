# Task 165 - 删除已保存的 Response

## 状态

Done.

## 背景

网关已经支持保存、续接和读取进程内 Response，但调用方无法主动删除不再需要的记录。

## 变更

- 新增需要鉴权的 `DELETE /v1/responses/{response_id}`。
- 按 gateway client 原子删除目标记录，并同步清理 entries、LRU 和 byte 统计。
- 主动删除不计入 expired/capacity eviction 指标。
- 删除只作用于目标 ID，不级联删除后续 Response，也不调用 provider。

## 验收

- 成功返回 OpenAI-compatible `{ "id", "object": "response", "deleted": true }`。
- 未知、过期、已删除、store 禁用和跨 client 删除返回统一 404。
- 删除后目标无法读取或续接，已有 descendant Response 保持可用。
- WSL `make verify` 通过。
