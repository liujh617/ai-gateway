# CI

本文记录 `open-ai-gateway` 的自动化验证入口。

## Local Equivalent

CI 的核心验证与本地 WSL `Ubuntu-24.04` 保持一致：

```bash
make release-check
```

其中 `make release-check` 包含：

- `gofmt -w cmd internal`
- `make check-line-endings`
- `make test-line-endings`
- `go test ./...`
- `go test -race ./...`
- `go vet ./...`
- `make check-config`
- `make check-config-examples`
- `make build`
- `make smoke`
- `make smoke-rate-limit`
- `make smoke-azure`
- `make smoke-deepseek-skip`

`make check-config` 会验证默认运行配置，`make check-config-examples` 会校验仓库内示例配置可被当前配置加载器接受，并执行一次配置自检。`make smoke` 使用 fake provider 启动本地服务，验证核心 HTTP 契约。`make smoke-rate-limit` 使用临时 fake 配置验证 gateway 限流响应。`make smoke-deepseek-skip` 强制清空 `DEEPSEEK_API_KEY`，只验证真实 provider smoke 的无 key 跳过路径。

`make check-line-endings` 验证所有受版本控制的 shell 脚本使用 LF；`make test-line-endings`
在临时 Git 仓库中验证检查器接受 LF 正例并拒绝 CRLF 反例。

## GitHub Actions

配置文件：

```text
.github/workflows/ci.yml
```

Job:

- `verify`: 设置 Go 1.22，运行 `make release-check`。
- `docker`: 运行 `make docker-build`。

## Gitee Workflow

配置文件：

```text
.gitee/workflows/ci.yml
```

当前内容与 GitHub Actions 保持一致。若 Gitee 工作流环境不支持某些 action，可以后续替换为 Gitee 官方 checkout/setup-go 动作，或使用预装 Go 的 runner。

## Notes

- CI 不使用真实 upstream API key。
- CI 不运行真实 provider 集成测试。
- Docker build 需要 runner 能访问基础镜像。
- `make smoke` 使用 fake provider 和本地端口，只覆盖无需真实 API key 的运行时契约。
- `make smoke-rate-limit` 使用 fake provider 和本地端口验证 `429`、`Retry-After` 和限流 metrics。
- `make smoke-azure` 启动本地 Go fake Azure upstream，通过真实 gateway 进程验证 deployment path、`api-version`、`api-key`、不发送 `Authorization`，以及 chat、SSE 和 embeddings；不使用真实凭据且不访问外网。
- `make smoke-deepseek-skip` 不访问 DeepSeek，只保证脚本在无 key 环境中安全跳过。
