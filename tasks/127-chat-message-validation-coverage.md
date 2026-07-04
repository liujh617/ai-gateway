# 127 Chat Message Validation Coverage

## 背景

Chat completions 请求校验要求每条 message 都包含非空 `role` 和可处理的 `content`。API 层已有缺少 `model` 和 `messages` 的测试，但 message 内部字段还缺少直接覆盖。

## 变更

- 增加缺少 message `role` 的 `400 invalid_request_error` 测试。
- 增加空白 message `content` 的 `400 invalid_request_error` 测试。

## 验证

- `go test ./internal/api`
- `make verify`
