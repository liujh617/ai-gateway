# Task 152 - Azure OpenAI Provider

## 背景

0.1.0 已支持标准 OpenAI-compatible upstream provider。Azure OpenAI 使用 deployment endpoint 和 `api-version` query，与标准 `/v1/chat/completions` 路径不同，需要独立 provider adapter。

## 变更

- 新增 `azure-openai` provider type。
- 新增 Azure OpenAI deployment endpoint 构造。
- 使用 `api-key` header 发送上游 API key。
- 支持 chat completions、streaming chat completions 和 embeddings。
- 新增 `providers.<name>.api_version` 配置和 check-config summary。
- 同步 schema、示例配置、README、兼容契约和架构文档。

## 验证

- `go test ./internal/provider/httpx ./internal/provider/openai ./internal/provider/azureopenai -count=1`
- `go test ./internal/config ./cmd/gateway -count=1`
- `make verify`
- `make check-config-examples`
