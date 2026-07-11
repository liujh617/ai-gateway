# 136 Single API Key Validation Docs

## 背景

Task 135 让单个 `api_key` 与 `api_keys`、`api_clients[].api_key` 保持一致，拒绝空值和首尾空白。该行为属于鉴权配置契约，需要同步文档。

## 变更

- 更新兼容契约，说明 gateway key 不允许为空或包含首尾空白。
- 更新 README 中 `api_key` 的配置说明。

## 验证

- 文档变更，无额外代码验证。
