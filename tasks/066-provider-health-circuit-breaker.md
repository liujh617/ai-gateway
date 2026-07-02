# Task 066: Provider Health Circuit Breaker

## 状态

Done

## 背景

Task 063 已支持按模型配置 fallback provider，Task 065 已暴露 fallback 指标。为了避免主 provider 已经持续异常时每个请求仍然先撞一次失败，需要在网关进程内维护轻量 provider health 状态，并在短时间内跳过 unhealthy provider。

## 范围

实现：

- 新增进程内 provider health tracker。
- provider 连续出现可 fallback 错误达到阈值后短暂熔断。
- 非流式 chat completions 和 embeddings 在 provider unhealthy 时跳过该 provider。
- 流式 chat completions 在建连阶段支持跳过 unhealthy provider；SSE 已开始发送后仍不切换 provider。
- provider 成功响应后恢复 healthy；冷却时间结束后允许重新尝试。
- 新增 `provider_health.failure_threshold` 和 `provider_health.cooldown_seconds` 配置。
- 新增 `open_ai_gateway_provider_health_status` metrics。
- 更新兼容契约、架构概览、README、配置示例和 JSON Schema。

暂不实现：

- 跨进程或持久化 provider health 状态。
- 半开状态并发探测限制。
- 按错误类型拆分熔断指标。
- 主动后台健康探测。
- weighted routing。

## 验收标准

- provider 连续可 fallback 错误达到阈值后，后续请求跳过 unhealthy provider 并直接尝试 fallback。
- 冷却时间结束后 provider 会被再次尝试，成功后恢复 healthy。
- embeddings 与非流式 chat completions 都覆盖跳过 unhealthy provider。
- provider health metrics 暴露 `healthy` 和 `unhealthy` 状态。
- 客户端错误类 provider error 不计入健康失败。
- `make verify`、`make build`、`make check-config-examples`、`make smoke` 和无 key 的 `make smoke-deepseek` 在 WSL `Ubuntu-24.04` 中通过。
