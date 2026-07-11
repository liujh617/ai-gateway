# 135 Single API Key Validation

## 背景

`api_keys` 和 `api_clients[].api_key` 已经拒绝空值和首尾空白，但单个 `api_key` 没有同等校验，可能让配置文件或 `GATEWAY_API_KEY` 覆盖接受不稳定的 Bearer token。

## 变更

- 增加单个 `api_key` 的非空和首尾空白校验。
- 增加配置加载回归测试。

## 验证

- `go test ./internal/config -run TestLoadConfigRejectsInvalidGatewayAPIKey -count=1`
- `go test ./internal/config`
- `make verify`
