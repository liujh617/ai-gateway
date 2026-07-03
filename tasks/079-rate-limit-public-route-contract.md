# Task 079 - Rate limit public route contract

## 状态

Done.

## 背景

路由元数据已经集中维护公开路由属性，鉴权和限流都会通过 `routes.IsPublicPath` 跳过公开路由。当前实现中 `/healthz`、`/readyz`、`/version` 和 `/metrics` 都不参与限流，但兼容契约的限流章节只写了 `/healthz`，容易让运维误以为其他探针和 metrics 会被业务限流影响。

## 目标

- 同步更新兼容契约中的限流约束。
- 明确所有公开路由都不参与 gateway rate limiter。
- 不改变运行时代码行为。

## 验收

- `openai-compatible-proxy-spec.md` 明确列出 `/healthz`、`/readyz`、`/version` 和 `/metrics` 不参与限流。
- WSL `Ubuntu-24.04` `make verify` 通过。
