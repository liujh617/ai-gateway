# Task 004: 本地验证与开发体验

## 状态

Done

## 背景

Task 003 已经完善了配置校验和健康检查。Task 004 将常用开发命令和本地服务验证流程固化下来，让新协作者可以快速运行、测试和验证网关。

## 范围

实现：

- `Makefile` 开发入口。
- `.gitignore`。
- `config.local.example.json`。
- `docs/local-verification.md`。
- `scripts/smoke-fake.sh`。
- README 补充开发命令入口。
- 在 WSL `Ubuntu-24.04` 中完成自动测试和 fake provider curl smoke test。

暂不实现：

- Dockerfile。
- CI pipeline。
- 自动启动真实 upstream。
- 性能压测脚本。

## 验收标准

- `make verify` 可以完成格式化、单元测试、race 测试和 vet。
- `make run` 可以启动默认 fake provider。
- `make smoke` 可以完成 fake provider curl smoke test。
- `/healthz`、`/v1/models`、非流式 `/v1/chat/completions`、流式 `/v1/chat/completions` 可以通过 curl smoke test。
- 本地真实配置文件名被 `.gitignore` 忽略。
