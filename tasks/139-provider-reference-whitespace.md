# 139 Provider Reference Whitespace

## 背景

Provider 名称已经拒绝首尾空白。模型路由中的 provider 引用和 fallback provider 引用也应给出同样明确的配置错误，避免把空白问题伪装成 unknown provider。

## 变更

- `models.<external>.provider` 拒绝首尾空白。
- `models.<external>.fallbacks[].provider` 拒绝首尾空白。
- 增加配置加载回归测试。

## 验证

- `go test ./internal/config -run TestLoadConfigRejectsProviderReferencesWithWhitespace -count=1`
- `go test ./internal/config`
- `make verify`
