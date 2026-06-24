# Task 011: Server Runtime Hardening

## 状态

Done

## 背景

网关不能长期使用裸 `http.ListenAndServe`。Task 011 增加 HTTP server 运行时超时和 SIGINT/SIGTERM graceful shutdown，让服务在开发和部署环境中更可控。

## 范围

实现：

- 使用 `http.Server` 替代裸 `http.ListenAndServe`。
- 支持 `ReadHeaderTimeout`。
- 支持 `ReadTimeout`。
- 支持 `WriteTimeout`。
- 支持 `IdleTimeout`。
- 支持 SIGINT/SIGTERM graceful shutdown。
- 支持 shutdown timeout 配置。
- 配置示例和文档同步。

暂不实现：

- systemd unit。
- Docker healthcheck。
- Kubernetes manifests。
- reload without restart。

## 验收标准

- 默认 `read_header_timeout_seconds` 为 10。
- 默认 `idle_timeout_seconds` 为 120。
- 默认 `shutdown_timeout_seconds` 为 10。
- timeout 配置不允许负数。
- `make verify` 和 `make smoke` 在 WSL `Ubuntu-24.04` 中通过。

