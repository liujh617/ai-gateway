# 132 Strict Config JSON Docs

## 背景

Task 131 让配置文件拒绝 trailing JSON。该行为属于配置契约，需要同步到 spec 和架构文档。

## 变更

- 在兼容契约中说明 JSON 配置必须只包含一个 JSON 值。
- 在架构概览中说明配置加载会严格拒绝 trailing JSON。

## 验证

- 文档变更，无额外代码验证。
