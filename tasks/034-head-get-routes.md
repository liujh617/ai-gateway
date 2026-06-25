# Task 034: HEAD Support for GET Routes

## 状态

Done

## 背景

Task 033 将已知路由和允许 method 集中到 `internal/routes`。共享元数据中 GET 路由同时允许 `HEAD`，但部分 handler 仍会写 body。虽然真实 HTTP server 通常会处理 HEAD body，测试和 handler 语义应显式一致。

## 范围

实现：

- `HEAD /v1/models` 返回状态码和响应头，不写 body，且仍需要 Bearer token。
- `HEAD /metrics` 返回状态码和响应头，不写 body，且不需要 Bearer token。
- `HEAD /version` 返回状态码和响应头，不写 body，且不需要 Bearer token。
- 保持 `/healthz` 和 `/readyz` 现有 HEAD 行为不变。
- 同步 API spec 和 architecture overview。

暂不实现：

- 为 POST 路由支持 HEAD。
- OPTIONS/CORS 预检。
- 单独的 HEAD 路由注册生成器。

## 验收标准

- 所有 GET 路由允许的 HEAD 行为与共享 route metadata 一致。
- HEAD 响应没有 body。
- 受保护路由的 HEAD 请求仍先执行鉴权。
- `make verify`、`make build`、`make check-config-examples` 和 `make smoke` 在 WSL `Ubuntu-24.04` 中通过。
