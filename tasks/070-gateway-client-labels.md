# Task 070: Gateway Client Labels

## 状态

Done

## 背景

Task 069 已支持多个 gateway API key。为了让后续按客户端观察请求量、token usage 和成本成为可能，需要给每个 gateway key 一个非敏感 client label，并确保日志和 metrics 使用 label 而不是 key 明文。

## 范围

实现：

- 新增 `api_clients` 配置，每项包含 `name` 和 `api_key`。
- 鉴权成功后将 gateway client name 写入 request context。
- access log 增加 `client` 字段。
- HTTP request metrics 增加 `client` 标签。
- token usage metrics 增加 `client` 标签。
- token cost metrics 增加 `client` 标签。
- 公开路由的 HTTP metrics client 标签为 `public`。
- 鉴权失败的 HTTP metrics client 标签为 `unauthenticated`。
- 旧 `api_key` 和 `api_keys` 继续兼容，默认 client 标签分别为 `default` 和 `key_1`、`key_2` 等。
- 配置自检不输出 gateway API key 明文。
- 更新兼容契约、架构概览、README 和 JSON Schema。

暂不实现：

- 按 client 的模型权限。
- 按 client 的独立限流额度。
- API key 哈希存储。
- API key 热加载。
- tenant / user 维度归集。

## 验收标准

- `api_clients` 中任一 client token 都可以通过鉴权。
- access log 输出 client name，但不输出 API key 明文。
- HTTP metrics、token metrics 和 cost metrics 包含 client 标签。
- 公开路由和鉴权失败请求有稳定 client 标签。
- 无效 `api_clients` 配置被校验拒绝。
- `make verify`、`make build`、`make check-config-examples`、`make smoke` 和无 key 的 `make smoke-deepseek` 在 WSL `Ubuntu-24.04` 中通过。
