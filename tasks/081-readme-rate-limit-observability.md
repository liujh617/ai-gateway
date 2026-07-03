# Task 081 - README rate limit observability

## 状态

Done.

## 背景

Task 078 和 Task 080 增加了 gateway rate limiter 的专用 rejection metrics 和 `Retry-After` header。兼容契约已经同步，但 README 的配置摘要和第一阶段范围还没有体现这些面向使用者的能力。

## 目标

- README 的 `rate_limit.requests_per_minute` 说明包含超限响应和 `Retry-After`。
- README 的第一阶段范围包含 rate limit rejection metrics。
- 不改变运行时代码行为。

## 验收

- README 能帮助使用者快速了解限流响应和观测能力。
- WSL `Ubuntu-24.04` `make verify` 通过。
