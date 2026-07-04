# 128 Embeddings Input Validation Coverage

## 背景

Embeddings 请求校验要求 `input` 存在且可处理。API 层已有缺少 `input` 的测试，还需要覆盖空白字符串输入。

## 变更

- 增加空白 embeddings `input` 返回 `400 invalid_request_error` 的测试。

## 验证

- `go test ./internal/api`
- `make verify`
