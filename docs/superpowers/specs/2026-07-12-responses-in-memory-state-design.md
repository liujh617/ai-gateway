# Responses API 进程内会话状态设计

## 背景与目标

网关已经支持最小文本版 `/v1/responses` 和 function tools，但调用方仍需在每次请求中重发完整历史。Task 159 增加 `previous_response_id`，让调用方只发送本轮新增输入，并由网关在进程内恢复上一响应对应的规范化 Chat transcript。

该能力保持 OpenAI-compatible 语义：响应默认存储；`store: false` 禁止存储当前响应，但不妨碍读取已存储的上一响应；顶层 `instructions`、`tools`、`tool_choice` 和采样参数不会自动继承。历史输入仍会发送给上游，因此仍占用上游 token。

本任务只提供单进程、易于理解的状态能力，不承诺跨实例、跨重启或持久化。

## 方案选择

采用独立的进程内 response store。它无需新增外部依赖，能够保持 provider adapter 无状态，也适合当前小型网关的范围。

未采用的方案：

- 外部持久化存储：可支持多实例和重启恢复，但引入部署、序列化、加密和一致性复杂度，超出当前范围。
- provider 原生 `previous_response_id` 透传：会让行为依赖具体 provider，破坏网关统一兼容契约，也无法稳定支持 fallback。

## 外部 API 契约

`POST /v1/responses` 接受可选字符串字段 `previous_response_id` 和可选布尔字段 `store`。

- `store` 未提供时等同于 `true`。
- `store: true` 时，仅在响应成功完成后存储新响应快照。
- `store: false` 时不存储新响应，但仍可引用一个已经存储的 `previous_response_id`。
- 同一个上一响应可以被并发或顺序引用多次，每次形成独立分支和新的响应 ID。
- continuation 必须使用与上一响应相同的外部模型名称。
- 每一轮都必须重新提供所需的 `instructions`、`tools`、`tool_choice` 和采样参数。

未找到、已过期、已淘汰或属于其他网关客户端的 ID，统一返回 HTTP 404：

```json
{
  "error": {
    "message": "previous response not found",
    "type": "invalid_request_error",
    "param": "previous_response_id"
  }
}
```

统一的 404 防止调用方通过错误差异探测其他客户端的响应 ID。以下情况返回稳定的 HTTP 400 `invalid_request_error`，`param` 均为 `previous_response_id`：

- continuation 的外部模型与上一响应不一致；
- response store 已禁用但请求提供了 `previous_response_id`；
- 合并后的规范化 transcript 超过单响应 4 MiB 上限。

模型不一致的内部 miss 指标单独计数，但对外不得暴露上一响应的模型或所有者。

## 架构与数据流

新增独立包 `internal/responsestore`。API 层持有 store，并通过窄接口使用 `Get` 和 `Put`；router 和 provider adapter 不感知 response state。

每条记录包含：

- response ID；
- 网关客户端身份；
- 请求使用的外部模型名称；
- 规范化 Chat transcript；
- 创建时间、最近访问时间和过期时间；
- transcript JSON 编码后的字节数。

处理 continuation 时：

1. API 层解析请求并识别当前网关客户端。
2. 使用客户端身份和 `previous_response_id` 查询 store。
3. store 在同一临界区内检查 TTL、客户端和模型，成功读取会刷新最近访问时间。
4. API 层把历史 transcript 与本轮 `input` 转换出的消息拼接，再走现有 Responses-to-Chat、router、provider 和 fallback 流程。
5. 非流式响应成功生成后，或流式响应正常发出 `response.completed` 后，将“历史 + 本轮输入 + 本轮模型输出”保存为新响应的独立快照。

快照包括文本消息、function call 和 function call output。它不包括顶层 `instructions`，也不保存或继承工具声明、工具选择和采样控制。fallback 场景保存实际成功响应对应的统一 transcript，但外部模型绑定仍使用请求中的模型名称。

上游错误、超时、客户端取消、流中断或其他 incomplete 结果均不得写入 store，其响应 ID 后续不可引用。

## Store 行为与并发

store 使用互斥锁保护记录表、总字节数和 LRU 元数据，不启动后台清理 goroutine。

`Get` 在锁内完成查找、惰性过期清理、客户端和模型验证以及 LRU 刷新，并返回 transcript 的深拷贝。原始 JSON 等可变字段也必须深拷贝，调用方不能修改 store 内部状态。

`Put` 先在锁外规范化并序列化 transcript、计算字节数；进入锁后按以下顺序处理：

1. 清理所有已过期记录；
2. 若单记录超过 `max_context_bytes`，拒绝写入；
3. 按最久未使用顺序淘汰记录，直到条目数和总字节数均可容纳新记录；
4. 插入新快照并更新计数。

response ID 由现有 ID 生成方式产生；若发生进程内碰撞则重新生成，不能覆盖旧记录。对同一 previous response 的并发读取天然形成相互独立的分支。

指标快照也会触发惰性过期清理。由于没有后台任务，过期记录可能在无请求和无指标抓取时暂时占用内存，但不会在下一次访问后继续可见。

## 配置与资源上限

新增配置：

```json
"response_store": {
  "ttl_seconds": 3600,
  "max_entries": 1000,
  "max_context_bytes": 4194304,
  "max_total_bytes": 67108864
}
```

整个配置段缺省时使用以上默认值并启用 store。任一字段为负数时启动失败；任一字段显式为 `0` 时整个 store 禁用，并输出不含敏感信息的配置警告。`check-config` 输出 store 是否启用以及生效后的限制。配置 schema、示例配置和 README 同步更新。

字节限制按规范化 Chat transcript 的 JSON 编码长度计算。达到容量时先清理过期项，再执行 LRU 淘汰。默认单响应上限为 4 MiB、总容量为 64 MiB、条目上限为 1,000、TTL 为一小时。成功 continuation 会刷新被引用记录的访问时间，但不延长其绝对过期时间；新响应拥有自己的创建时间和完整 TTL。

## 安全、日志与可观测性

状态只保存在进程内存，不写磁盘。运维文档明确说明其中可能包含 prompt、completion 和工具参数等敏感内容，并说明重启丢失、单实例范围和内存上限。

普通访问日志只增加布尔字段 `previous_response`，不得记录 response ID 或完整 transcript。现有审计通道可按其安全策略记录完整 ID，但不得记录 Authorization header。

新增指标：

- `open_ai_gateway_response_store_entries`
- `open_ai_gateway_response_store_bytes`
- `open_ai_gateway_response_store_evictions_total{reason="expired|capacity"}`
- `open_ai_gateway_response_store_misses_total{reason="not_found|expired|client|model"}`

指标不得使用 response ID、客户端身份或模型作为标签。response store 的状态不影响 readiness。

## 测试与验证

`internal/responsestore` 单元测试覆盖：Put/Get、深拷贝、TTL、惰性清理、LRU、条目上限、单记录和总字节上限、分支、ID 碰撞以及并发访问；完整验证包含 race detector。

配置测试覆盖：默认值、合法限制、任一零值禁用、负值拒绝、schema、示例配置和 `check-config` 输出。

API 与集成测试覆盖：

- 非流式两轮文本 continuation；
- 正常 `response.completed` 的流式 continuation；
- 文本、function call 和 function call output 历史回放；
- `instructions`、`tools` 和控制参数不继承；
- `store: false` 不写入，但可以读取已存储的 previous response；
- 客户端隔离、模型不一致和统一错误契约；
- 上游错误、流错误、超时和取消不写入；
- provider fallback 成功后可继续引用；
- 指标、审计和普通日志不泄露敏感状态。

新增离线 `make smoke-responses-state`，覆盖文本 continuation 和 function continuation，并纳入 `make release-check`。最终在 WSL `Ubuntu-24.04` 中执行项目规定的完整验证。

## 文档同步

实现时同步更新：

- Task 159 文档；
- `openai-compatible-proxy-spec.md`；
- `architecture/overview.md`；
- README、配置 schema 和示例配置；
- CI/本地验证文档及 changelog。

## 非目标

本任务不实现：

- `GET` 或 `DELETE /v1/responses/{id}`；
- Conversations API；
- 跨进程共享、持久化、静态加密或重启恢复；
- 后台压缩或后台过期清理；
- 自动继承 `instructions`、tools 或采样设置；
- reasoning Items 或本任务之外的新 Responses Item 类型。
