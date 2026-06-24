# Task 007: 请求观测与结构化日志

## 状态

Done

## 背景

网关需要在不记录 prompt、completion 和密钥的前提下，提供足够的请求级上下文，方便排查模型路由、provider 错误、限流和 streaming 问题。

## 范围

实现：

- access log 增加 request id、method、path、status、latency。
- chat completions 日志增加 external model、provider、upstream model。
- 日志增加 stream 标记。
- 错误响应日志增加 error type 和 error code。
- authentication/rate limit 错误写入日志 error type。
- 日志测试确认不泄露 Authorization header 或 token。

暂不实现：

- metrics endpoint。
- tracing。
- request/response body 审计。
- 成本统计。

## 验收标准

- 正常 chat completion 日志包含 `external_model`、`provider`、`upstream_model`、`stream`。
- 错误请求日志包含 `error_type`。
- stream 请求日志状态码正确。
- 日志不包含 `Authorization` header 或 API key。
- `make verify` 和 `make smoke` 在 WSL `Ubuntu-24.04` 中通过。

