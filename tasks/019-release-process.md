# Task 019: Release Process

## 状态

Done

## 背景

项目已经具备版本注入、CI、容器化和配置自检，但还缺少统一发布前检查和变更日志。Task 019 增加手工发布流程和 release preflight。

## 范围

实现：

- `CHANGELOG.md`。
- `docs/release.md`。
- `make release-check`。
- CI 执行 `make release-check`。
- README 增加 release 文档入口。

暂不实现：

- 自动生成 changelog。
- 自动创建 Git tag。
- 自动推送 Docker image。
- GitHub/Gitee release artifact 上传。

## 验收标准

- `make release-check` 串联 verify、默认 config check、config examples、build 和 smoke。
- release 文档说明 SemVer、changelog、build、runtime version 和 git tag。
- CI 调用 release-check。
- `make release-check` 在 WSL `Ubuntu-24.04` 中通过。
