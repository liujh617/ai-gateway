# CI

本文记录 `open-ai-gateway` 的自动化验证入口。

## Local Equivalent

CI 的核心验证与本地 WSL `Ubuntu-24.04` 保持一致：

```bash
make verify
make check-config-examples
make build
```

其中 `make verify` 包含：

- `gofmt -w cmd internal`
- `go test ./...`
- `go test -race ./...`
- `go vet ./...`

`make check-config-examples` 会校验仓库内示例配置可被当前配置加载器接受，并执行一次配置自检。

## GitHub Actions

配置文件：

```text
.github/workflows/ci.yml
```

Job:

- `verify`: 设置 Go 1.22，运行 `make verify`、`make check-config-examples` 和 `make build`。
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
- 本地 smoke test 仍通过 `make smoke` 执行，不默认放入 CI，避免端口占用导致偶发失败。
