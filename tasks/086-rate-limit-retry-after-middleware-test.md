# Task 086 - Rate limit Retry-After middleware test

## 状态

Done.

## 背景

Task 083 已让 rate limiter 根据固定窗口剩余时间返回 `Retry-After`。此前精确剩余时间只在内部 `allow` 单测中覆盖，API 集成测试为了避免时间抖动只断言 `1..60`。中间件真实响应头缺少可控时间源下的精确测试。

## 目标

- 为 rate limiter 增加内部可注入时间源，默认使用 `time.Now`。
- 中间件测试在可控时间下验证 `Retry-After` 使用剩余窗口秒数。
- 不暴露新的公共配置或 API。

## 验收

- 同一 client 在窗口开始 45 秒后再次请求，中间件返回 `Retry-After: 15`。
- WSL `Ubuntu-24.04` `make verify` 通过。
