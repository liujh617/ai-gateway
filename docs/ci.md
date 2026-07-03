# CI

本文记录 `open-ai-gateway` 的自动化验证入口。

## Local Equivalent

CI 的核心验证与本地 WSL `Ubuntu-24.04` 保持一致：

```bash
make release-check
```

其中 `make release-check` 包含：

- `gofmt -w cmd internal`
- `go test ./...`
- `go test -race ./...`
- `go vet ./...`
- `make check-config`
- `make check-config-examples`
- `make build`
- `make smoke`

`make check-config` 会验证默认运行配置，`make check-config-examples` 会校验仓库内示例配置可被当前配置加载器接受，并执行一次配置自检。`make smoke` 使用 fake provider 启动本地服务，验证核心 HTTP 契约。

## GitHub Actions

配置文件：

```text
.github/workflows/ci.yml
```

Job:

- `verify`: 设置 Go 1.22，运行 `make verify`、`make check-config-examples`、`make build` 和 `make release-check`。
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
