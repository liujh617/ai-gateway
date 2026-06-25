# Task 039: Strict Upstream JSON Response

## 状态

Done

## 背景

Task 026 已经让客户端 JSON 请求体必须只包含一个 JSON 值。OpenAI-compatible provider 解析上游 JSON 响应时也应采用同样的严格语义，避免上游或代理返回拼接 JSON 时被静默接受第一段内容。

## 范围

实现：

- OpenAI-compatible provider 的 JSON response decoder 拒绝 trailing JSON。
- 保留上游响应体大小限制。
- 覆盖 chat completions 上游响应 trailing JSON 测试。
- 覆盖 embeddings 上游响应 trailing JSON 测试。
- 同步 API spec 和 architecture overview。

暂不实现：

- 对 SSE `data:` payload 之外的 stream 整体做额外 JSON framing 校验。
- `DisallowUnknownFields`，因为上游响应可能包含兼容扩展字段。
- 对 upstream error body 强制单一 JSON 值。

## 验收标准

- 非流式 chat completions 上游响应拼接第二个 JSON 值时返回 provider error。
- embeddings 上游响应拼接第二个 JSON 值时返回 provider error。
- 正常上游 JSON 响应行为不变。
- `make verify`、`make build`、`make check-config-examples` 和 `make smoke` 在 WSL `Ubuntu-24.04` 中通过。
