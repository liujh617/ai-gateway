# Task 076 - Provider fallback client metrics

## 背景

HTTP、token 和 cost metrics 已经包含 gateway client 标签，但 provider fallback metrics 仍只按 path、model 和 provider 组合聚合。多 client 接入后，无法判断 fallback 是否集中发生在某个 client 或模型白名单路径上。

## 目标

- `open_ai_gateway_provider_fallbacks_total` 增加 `client` 标签。
- fallback 发生时从 request context 读取 gateway client name。
- 未配置 gateway auth 的 fallback 使用 `unconfigured` 标签。
- 文档同步更新 fallback 指标标签。

## 验收

- 多个 gateway client 触发同一 provider fallback 时产生独立 metrics 序列。
- 既有 fallback 计数语义不变。
- WSL `Ubuntu-24.04` 完整验证通过。

## 状态

Done.
