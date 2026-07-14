# 删除已保存 Response API 设计

## 背景

网关已经支持创建、保存、续接和读取进程内 Response，但调用方无法主动删除不再需要的记录，只能等待 TTL 或容量淘汰。OpenAI Responses API 的 [Delete a model response](https://developers.openai.com/api/reference/resources/responses/methods/delete) 定义了 `DELETE /v1/responses/{response_id}`，成功时返回包含 response ID、`object: "response"` 和 `deleted: true` 的 JSON。

本任务补充单条 Response 的主动删除能力，复用现有进程内 store、鉴权和 client 隔离。它不调用 provider，不引入 tombstone，也不扩展为持久化或批量删除。

## API 契约

扩展现有受保护路由：

- `GET /v1/responses/{response_id}`
- `HEAD /v1/responses/{response_id}`
- `DELETE /v1/responses/{response_id}`

成功的 DELETE 返回 `200 OK` 和 `Content-Type: application/json`：

```json
{
  "id": "resp_123",
  "object": "response",
  "deleted": true
}
```

响应由 compat 层的稳定类型表示。DELETE 不接收 request body，也不支持查询选项。路径中的 `{response_id}` 继续只接受单个非空路径段。

## 删除范围

删除只作用于目标 ID，不级联删除通过该 Response 创建的后续 Response。

例如 `resp_2` 通过 `previous_response_id: resp_1` 创建后，删除 `resp_1`：

- `resp_1` 无法再读取或作为 `previous_response_id` 使用；
- `resp_2` 仍可读取和续接，因为每条记录已经保存独立的完整 transcript 与 Response JSON；
- store 不维护父子索引，也不遍历其他记录。

删除后不保留 tombstone。Response ID 由网关随机生成，现有 `Put` 只防止当前存量记录发生 ID collision；本任务不增加永久 ID 保留表。

## Store 设计

`internal/responsestore.Store` 增加 client-aware 原子删除方法：

```go
func (s *Store) DeleteByID(id, client string) (MissReason, bool)
```

该方法在同一次 store 锁定期间依次执行：

1. 检查 store 是否启用；
2. 按 ID 查找记录；
3. 检查绝对 TTL，过期时按现有 expired eviction 处理；
4. 检查记录所属 client；
5. 从 entries map、LRU list 和总 byte 统计中移除目标记录。

成功时返回空 reason 和 `true`。失败时返回现有 `MissNotFound`、`MissExpired` 或 `MissClient` 和 `false`，并沿用现有 miss 计数语义。client 不匹配时不删除、不刷新 LRU，也不暴露记录是否属于其他调用方。

现有 `removeLocked` 同时负责结构移除和 eviction 指标。为避免主动删除被错误统计为 expired/capacity eviction，实现时拆出不带指标的底层 `removeEntryLocked`：

- TTL 和容量淘汰调用底层移除后增加相应 eviction 指标；
- `DeleteByID` 只调用底层移除；
- 成功删除减少 `Stats.Entries` 和 `Stats.Bytes`，但不增加任何 eviction reason。

不采用先 `GetByID` 再删除的两步方案，避免校验与删除之间的状态窗口、重复 miss 计数和无意义的 LRU 刷新。不采用 tombstone，避免额外内存、TTL 和指标契约。

## 路由设计

当前 route registry 假定每个路径只有一个非 HEAD 方法，并通过 `Route.RegistrationPattern()` 生成带 method 的 Go 1.22 ServeMux pattern。Responses 实例路径增加 DELETE 后，同一路径存在 GET 和 DELETE 两个非 HEAD 方法。

`RegistrationPattern` 调整为：

- 只有一个非 HEAD 方法时，继续返回 `"METHOD /path"`，现有路由注册行为不变；
- 存在多个非 HEAD 方法时，返回不带 method 的 path pattern；
- methodless handler 仍位于现有鉴权、method validation、日志和 metrics 中间件内部。

`/v1/responses/{response_id}` 的 route methods 固定为 `GET, HEAD, DELETE`。共享 method middleware 在进入 ServeMux 前校验方法，因此 PUT、POST 等方法返回现有 JSON `405 invalid_request_error`，并设置稳定的 `Allow: GET, HEAD, DELETE`。现有 `/v1/responses` POST 精确路径和其他单方法路由不受影响。

具体 response ID 继续规范化为 `/v1/responses/{response_id}`，避免日志和 metrics 产生高基数标签。

## API 处理流程

`handleResponse` 根据 method 分派：

- GET 和 HEAD 保持现有读取行为；
- DELETE 调用 `responseStore.DeleteByID(responseID, client)`；
- 成功时编码 compat 删除响应；
- 失败时统一写入 response-not-found 错误。

DELETE 只访问本地 store，不解析 request body、不调用 model router、不检查 provider health、不调用 provider，也不产生 model invocation audit event。通用 request log 和 HTTP metrics 继续记录规范化路径、DELETE method 和最终 status。

## 错误语义

以下情况统一返回 `404 invalid_request_error`，`param` 为 `response_id`，message 为现有稳定的 `response not found`：

- response ID 不存在；
- Response 已过期或被容量淘汰；
- Response 已被删除，包括重复 DELETE；
- Response 属于其他 gateway client；
- response store 禁用或未配置。

统一 404 避免向调用方泄漏其他 client 的记录存在性。删除不是幂等成功：第一次成功返回删除对象，后续相同 DELETE 返回 404。

缺少或错误 Bearer token 仍由现有鉴权中间件先返回 `401 authentication_error`。不支持的方法返回 `405 invalid_request_error` 和 `Allow: GET, HEAD, DELETE`。

## 测试策略

按 TDD 增加以下覆盖：

### Store

- `DeleteByID` 成功后目标记录无法读取；
- entries 和 bytes 精确减少；
- 主动删除不增加 expired/capacity eviction 指标；
- 未知 ID 返回 `MissNotFound`；
- 过期记录返回 `MissExpired`，并保持现有 expired eviction 统计；
- client 不匹配返回 `MissClient` 且记录保持可读；
- 重复删除返回 `MissNotFound`；
- 并发读取、写入和删除通过 race test。

### Routes

- concrete response path 对 GET、HEAD、DELETE 为 known/allowed；
- `AllowHeader` 返回 `GET, HEAD, DELETE`；
- 多个非 HEAD method 的 route 使用 methodless registration pattern；
- 单方法和 GET/HEAD route 的 registration pattern 不回归；
- 空 ID 和额外路径段继续作为未知路由处理。

### API

- DELETE 成功响应的 status、content type 和 JSON 字段与兼容契约一致；
- 删除后 GET 返回 404；
- 删除后以该 ID 续接返回 404；
- 删除父 Response 后，已经创建的子 Response 仍可读取和续接；
- 未鉴权、未知、store 禁用、cross-client 和重复删除返回预期错误；
- cross-client 删除失败后，原 client 仍能读取记录；
- DELETE 不增加 provider 调用次数；
- GET、HEAD 和现有 Responses 创建/续接行为不回归。

最终在 WSL `Ubuntu-24.04` 中运行 `make verify`。

## 文档同步

实现时同步更新：

- `openai-compatible-proxy-spec.md`
- `architecture/overview.md`
- `README.md`
- `CHANGELOG.md`
- `tasks/165-delete-response.md`
- `docs/trae-deepseek.md` 中已过时的支持端点列表

## 非目标

- 不级联删除 descendant Responses；
- 不实现批量删除、按 client 清空或管理接口；
- 不实现 tombstone、软删除或删除审计存档；
- 不实现 cancel、input items、background mode 或持久化 store；
- 不修改 provider adapter、model router 或 provider health。
