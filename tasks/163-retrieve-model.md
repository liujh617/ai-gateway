# Task 163 - 查询单个模型

## 状态

Done.

## 背景

网关已经通过 `GET /v1/models` 列出当前 client 可见的模型，但尚未支持按模型 ID 查询单个已配置模型。

## 变更

- 新增需要鉴权的 `GET` 和 `HEAD /v1/models/{model}`。
- 执行 gateway client 模型白名单，并避免泄露隐藏模型是否存在。
- 为日志、metrics、方法处理和路由元数据规范化具体模型路径。
- 查询只读取本地配置，不调用或健康检查上游 provider。

## 验收

- 已知且可见的模型返回 OpenAI-compatible 模型元数据。
- 未知模型和 client 隐藏模型返回相同的 `404 invalid_request_error`。
- WSL `make verify` 通过。
