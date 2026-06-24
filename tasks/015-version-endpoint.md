# Task 015: Version Endpoint

## 状态

Done

## 背景

部署后需要快速确认运行中的网关版本、commit 和构建时间。Task 015 增加 `/version`，并通过 `ldflags` 在二进制和 Docker 镜像构建时注入构建元数据。

## 范围

实现：

- `GET /version`。
- `/version` 不需要 Bearer token。
- `/version` 不参与 rate limit。
- `internal/version` 构建信息包。
- `make build` 注入 version、commit、build time。
- `make docker-build` 通过 build args 注入 version、commit、build time。
- smoke test 覆盖 `/version`。
- 文档同步。

暂不实现：

- SemVer release automation。
- changelog 生成。
- Git tag 自动发布。

## 验收标准

- 默认 `/version` 返回 `version=dev`。
- 未注入 commit 时返回 `commit=unknown`。
- `/version` 无 Authorization header 可访问。
- `/version` 不受 rate limit 影响。
- `make verify`、`make build` 和 `make smoke` 在 WSL `Ubuntu-24.04` 中通过。

