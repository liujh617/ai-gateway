# Task 038: Route Metadata Registration

## 状态

Done

## 背景

Task 033 到 Task 037 已经把路由路径、允许 method、公开属性、405、`Allow` header、metrics、access log 和鉴权/限流绕过集中到 `internal/routes`。但 HTTP mux 注册仍然逐条调用 `mux.HandleFunc`，虽然使用了共享 path 常量，仍然需要和 route metadata 并行维护。

## 范围

实现：

- 在 `internal/routes` 中提供 `All()`，返回 route metadata copy。
- 在 route metadata 上提供 mux registration pattern helper。
- HTTP server 改为遍历 `routes.All()` 注册 mux handler。
- HTTP server 在 route metadata 缺少 handler 时启动即失败。
- 补充 routes 包测试。
- 同步 architecture overview。

暂不实现：

- 自动从 handler 方法名反射注册路由。
- 动态路由或插件路由。
- 对外暴露 route metadata endpoint。

## 验收标准

- mux 注册由 `internal/routes` metadata 驱动。
- route metadata 返回值不能被调用方修改内部状态。
- GET+HEAD 路由注册为 GET pattern，POST 路由注册为 POST pattern。
- 现有 HTTP 行为不变。
- `make verify`、`make build`、`make check-config-examples` 和 `make smoke` 在 WSL `Ubuntu-24.04` 中通过。
