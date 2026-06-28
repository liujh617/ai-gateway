# Task 062: DeepSeek Smoke Test

## 状态

Done

## 背景

Task 061 已经提供 Trae + DeepSeek 的本地代理配置和文档。下一步需要一个可重复执行的真实链路 smoke test：在开发者提供 `DEEPSEEK_API_KEY` 时，启动本地网关并调用 DeepSeek；在没有 key 的环境中明确跳过，避免 CI 或普通本地验证因为外部依赖失败。

## 范围

实现：

- 新增 `scripts/smoke-deepseek.sh`。
- 新增 `make smoke-deepseek`。
- 默认验证 health、readiness、models 和一次非流式 chat completions。
- 通过 `DEEPSEEK_SMOKE_STREAM=1` 可选验证流式 chat completions。
- 更新 Trae + DeepSeek 文档，记录 smoke 脚本用法。

暂不实现：

- 将真实 DeepSeek smoke 纳入默认 `release-check`。
- 在没有 `DEEPSEEK_API_KEY` 时失败。
- 自动配置 Trae 客户端。

## 验收标准

- 未设置 `DEEPSEEK_API_KEY` 时，`make smoke-deepseek` 输出 skip 并成功退出。
- 设置 `DEEPSEEK_API_KEY` 时，脚本启动本地网关并通过 DeepSeek 非流式 chat completions 验证。
- 设置 `DEEPSEEK_SMOKE_STREAM=1` 时，脚本额外验证流式 `[DONE]`。
- `make verify`、`make build`、`make check-config-examples`、`make smoke` 和无 key 的 `make smoke-deepseek` 在 WSL `Ubuntu-24.04` 中通过。
