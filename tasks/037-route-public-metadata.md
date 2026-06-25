# Task 037: Route Public Metadata

## 状态

Done

## 背景

Task 033 到 Task 036 已经把路由路径、允许 method、mux 注册、405 判断、`Allow` header、metrics 和 access log 的 path 规则集中到 `internal/routes`。鉴权 middleware 仍然手写公开 runtime 路由列表，新增公开路由时仍可能和共享元数据发生漂移。

## 范围

实现：

- 在 `internal/routes` route metadata 中声明公开路由。
- 提供 `routes.IsPublicPath` helper。
- Auth middleware 改为使用共享 route metadata 判断公开路由。
- Rate limiter middleware 改为使用共享 route metadata 判断公开路由绕过。
- 删除 Auth middleware 中的手写公开路径判断。
- 补充 routes 包测试。
- 同步 architecture overview。

暂不实现：

- 基于调用方或 token scope 的细粒度授权。
- 动态公开路由配置。
- 多租户鉴权策略。

## 验收标准

- `/healthz`、`/readyz`、`/metrics`、`/version` 的公开属性来自 `internal/routes`。
- `/v1/models`、`/v1/chat/completions`、`/v1/embeddings` 仍默认为受保护路由。
- 公开路由和受保护路由的现有鉴权行为不变。
- 公开路由和受保护路由的现有限流行为不变。
- `make verify`、`make build`、`make check-config-examples` 和 `make smoke` 在 WSL `Ubuntu-24.04` 中通过。
