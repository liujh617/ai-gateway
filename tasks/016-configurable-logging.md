# Task 016: Configurable Logging

## 状态

Done

## 背景

网关默认使用 text 日志，适合本地开发。容器和生产环境通常需要 JSON 日志，并且不同环境需要不同日志级别。Task 016 增加日志格式和级别配置。

## 范围

实现：

- `log.format` 配置。
- `log.level` 配置。
- 支持 `text` 和 `json`。
- 支持 `debug`、`info`、`warn`、`error`。
- 默认 `text/info`。
- 启动日志输出当前日志配置。
- 示例配置区分生产 JSON 和本地 debug。

暂不实现：

- 日志采样。
- 动态调整日志级别。
- 文件日志输出。

## 验收标准

- 默认日志配置为 `text/info`。
- `json/debug` 配置可以加载。
- 未知日志格式启动失败。
- `make verify`、`make build` 和 `make smoke` 在 WSL `Ubuntu-24.04` 中通过。

