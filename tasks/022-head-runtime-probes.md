# Task 022: HEAD Runtime Probes

## 状态

Done

## 背景

一些负载均衡器、反向代理和运行时探针会使用 `HEAD` 请求检查服务状态，只关心状态码和响应头，不需要 JSON body。网关已经提供 `GET /healthz` 和 `GET /readyz`，但需要显式保证 `HEAD` 请求不会写响应体。

## 范围

实现：

- `HEAD /healthz` 返回 `200`，不需要 Bearer token，不返回 body。
- `HEAD /readyz` 在 ready 时返回 `200`，不需要 Bearer token，不返回 body。
- `HEAD /readyz` 在未加载模型时返回 `503`，不返回 body。
- 保留 `Content-Type: application/json`，与对应 `GET` 探针一致。
- 同步 API spec、部署文档和本地验证文档。

暂不实现：

- `HEAD /metrics`。
- `HEAD /version`。
- 主动探测上游 provider 的 readiness。

## 验收标准

- HEAD 探针不需要鉴权。
- HEAD 探针响应体为空。
- ready 和 not ready 状态码与 `GET /readyz` 一致。
- `make verify`、`make build`、`make check-config-examples` 和 `make smoke` 在 WSL `Ubuntu-24.04` 中通过。
