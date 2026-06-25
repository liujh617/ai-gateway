# Task 035: Route Registration Constants

## 状态

Done

## 背景

Task 033 将已知路由和允许 method 集中到 `internal/routes`，但 HTTP mux 注册仍然直接写字符串。这样新增或修改路由时，mux 注册与 route metadata 仍可能发生漂移。

## 范围

实现：

- 在 `internal/routes` 中导出已知 path 常量。
- 在 `internal/routes` 中提供 HTTP mux pattern helper。
- HTTP server mux 注册改为使用共享 route path 常量。
- route metadata 定义复用同一组 path 常量。
- 补充 routes 包测试。
- 同步 architecture overview。

暂不实现：

- 由 route metadata 自动生成 handler 注册。
- 自动校验 handler 覆盖所有 route metadata。
- 动态路由或路由模板。

## 验收标准

- mux 注册不再手写 route path 字符串。
- 405、metrics、access log 和 mux 注册复用同一组 path 常量。
- 现有 HTTP 行为不变。
- `make verify`、`make build`、`make check-config-examples` 和 `make smoke` 在 WSL `Ubuntu-24.04` 中通过。
