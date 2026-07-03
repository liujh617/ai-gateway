# Release

本文记录 `open-ai-gateway` 的手工发布流程。

## Versioning

使用 SemVer：

```text
MAJOR.MINOR.PATCH
```

示例：

```text
0.1.0
```

## Preflight

在 WSL `Ubuntu-24.04` 中执行：

```bash
make release-check VERSION=0.1.0
```

`release-check` 会执行：

- `make verify`
- `make check-config`
- `make check-config-examples`
- `make build VERSION=<version>`
- `make smoke`
- `make smoke-rate-limit`
- `make smoke-deepseek-skip`

## Changelog

发布前更新 [CHANGELOG.md](../CHANGELOG.md)：

1. 将 `Unreleased` 中的条目移动到目标版本。
2. 将版本日期改为发布当天日期。
3. 保留新的空 `Unreleased` 小节。

## Build

构建二进制：

```bash
make build VERSION=0.1.0
```

构建镜像：

```bash
make docker-build VERSION=0.1.0 IMAGE=open-ai-gateway:0.1.0
```

## Verify Runtime Version

启动服务后检查：

```bash
curl -sS http://127.0.0.1:8080/version
```

确认：

- `version` 是目标版本。
- `commit` 是发布 commit。
- `build_time` 是 UTC 构建时间。

## Git Tag

```bash
git tag v0.1.0
git push origin v0.1.0
```

## Notes

- 不要在 release artifact 中包含 `config.local.json` 或任何真实 API key。
- Docker 镜像发布前必须确认基础镜像来源符合部署环境要求。
- 当前流程不自动推送镜像，镜像发布由后续任务补充。
