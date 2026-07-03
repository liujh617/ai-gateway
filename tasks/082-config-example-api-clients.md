# Task 082 - Config example API clients

## 状态

Done.

## 背景

Task 070 到 Task 072 已经支持 `api_clients`、per-client 限流和模型白名单，后续任务又补充了 client 维度观测。兼容契约和 README 已说明这些能力，但通用 `config.example.json` 仍只展示旧的单 `api_key` 写法，不利于用户直接复制新配置形态。

## 目标

- 在 `config.example.json` 中展示 `api_clients`。
- 示例 client 包含模型白名单和 per-client rate limit。
- 保留旧 `api_key` 占位值用于兼容说明。

## 验收

- `config.example.json` 通过配置自检。
- WSL `Ubuntu-24.04` `make check-config-examples` 通过。
- WSL `Ubuntu-24.04` `make verify` 通过。
