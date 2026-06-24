# Task 014: CI Verification

## 状态

Done

## 背景

项目已经有稳定的本地验证入口，但提交后还需要自动化验证来防止回归。Task 014 增加 CI 配置，保持 CI 与 WSL 本地 `make verify` 一致。

## 范围

实现：

- GitHub Actions CI。
- Gitee Workflow CI。
- CI 文档。
- README 增加 CI 文档入口。
- CI 执行 `make verify`。
- CI 执行 `make build`。
- CI 可选执行 `make docker-build`。

暂不实现：

- 镜像推送。
- release automation。
- 真实 provider 集成测试。
- secret 注入。

## 验收标准

- CI 配置使用 Go 1.22。
- CI 核心验证与本地 `make verify` 一致。
- CI 不依赖真实 API key。
- `make verify` 和 `make smoke` 在 WSL `Ubuntu-24.04` 中通过。

