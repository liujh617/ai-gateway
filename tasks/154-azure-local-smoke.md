# Task 154 - Azure OpenAI Local Smoke

## 状态

Done.

## 背景

Azure OpenAI provider 已有单元测试，但 CI 尚未通过真实 gateway 进程验证配置加载、模型路由、
deployment endpoint 和三类 API 的完整链路。

## 变更

- 新增 Go 标准库实现的本地 Azure fake upstream。
- 新增无需凭据的 `scripts/smoke-azure.sh`。
- 覆盖非流式 chat、SSE chat 和 embeddings。
- 严格验证 Azure path、`api-version`、`api-key`，并确认不发送 `Authorization`。
- 新增 `make smoke-azure` 并接入 `make release-check`。
- 同步 CI 与本地验证文档。

## 验收

- `go test ./internal/testupstream/azurefake -count=1`
- `make smoke-azure`
- `make release-check`
