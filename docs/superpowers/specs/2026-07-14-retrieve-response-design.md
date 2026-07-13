# 查询已保存 Response API 设计

## 背景

网关已经支持 `POST /v1/responses`，并可在启用 response store 时通过 `store: true` 保存用于 `previous_response_id` 的对话 transcript，但尚不能按 response ID 读取原始 Response。OpenAI Responses API 的 [Retrieve a response](https://developers.openai.com/api/reference/resources/responses/methods/retrieve) 提供 `GET /v1/responses/{response_id}`；SDK 和调用方会用它读取先前保存的完整响应，而不仅是恢复对话上下文。

本任务补充进程内读取能力。它复用现有 TTL、LRU、容量限制和 client 隔离，不调用上游 provider，也不扩大为持久化数据库。

## API 契约

新增受保护路由：

- `GET /v1/responses/{response_id}`
- `HEAD /v1/responses/{response_id}`

`{response_id}` 必须是单个非空路径段。成功的 GET 返回创建该 ID 时保存的完整 `compat.Response` JSON，包括原始 response ID、output item ID、usage、tools、`previous_response_id` 和 `store`。成功的 HEAD 返回相同状态码和 `Content-Type: application/json`，但不返回 body。

本轮不实现 `DELETE /v1/responses/{response_id}`、cancel、input items、`include` 参数或从外部存储恢复。

## 状态与可见性

只有满足以下条件的 Response 可以读取：

- Responses 请求成功完成；
- 请求采用默认 `store: true` 或显式 `store: true`；
- response store 已启用且保存成功；
- 记录未过期、未被容量策略淘汰；
- 当前鉴权 client 与创建记录的 client 相同。

未知 ID、`store: false`、store 禁用、过期、淘汰和其他 client 的记录统一返回 `404 invalid_request_error`，错误参数为 `response_id`。这种统一行为避免泄漏某个 ID 是否曾由其他 client 创建。

读取不会调用 provider，不检查 provider health，也不会重新执行模型推理。

## 存储表示

采用在现有 `responsestore.Record` 中增加 `json.RawMessage` 的方案，同时保存 transcript 和完整 Response JSON：

- transcript 继续服务 `previous_response_id`；
- Response JSON 服务读取端点，并原样保留返回字段和值；
- byte slice 在写入和读取时深拷贝，调用方不能修改 store 内部状态；
- 单条记录和全局 byte 限额同时计算 transcript 编码长度与 Response JSON 长度。

不使用 transcript 重建 Response，因为重建会丢失 output item ID、usage、tools 等信息。也不直接保存 `*compat.Response`，因为其中的 `any` 和嵌套 slice 需要复杂且容易遗漏的深拷贝；JSON bytes 是更小、更明确的边界。

`Store.Get(id, client, model)` 保持现有 continuation 语义。新增 `GetByID(id, client)`，它执行相同的 enabled、存在性、TTL、client 和 LRU 检查，但不要求调用方预先知道 model。两种读取复用一个内部实现，确保 miss 计数只增加一次。

## 写入时机

非流式请求在返回前完成以下顺序：

1. 构造最终 `compat.Response`；
2. 确定 `store` 字段和 `previous_response_id`；
3. 将最终 Response 编码为 JSON；
4. 将 transcript 与 Response JSON 作为同一条记录写入 store；
5. 写审计事件和 HTTP 响应。

若记录超过容量限制，沿用现有 `response context is too large` 错误。其他保存失败继续返回稳定的 500，不向调用方返回一个声称已保存但实际无法读取的 response。

流式请求在成功生成 `response.completed` 的最终 Response 后保存相同对象。保存仍发生在完整 stream 完成后；中途失败、客户端取消或 provider 错误不产生可读取记录。流式保存失败发生在 SSE 已发送后，无法更改 HTTP 状态，因此记录 warning，且该 ID 后续读取为 404，沿用现有流式存储失败策略。

## 路由与观测

`internal/routes` 增加规范路径 `/v1/responses/{response_id}`。具体实例（例如 `/v1/responses/resp_123`）必须：

- 规范化为模板路径，避免日志和 metrics 使用真实 ID 形成高基数标签；
- 对 GET、HEAD 返回 allowed；
- 对其他方法返回 JSON 405 和 `Allow: GET, HEAD`；
- 继续被鉴权中间件视为非 public route。

动态匹配仅接受 `/v1/responses/` 后一个非空路径段，不改变 `/v1/responses` POST 和其他精确路由的行为。

读取本身不产生 model invocation audit 事件；通用 request log 和 HTTP metrics 继续记录规范化路径。这样避免把已保存响应内容复制到新的 agent audit 事件中。

## 测试策略

按 TDD 覆盖：

- store 对 Response JSON 的深拷贝和 byte 统计；
- `GetByID` 的命中、过期、未知 ID 和 client 隔离；
- route metadata 的规范化、GET/HEAD、405 和非法路径形状；
- 非流式 `store: true` 返回后可读取，读取值与原响应语义一致；
- `store: false`、store 禁用、未知 ID 和其他 client 统一 404；
- HEAD 返回 JSON content type 且 body 为空；
- 流式最终 `response.completed.response` 可被读取，字段和值一致；
- 未鉴权请求返回 401，读取不触发 provider 调用；
- 现有 `previous_response_id` continuation 行为不回归。

最终在 WSL `Ubuntu-24.04` 中执行 `make verify`。

## 文档同步

实现时同步更新：

- `openai-compatible-proxy-spec.md`
- `architecture/overview.md`
- `README.md`
- `CHANGELOG.md`
- `tasks/164-retrieve-response.md`
