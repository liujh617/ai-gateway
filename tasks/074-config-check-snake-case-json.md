# Task 074 - Config check snake_case JSON

## 背景

`check-config` 是面向运维和 CI 的机器可读输出。此前报告直接使用 Go 导出字段名，输出类似 `GatewayAPIKeyCount`，不符合配置文件中已有的 snake_case 风格，也不利于后续脚本稳定解析。

## 目标

- 为 `CheckReport` 及其摘要结构补充稳定 JSON tag。
- `check-config` 输出使用 snake_case 字段名。
- 保持报告不输出 gateway API key 或 upstream API key 明文。
- 文档同步说明自检输出字段风格。

## 验收

- 报告包含 `gateway_api_key_count`、`gateway_clients`、`provider_count`、`api_key_env_set` 等字段。
- 报告不再出现 Go PascalCase 字段名。
- 既有配置自检和示例配置验证继续通过。

## 状态

Done.
