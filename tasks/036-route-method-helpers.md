# Task 036: Route Method Helpers

## 状态

Done

## 背景

Task 033 到 Task 035 已经把路由路径、允许 method、path normalization 和 mux 注册集中到 `internal/routes`。API 层仍然保留了 method 判断和 `Allow` header 拼接逻辑，虽然数据来自共享元数据，但行为本身仍分散在不同包里。

## 范围

实现：

- 在 `internal/routes` 中提供 method allowed 判断 helper。
- 在 `internal/routes` 中提供 `Allow` header helper。
- API 的 405 middleware 改为只调用 routes helper。
- 删除 API 层本地 method 判断函数。
- 补充 routes 包测试。
- 同步 architecture overview。

暂不实现：

- OPTIONS/CORS 预检。
- 自动生成路由注册表。
- 对外暴露 route metadata API。

## 验收标准

- 405 判断复用 `internal/routes` 中的 method helper。
- `Allow` header 由 `internal/routes` 基于同一份 metadata 生成。
- 已知路由错误 method 的响应行为不变。
- `make verify`、`make build`、`make check-config-examples` 和 `make smoke` 在 WSL `Ubuntu-24.04` 中通过。
