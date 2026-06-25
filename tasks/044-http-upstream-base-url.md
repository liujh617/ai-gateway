# Task 044: HTTP Upstream Base URL

## 状态

Done

## 背景

OpenAI-compatible provider 只能通过 HTTP 客户端访问上游。此前 `base_url` 只做了通用 URI 解析，可能接受 `ftp://...` 等非 HTTP URL。应在配置层和 provider 构造层都拒绝非 HTTP(S) URL，尽早暴露配置错误。

## 范围

实现：

- OpenAI-compatible provider 构造时要求 `base_url` 使用 `http` 或 `https` scheme。
- 配置校验时要求 openai-compatible provider `base_url` 使用 `http` 或 `https` scheme。
- `base_url` 仍必须包含 host。
- 补充 provider 和 config 测试。
- 同步 API spec 和 architecture overview。

暂不实现：

- 限制必须使用 `https`。
- 配置级别允许自定义非 HTTP transport。
- URL 连通性探测。

## 验收标准

- `ftp://...` 这类非 HTTP(S) `base_url` 在配置加载时被拒绝。
- 直接构造 OpenAI-compatible provider 时也拒绝非 HTTP(S) `base_url`。
- 现有 HTTP(S) 配置行为不变。
- `make verify`、`make build`、`make check-config-examples` 和 `make smoke` 在 WSL `Ubuntu-24.04` 中通过。
