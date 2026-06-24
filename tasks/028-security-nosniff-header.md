# Task 028: Security Nosniff Header

## 状态

Done

## 背景

网关会返回 JSON、SSE 和 Prometheus metrics 等明确媒体类型。为了减少浏览器或代理对响应内容做 MIME sniffing 的风险，所有响应都应带上基础安全响应头 `X-Content-Type-Options: nosniff`。

## 范围

实现：

- 新增安全响应头 middleware。
- 所有正常响应带 `X-Content-Type-Options: nosniff`。
- 错误响应也带 `X-Content-Type-Options: nosniff`。
- 保持现有 `Content-Type` 行为不变。
- 同步 API spec、架构说明和部署文档。

暂不实现：

- CORS header。
- CSP/HSTS 等面向浏览器页面的策略。
- 可配置安全 header。

## 验收标准

- `/healthz` 响应包含 `X-Content-Type-Options: nosniff`。
- `/metrics` 响应包含 `X-Content-Type-Options: nosniff`。
- 鉴权错误响应包含 `X-Content-Type-Options: nosniff`。
- `make verify`、`make build`、`make check-config-examples` 和 `make smoke` 在 WSL `Ubuntu-24.04` 中通过。
