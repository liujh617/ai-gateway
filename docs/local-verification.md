# Local Verification

本文记录如何在 WSL `Ubuntu-24.04` 中对 `open-ai-gateway` 做本地开发验证。

## 环境

从 Windows PowerShell 执行：

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/open-ai-gateway
```

之后在 WSL bash 中运行本页命令。

## 基础验证

```bash
make verify
```

等价于：

```bash
gofmt -w cmd internal
go test ./...
go test -race ./...
go vet ./...
```

## 配置自检

默认 fake provider 配置：

```bash
make check-config
```

指定配置文件：

```bash
GATEWAY_CONFIG=config.local.json make check-config
```

也可以直接调用子命令：

```bash
go run ./cmd/gateway check-config
```

自检输出包含 listen addr、gateway client、server runtime、log、rate limit、provider health、provider/model 摘要和 warning，不包含 API key 明文。

校验仓库示例配置：

```bash
make check-config-examples
```

配置 schema 位于：

```text
schema/config.schema.json
```

## Fake Provider 服务验证

自动 smoke test：

```bash
make smoke
```

Responses API 文本兼容 smoke：

```bash
make smoke-responses
```

该命令验证非流式 response envelope、typed SSE 事件和不发送 `[DONE]` 的契约，不需要真实凭据或外部网络。

限流 smoke test：

```bash
make smoke-rate-limit
```

DeepSeek smoke 的无 key 跳过路径：

```bash
make smoke-deepseek-skip
```

Azure OpenAI 本地 fake upstream smoke：

```bash
make smoke-azure
```

该命令不需要 Azure 凭据，也不会访问外网。它会通过真实 gateway 进程验证非流式 chat、SSE chat 和 embeddings。

手工验证：

启动网关：

```bash
make run
```

健康检查：

```bash
curl -sS http://127.0.0.1:8080/healthz
curl -I http://127.0.0.1:8080/healthz
```

Readiness：

```bash
curl -sS http://127.0.0.1:8080/readyz
curl -I http://127.0.0.1:8080/readyz
```

Version：

```bash
curl -sS http://127.0.0.1:8080/version
```

Metrics：

```bash
curl -sS http://127.0.0.1:8080/metrics
```

Common metrics name check:
```bash
curl -sS http://127.0.0.1:8080/metrics | grep -E 'open_ai_gateway_(http_requests_total|tokens_total|token_cost_usd_total|rate_limit_rejections_total|provider_circuit_open_total|provider_fallbacks_total|provider_health_status)'
```

Rate limit, fallback, and circuit-open metric series appear only after the corresponding behavior has happened.

模型列表：

```bash
curl -sS http://127.0.0.1:8080/v1/models \
  -H "Authorization: Bearer test-gateway-key"
```

非流式 chat completions：

```bash
curl -sS http://127.0.0.1:8080/v1/chat/completions \
  -H "Authorization: Bearer test-gateway-key" \
  -H "Content-Type: application/json" \
  -d '{"model":"test-model","messages":[{"role":"user","content":"hello"}]}'
```

流式 chat completions：

```bash
curl -N http://127.0.0.1:8080/v1/chat/completions \
  -H "Authorization: Bearer test-gateway-key" \
  -H "Content-Type: application/json" \
  -d '{"model":"test-model","stream":true,"messages":[{"role":"user","content":"hello"}]}'
```

Embeddings：

```bash
curl -sS http://127.0.0.1:8080/v1/embeddings \
  -H "Authorization: Bearer test-gateway-key" \
  -H "Content-Type: application/json" \
  -d '{"model":"test-model","input":"hello"}'
```

## Agent 审计 JSONL 验证

审计模式默认关闭。启用后会把完整请求体、响应体、流式 chunk、错误响应和 embedding 数据写入本地 JSONL 文件，仅用于本机研究自己的第三方 agent 流量。

```bash
GATEWAY_AUDIT_ENABLED=1 GATEWAY_AUDIT_PATH=tmp/agent-audit.jsonl GATEWAY_AUDIT_MAX_FILE_BYTES=1048576 make smoke
tail -n 5 tmp/agent-audit.jsonl
```

查看不含完整 body 的摘要：

```bash
go run ./cmd/gateway audit-inspect tmp/agent-audit.jsonl
```

也可以在手工请求中加入 trace header：

```bash
curl -sS http://127.0.0.1:8080/v1/chat/completions \
  -H "Authorization: Bearer test-gateway-key" \
  -H "Content-Type: application/json" \
  -H "X-Agent-Trace-Id: local-agent-run-1" \
  -d '{"model":"test-model","messages":[{"role":"user","content":"hello"}]}'
```

## OpenAI-compatible 本地上游

复制示例配置：

```bash
cp config.local.example.json config.local.json
```

按本地 upstream 修改：

- `providers.local.base_url`
- `providers.local.api_key`
- `models.local-model.upstream_model`

启动：

```bash
GATEWAY_CONFIG=config.local.json make run
```

请求时使用配置中的对外模型名：

```bash
curl -sS http://127.0.0.1:8080/v1/chat/completions \
  -H "Authorization: Bearer test-gateway-key" \
  -H "Content-Type: application/json" \
  -d '{"model":"local-model","messages":[{"role":"user","content":"hello"}]}'
```

## 交付记录

提交或交付时记录实际执行过的命令：

```text
Verified in WSL Ubuntu-24.04:
- make verify
- make smoke
- make smoke-rate-limit
```
