# 133 Env API Keys Empty Entry

## 背景

`GATEWAY_API_KEYS` 用逗号分隔多个 gateway API key。旧实现会丢弃空片段，可能让 `env-key-1,,env-key-2` 被静默接受，甚至让仅包含逗号的值变成无鉴权配置。

## 变更

- `GATEWAY_API_KEYS` 分割后保留空片段。
- 复用现有 `api_keys` 校验，让空片段返回配置错误。
- 增加环境变量覆盖回归测试。

## 验证

- `go test ./internal/config -run TestEnvironmentAPIKeysRejectsEmptyEntry -count=1`
- `go test ./internal/config`
- `make verify`
