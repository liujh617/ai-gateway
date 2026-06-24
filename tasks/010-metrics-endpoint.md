# Task 010: Metrics Endpoint

## 状态

Done

## 背景

结构化日志适合排障，但服务运行还需要可抓取的聚合指标。Task 010 增加轻量 in-memory HTTP metrics，并通过 `/metrics` 暴露 Prometheus text exposition 格式。

## 范围

实现：

- `GET /metrics`。
- `/metrics` 不需要 Bearer token。
- `/metrics` 不参与限流。
- 记录 HTTP request count。
- 记录 HTTP request duration seconds total。
- 指标标签：`method`、`path`、`status`。
- smoke test 覆盖 `/metrics`。

暂不实现：

- Prometheus client dependency。
- histogram buckets。
- provider 级指标。
- token usage/cost metrics。
- `/metrics` 鉴权配置。

## 验收标准

- `/metrics` 无 Authorization header 返回 `200`。
- 成功和错误请求都会产生对应 status counter。
- `/metrics` 不受 rate limit 影响。
- `make verify` 和 `make smoke` 在 WSL `Ubuntu-24.04` 中通过。

