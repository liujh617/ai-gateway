# 131 Strict Config JSON

## 背景

请求体和上游 JSON 响应已经拒绝 trailing JSON。配置文件加载也应保持同样严格，避免 `config.json` 中拼接的第二个 JSON 值被静默忽略。

## 变更

- 配置文件 decode 后继续检查 EOF。
- 如果配置文件包含第二个 JSON value，返回 `decode config: config must contain a single JSON value`。
- 增加配置加载回归测试。

## 验证

- `go test ./internal/config -run TestLoadConfigRejectsTrailingJSON -count=1`
- `go test ./internal/config`
- `make verify`
