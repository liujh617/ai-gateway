# Task 002: 接入 OpenAI-compatible upstream provider

## 状态

Done

## 背景

Task 001 已经完成 fake provider 版本的 `/v1/chat/completions`。Task 002 将网关推进到真实代理能力：通过配置接入一个 OpenAI-compatible upstream endpoint，并支持非流式与 SSE 流式转发。

## 范围

实现：

- OpenAI-compatible provider adapter。
- `POST /chat/completions` 非流式上游转发。
- `POST /chat/completions` SSE 流式上游转发。
- upstream error 转换为 OpenAI-compatible error。
- JSON 配置文件加载。
- 模型映射配置。
- 保留无配置时的 fake provider 开发模式。
- 使用 `httptest.Server` 覆盖 provider 集成测试。

暂不实现：

- YAML 配置解析。
- 多 provider fallback。
- retry 策略。
- upstream `/models` 动态同步。
- 成本统计。

## 配置示例

```json
{
  "addr": "127.0.0.1:8080",
  "api_key": "test-gateway-key",
  "providers": {
    "openai": {
      "type": "openai-compatible",
      "base_url": "https://api.openai.com/v1",
      "api_key_env": "OPENAI_API_KEY",
      "timeout_seconds": 60
    }
  },
  "models": {
    "gpt-4o-mini": {
      "provider": "openai",
      "upstream_model": "gpt-4o-mini"
    }
  }
}
```

启动：

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/open-ai-gateway -- bash -lc "GATEWAY_CONFIG=config.json go run ./cmd/gateway"
```

## 验收标准

- 无配置文件时仍可使用 fake provider 启动。
- 配置 `openai-compatible` provider 后，请求会转发到上游 `/chat/completions`。
- 非流式响应保持 OpenAI-compatible response。
- 流式响应逐 chunk 转发，并以 `[DONE]` 结束。
- upstream 4xx/5xx 被转换为网关错误响应。
- `go test ./...`、`go test -race ./...`、`go vet ./...` 在 WSL `Ubuntu-24.04` 中通过。

