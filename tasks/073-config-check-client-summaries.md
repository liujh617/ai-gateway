# Task 073 - Config check gateway client summaries

## 背景

Task 070-072 已经让 gateway client 承载非敏感名称、模型白名单和限流覆盖。配置自检此前只输出 key 数量、provider 和 model 摘要，排查 client 配置时还需要人工打开配置文件核对。

## 目标

- 在 `check-config` 报告中输出 gateway client 摘要。
- 摘要只包含非敏感 `name`、`models` 白名单和 `rate_limit.requests_per_minute` 覆盖。
- 保持只输出 gateway API key 数量，不输出任何 gateway API key 明文。
- 文档同步说明自检输出包含 gateway client 摘要。

## 验收

- `config.Check` 报告包含 `GatewayClients`。
- 报告不会泄露 gateway API key 或 upstream API key。
- 有模型白名单和限流覆盖的 client 可以在报告中看到对应摘要。

## 状态

Done.
