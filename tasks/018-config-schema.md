# Task 018: Config Schema

## 状态

Done

## 背景

Task 017 增加了配置自检，但仓库还缺少机器可读的配置契约。Task 018 增加 JSON Schema 和示例配置校验入口，让配置格式更容易被编辑器、CI 和部署流程复用。

## 范围

实现：

- `schema/config.schema.json`。
- 示例配置加载测试。
- `scripts/check-config-examples.sh`。
- `make check-config-examples`。
- CI 增加示例配置校验。
- 文档同步。

暂不实现：

- 在运行时使用 JSON Schema validator。
- 生成 Go 类型。
- schema 自动生成。

## 验收标准

- `config.example.json` 可以被当前配置加载器接受。
- `config.local.example.json` 可以被当前配置加载器接受。
- `make check-config-examples` 通过。
- CI 调用 `make check-config-examples`。
- `make verify`、`make build`、`make check-config-examples` 和 `make smoke` 在 WSL `Ubuntu-24.04` 中通过。

