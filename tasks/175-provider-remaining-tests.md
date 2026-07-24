# Task 175 - Provider Remaining Method Tests

## 背景

openai 和 azureopenai 提供者仍有 4 个方法零测试覆盖（每提供者），共 8 个方法：

| 方法 | openai | azureopenai |
|---|---|---|
| CreateCompletion | 0% | 0% |
| StreamCompletion | 0% | 0% |
| CreateImage | 0% | 0% |
| CreateModeration | 0% | 0% |

## 变更

- `internal/provider/openai/openai_test.go`：4 组测试（请求转发、错误映射）
- `internal/provider/azureopenai/azureopenai_test.go`：4 组测试
- `internal/provider/httpx/httpx_test.go`：CompletionStream 测试

## 验证

- `go test ./internal/provider/openai ./internal/provider/azureopenai ./internal/provider/httpx -count=1`
- 目标覆盖率：openai 54% → 65%+，azureopenai 48% → 60%+
