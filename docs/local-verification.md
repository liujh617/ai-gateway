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

自检输出包含 provider/model 摘要和 warning，不包含 API key 明文。

## Fake Provider 服务验证

自动 smoke test：

```bash
make smoke
```

手工验证：

启动网关：

```bash
make run
```

健康检查：

```bash
curl -sS http://127.0.0.1:8080/healthz
```

Readiness：

```bash
curl -sS http://127.0.0.1:8080/readyz
```

Version：

```bash
curl -sS http://127.0.0.1:8080/version
```

Metrics：

```bash
curl -sS http://127.0.0.1:8080/metrics
```

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
- fake provider curl smoke test
```
