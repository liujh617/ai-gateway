# 137 Provider Model Name Whitespace

## 背景

Gateway client、API key 和 client model 白名单已经拒绝首尾空白。Provider 名称和 external model 名称作为配置引用键，也应拒绝首尾空白，避免配置看起来相同但引用失败或暴露异常模型名。

## 变更

- provider map key 拒绝首尾空白。
- model map key 拒绝首尾空白。
- 增加配置加载回归测试。

## 验证

- `go test ./internal/config -run TestLoadConfigRejectsProviderAndModelNamesWithWhitespace -count=1`
- `go test ./internal/config`
- `make verify`
