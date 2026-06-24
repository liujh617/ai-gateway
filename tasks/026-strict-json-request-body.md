# Task 026: Strict JSON Request Body

## 状态

Done

## 背景

Go 的 `json.Decoder.Decode` 默认会读取第一个 JSON 值后停止。如果请求体后面还有第二个 JSON 值或其他 token，旧实现会静默接受第一段内容。这会让客户端误发的 body 被当成正常请求处理，不利于兼容性和排障。

## 范围

实现：

- chat completions 请求体必须只包含一个 JSON 值。
- embeddings 请求体必须只包含一个 JSON 值。
- 合法 JSON 后的第二个 JSON 值或其他 token 返回 `400 invalid_request_error`。
- 保留请求体大小限制错误映射为 `413 invalid_request_error`。
- 提取公共 JSON body 解码 helper。
- 同步 API spec。

暂不实现：

- `DisallowUnknownFields`，因为兼容层需要透传未知字段。
- Content-Type 强校验。
- 对 GET/HEAD 请求体做额外限制。

## 验收标准

- chat completions 拒绝 trailing JSON。
- embeddings 拒绝 trailing JSON。
- 正常 chat completions 和 embeddings 请求不受影响。
- `make verify`、`make build`、`make check-config-examples` 和 `make smoke` 在 WSL `Ubuntu-24.04` 中通过。
