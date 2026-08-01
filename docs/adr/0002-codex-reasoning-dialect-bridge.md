# ADR 0002：Codex reasoning dialect bridge

## 状态

Accepted，2026-08-01。

## 背景

Codex Responses 客户端以 reasoning item 携带不透明状态，DeepSeek thinking mode 则要求工具调用续接时原样回传 assistant `reasoning_content`。简单丢弃字段会导致 DeepSeek 拒绝第二轮请求；进程内 cache 又无法支持 `store:false`、重启或多实例切换。

## 决策

采用 normalized conversation IR + provider dialect adapter。Responses wire codec、IR 校验、DeepSeek 映射和 HTTP transport 保持独立。DeepSeek reasoning 使用 AES-256-GCM envelope 返回给 Codex，AAD 绑定 audience、client 和 external model，payload 绑定 dialect/provider/upstream model；带活动 tool call 的 replay 固定到该路由。

启用 replay 的配置必须在启动时具备有效 active key、可选 previous keys 和声明 replay 能力的 dialect，否则拒绝启动。审计实现、配置和 JSONL schema 本期不变。

## 后果

- `store:false` 可跨重启和共享 key 的多实例续接，不依赖服务端 reasoning cache。
- replay 期间不能跨 provider fallback；可用性让位于语义完整性和安全性。
- key 轮换必须保留旧 key 至少一个 TTL。
- Kimi、GLM 等模型通过新增 dialect 接入，无需修改 Responses handler。

未采用 provider-specific Responses handler、配置化字段重命名引擎、明文/Base64 envelope 或仅进程内缓存，因为这些方案分别造成重复逻辑、无法表达有状态语义、缺少保密完整性或无法跨实例。
