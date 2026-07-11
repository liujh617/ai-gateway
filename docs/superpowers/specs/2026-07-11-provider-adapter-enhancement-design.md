# Provider Adapter Enhancement Design

## 背景

`open-ai-gateway` 0.1.0 已经具备最小可用代理能力：OpenAI-compatible chat completions、streaming、embeddings、models、fallback、provider health、metrics、rate limit 和本地 audit。下一轮功能应优先扩大真实上游接入面，同时保持项目“标准库优先、小而清晰”的方向。

当前 `openai-compatible` provider 假设上游路径为：

- `GET <base_url>/models`
- `POST <base_url>/chat/completions`
- `POST <base_url>/embeddings`

这适合 OpenAI、DeepSeek、Trae local proxy 等 OpenAI-compatible 服务，但 Azure OpenAI 常用路径包含 deployment 和 `api-version` query，例如：

```text
POST /openai/deployments/{deployment}/chat/completions?api-version=...
POST /openai/deployments/{deployment}/embeddings?api-version=...
```

因此 0.1.1 推荐聚焦为：新增 Azure OpenAI provider 支持，而不是先引入完全通用的自定义 headers/query/path 模板系统。

## 目标

- 新增 provider type：`azure-openai`。
- 支持 Azure OpenAI chat completions、streaming chat completions 和 embeddings。
- 继续复用内部 `provider.Provider` 接口，不把 Azure 专属逻辑写入 handler 或 router。
- 保持模型路由语义不变：external model 仍通过 `models.<external>.provider` 和 `upstream_model` 映射到 provider 调用。
- 配置和日志不得输出 Azure API key。
- 同步更新 spec、architecture、README、schema、example config 和 task 文档。

## 非目标

- 不实现 Azure AD / Entra ID token 鉴权。
- 不实现 image、audio、responses、assistants 等更多 OpenAI API。
- 不实现任意 provider path 模板、任意 headers 注入或任意 query 注入。
- 不主动探测 Azure deployment 列表作为 readiness 条件。
- 不新增第三方依赖。

## 配置设计

`ProviderConfig` 增加 Azure 所需字段：

```json
{
  "providers": {
    "azure": {
      "type": "azure-openai",
      "base_url": "https://example.openai.azure.com",
      "api_key_env": "AZURE_OPENAI_API_KEY",
      "api_version": "2024-02-15-preview",
      "timeout_seconds": 60
    }
  },
  "models": {
    "gpt-4o-mini": {
      "provider": "azure",
      "upstream_model": "my-chat-deployment",
      "capabilities": ["chat"]
    },
    "text-embedding-3-small": {
      "provider": "azure",
      "upstream_model": "my-embedding-deployment",
      "capabilities": ["embeddings"]
    }
  }
}
```

字段语义：

- `type`: 新增 `azure-openai`。
- `base_url`: Azure resource endpoint，必须是 `http` 或 `https` URL，不能包含 query 或 fragment，规范化时去除末尾 `/`。
- `api_key` / `api_key_env`: 复用现有上游 key 配置。Azure provider 使用 `api-key` header，不使用 `Authorization: Bearer`。
- `api_version`: 必填，不能包含首尾空白，必须能作为 URL query value 编码。
- `models.<external>.upstream_model`: 对 Azure 表示 deployment name，而不是模型名。为空时仍按现有规则回退为 external model，便于简单配置。

`check-config` 的 provider summary 增加 `api_version`，但不输出 key 明文。

## Provider 设计

新增包：

```text
internal/provider/azureopenai/
  azureopenai.go
  azureopenai_test.go
```

Azure provider 实现同一个 `provider.Provider` 接口：

- `CreateChatCompletion`: `POST /openai/deployments/{deployment}/chat/completions?api-version=<api_version>`
- `StreamChatCompletion`: 同一路径，设置 `stream=true` 并要求 SSE response。
- `CreateEmbedding`: `POST /openai/deployments/{deployment}/embeddings?api-version=<api_version>`
- `ListModels`: 返回 provider 错误或空列表不可取。为了保持 `/v1/models` 来自 gateway config 的当前语义，Azure provider 的 `ListModels` 可以实现为返回 `ErrUnsupported`，但 API 层不应依赖 provider list models 来返回 gateway models。若现有构造路径需要 provider 支持该方法，则返回空切片和 nil，避免影响启动；真实模型列表仍由 router 配置暴露。

请求体处理：

- 复用 compat request/response 类型。
- 转发前把 `model` 改为 deployment name，保持当前 router 已经传入 `upstream_model` 的行为。
- 非流式 chat 清除 `stream=true`。
- unknown request fields 的透传行为保持不变。

Header 处理：

- `api-key: <key>`。
- `User-Agent: open-ai-gateway/<version>`。
- `X-Request-Id` 继续转发。
- JSON 请求设置 `Content-Type: application/json`。
- 非流式请求设置 `Accept: application/json`。
- 流式请求设置 `Accept: text/event-stream`。

响应和错误处理：

- 复用现有 OpenAI-compatible provider 的 JSON 单值校验、Content-Type 校验、SSE parser、timeout normalization 和 upstream error 映射逻辑。
- 为避免复制过多代码，可以先抽取 `internal/provider/openai` 中与 HTTP 响应解析和 SSE 读取无关 provider 类型的通用 helper，放到 `internal/provider/httputil` 或 `internal/provider/openai` 内部共享文件。包名应避免和标准库 `net/http/httputil` 混淆，推荐 `internal/provider/httpx`。
- 抽取范围只限 response decode、content-type check、SSE stream parser、transport timeout normalization；endpoint 构造和 headers 保持各 provider 自己负责。

## 路由与构造

`cmd/gateway` 或现有 provider factory 增加 `azure-openai` 分支：

- 校验 `base_url`。
- 解析 API key。
- 校验 `api_version`。
- 构造 Azure provider。

Router 不需要知道 Azure。它只负责把 external model 映射为 provider name 和 upstream model。

## 文档与兼容契约

需要同步更新：

- `openai-compatible-proxy-spec.md`: provider 类型、Azure endpoint、`api-key` header、`api_version` 配置。
- `architecture/overview.md`: 当前 provider 实现列表新增 Azure OpenAI。
- `README.md`: 配置结构新增 `azure-openai` 和示例链接。
- `schema/config.schema.json`: provider type enum 和 `api_version` 字段。
- `config.azure-openai.example.json`: 新增示例配置，使用明显占位值。
- `tasks/152-azure-openai-provider.md`: 新增任务文档。

## 测试策略

新增单元测试：

- Azure chat non-stream 请求路径、query、headers、body model/deployment 映射。
- Azure streaming chat 请求 `Accept: text/event-stream`，正常 SSE chunk 和 `[DONE]`。
- Azure embeddings 请求路径、query、headers、body。
- Azure upstream JSON error 映射。
- Azure timeout normalization。
- 配置校验：缺少 `api_version`、base_url 带 query/fragment、key 缺失、合法配置通过。
- schema 示例配置测试。

验证命令：

```bash
go test ./internal/provider/azureopenai ./internal/config ./cmd/gateway -count=1
make verify
make check-config-examples
```

最终验证仍按项目约定在 WSL `Ubuntu-24.04` 中执行。

## 风险与取舍

- Azure 的 deployment 名和 OpenAI 的 model 名语义不同。设计明确将 `upstream_model` 解释为 deployment name，避免新增 `deployment` 字段导致 router 复杂化。
- Azure 的错误响应通常接近 OpenAI-compatible，但并不完全保证一致。0.1.1 只做兼容字段保留和默认映射，不把 Azure 内部错误结构泄露到 handler。
- 通用 headers/query/path 模板看似更灵活，但会扩大安全和配置校验面。先实现明确的 `azure-openai` provider 更符合当前小核心路线。

## 验收标准

- 使用 `azure-openai` provider 配置时，chat、streaming chat 和 embeddings 都能转发到 Azure deployment endpoint。
- OpenAI-compatible provider 现有行为不回退。
- 所有新增配置在 `check-config` 和 JSON Schema 中可见且不泄露 API key。
- 文档、任务和示例配置与实现契约一致。
