# Trae + DeepSeek Local Proxy

本文记录如何把 Trae 或其他支持 OpenAI-compatible 自定义 endpoint 的客户端接到本地 `open-ai-gateway`，再由网关转发到 DeepSeek API。

## 请求链路

```text
Trae
  -> http://127.0.0.1:8080/v1
  -> open-ai-gateway
  -> https://api.deepseek.com
```

需要区分两类 key：

- 客户端到网关：使用网关 API key，例如 `trae-local-gateway-key`。
- 网关到 DeepSeek：使用 DeepSeek API key，通过 `DEEPSEEK_API_KEY` 环境变量注入。

不要把 DeepSeek API key 填到 Trae 的本地网关配置里。

## 配置

仓库提供示例配置：

```text
config.deepseek.example.json
```

关键字段：

- `addr`: 本地监听地址，默认 `127.0.0.1:8080`。
- `api_key`: 客户端访问网关时使用的 Bearer token。
- `providers.deepseek.base_url`: DeepSeek OpenAI-compatible base URL，使用 `https://api.deepseek.com`。
- `providers.deepseek.api_key_env`: DeepSeek API key 环境变量名，默认 `DEEPSEEK_API_KEY`。
- `models.*.upstream_model`: 转发给 DeepSeek 的真实模型名。

DeepSeek 当前 OpenAI-compatible 模型示例：

- `deepseek-v4-pro`
- `deepseek-v4-flash`

## 启动网关

在 Windows PowerShell 中执行：

```powershell
cd E:\code\open-ai-gateway
$env:DEEPSEEK_API_KEY="<your-deepseek-api-key>"

wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/open-ai-gateway -- bash -lc "DEEPSEEK_API_KEY='$env:DEEPSEEK_API_KEY' GATEWAY_CONFIG=config.deepseek.example.json ./bin/open-ai-gateway"
```

如果尚未构建二进制，先执行：

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/open-ai-gateway -- bash -lc "make build"
```

## 验证

健康检查：

```powershell
curl.exe http://127.0.0.1:8080/healthz
```

模型列表：

```powershell
curl.exe http://127.0.0.1:8080/v1/models `
  -H "Authorization: Bearer trae-local-gateway-key"
```

非流式 chat completions：

```powershell
curl.exe http://127.0.0.1:8080/v1/chat/completions `
  -H "Authorization: Bearer trae-local-gateway-key" `
  -H "Content-Type: application/json" `
  -d "{\"model\":\"deepseek-v4-flash\",\"messages\":[{\"role\":\"user\",\"content\":\"hello\"}]}"
```

流式 chat completions：

```powershell
curl.exe -N http://127.0.0.1:8080/v1/chat/completions `
  -H "Authorization: Bearer trae-local-gateway-key" `
  -H "Content-Type: application/json" `
  -d "{\"model\":\"deepseek-v4-flash\",\"stream\":true,\"messages\":[{\"role\":\"user\",\"content\":\"hello\"}]}"
```

## Trae 客户端设置

如果 Trae 支持自定义 OpenAI-compatible provider，使用以下值：

```text
Base URL: http://127.0.0.1:8080/v1
API Key: trae-local-gateway-key
Model: deepseek-v4-flash
```

如果 Trae 只支持直接填写 DeepSeek provider，则可以先让 Trae 直连 DeepSeek；这种方式不会经过 `open-ai-gateway`，也不会使用网关日志、限流、模型映射和错误转换。

## 注意事项

- ChatGPT Plus 订阅不能作为 DeepSeek 或 OpenAI API key 使用。
- DeepSeek API key 只应提供给网关进程，不应提交到仓库。
- 本地代理建议只监听 `127.0.0.1`，避免暴露到局域网或公网。
- 当前网关第一阶段支持 `/v1/models`、`/v1/chat/completions` 和 `/v1/embeddings`；客户端如果强制依赖其他接口，需要先补齐对应兼容层。
