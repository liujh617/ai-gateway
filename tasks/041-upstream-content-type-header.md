# Task 041: Upstream Content-Type Header

## 状态

Done

## 背景

Task 040 明确了上游 `Accept` header。OpenAI-compatible provider 之前通过一个通用 header helper 给所有上游请求都设置 `Content-Type: application/json`，包括无请求体的 `GET /models`。`Content-Type` 应表达请求体格式，因此应只在带 JSON body 的请求上发送。

## 范围

实现：

- 拆分通用上游 headers 和 JSON body headers。
- `GET /models` 不发送 `Content-Type`。
- chat completions、stream chat completions 和 embeddings 的 POST 请求继续发送 `Content-Type: application/json`。
- 保持 `Accept`、`User-Agent`、Authorization 和 request id 行为不变。
- 补充 provider 请求 header 测试。
- 同步 API spec 和 architecture overview。

暂不实现：

- 根据不同 provider 类型配置 `Content-Type`。
- 校验上游响应 `Content-Type`。
- 修改客户端入站 Content-Type 规则。

## 验收标准

- 无请求体的 `GET /models` 上游请求不发送 `Content-Type`。
- 带 JSON body 的上游 POST 请求发送 `Content-Type: application/json`。
- 现有 provider 行为不变。
- `make verify`、`make build`、`make check-config-examples` 和 `make smoke` 在 WSL `Ubuntu-24.04` 中通过。
