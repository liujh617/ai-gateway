# Task 045: Upstream Base URL Shape

## 状态

Done

## 背景

Task 044 已经要求 OpenAI-compatible provider 的 `base_url` 使用 `http` 或 `https`。provider 通过 `base_url + path` 生成上游 endpoint，因此 `base_url` 如果包含 query 或 fragment，会让后续 endpoint 语义不清晰，例如 `https://api.example.com/v1?tenant=one/chat/completions`。

## 范围

实现：

- OpenAI-compatible provider 构造时拒绝包含 query 的 `base_url`。
- OpenAI-compatible provider 构造时拒绝包含 fragment 的 `base_url`。
- 配置校验时同步拒绝包含 query 或 fragment 的 `base_url`。
- 补充 provider 和 config 测试。
- 同步 API spec 和 architecture overview。

暂不实现：

- 自动移动 query 参数到每个上游请求。
- 配置级别 path join 重构。
- 限制 trailing slash，当前仍会 trim。

## 验收标准

- `https://api.example.com/v1?tenant=one` 在配置加载和 provider 构造时被拒绝。
- `https://api.example.com/v1#models` 在配置加载和 provider 构造时被拒绝。
- 现有不带 query/fragment 的 HTTP(S) 配置行为不变。
- `make verify`、`make build`、`make check-config-examples` 和 `make smoke` 在 WSL `Ubuntu-24.04` 中通过。
