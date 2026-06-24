# Deployment

本文记录 `open-ai-gateway` 的基础部署方式。标准验证环境仍是 WSL `Ubuntu-24.04`。

## Binary

构建 Linux 静态二进制：

```bash
make build
```

输出：

```text
bin/open-ai-gateway
```

运行：

```bash
GATEWAY_CONFIG=config.local.json ./bin/open-ai-gateway
```

## Docker

构建镜像：

```bash
make docker-build
```

运行默认 fake provider：

```bash
docker run --rm -p 8080:8080 \
  -e GATEWAY_ADDR=0.0.0.0:8080 \
  open-ai-gateway:local
```

运行真实 OpenAI-compatible upstream：

```bash
docker run --rm -p 8080:8080 \
  -e GATEWAY_ADDR=0.0.0.0:8080 \
  -e GATEWAY_CONFIG=/config/config.json \
  -e OPENAI_API_KEY=<your-key> \
  -v "$PWD/config.example.json:/config/config.json:ro" \
  open-ai-gateway:local
```

## Runtime Endpoints

- `GET /healthz`: health check，不需要鉴权。
- `GET /readyz`: readiness check，不需要鉴权。
- `GET /version`: build/version metadata，不需要鉴权。
- `GET /metrics`: Prometheus text metrics，不需要鉴权。
- `GET /v1/models`: OpenAI-compatible models，需要 Bearer token。
- `POST /v1/chat/completions`: chat completions，需要 Bearer token。
- `POST /v1/embeddings`: embeddings，需要 Bearer token。

## Production Notes

- 不要把真实 API key 写入镜像。
- 优先使用环境变量或 secret manager 注入上游 API key。
- 部署前执行 `GATEWAY_CONFIG=/config/config.json open-ai-gateway check-config`。
- 容器监听地址应设置为 `0.0.0.0:8080`。
- 容器环境建议使用 `log.format=json`。
- `write_timeout_seconds` 默认保持 `0`，避免误伤长时间 streaming。
- 反向代理或负载均衡器也应配置合理的 streaming timeout。

## Build Metadata

二进制和镜像支持注入版本信息：

```bash
make build VERSION=0.1.0
make docker-build VERSION=0.1.0 IMAGE=open-ai-gateway:0.1.0
```

运行后查看：

```bash
curl -sS http://127.0.0.1:8080/version
```

## Config Check

容器中执行配置自检：

```bash
docker run --rm \
  -e GATEWAY_CONFIG=/config/config.json \
  -e OPENAI_API_KEY=<your-key> \
  -v "$PWD/config.example.json:/config/config.json:ro" \
  open-ai-gateway:local check-config
```

输出中只展示 API key 是否配置，不展示明文值。

配置 schema 位于：

```text
schema/config.schema.json
```
