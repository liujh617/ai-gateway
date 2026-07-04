# 130 Stream Timeout Provider Health Docs

## 背景

Task 129 让流式读取阶段 timeout 计入 provider failure。该行为属于 fallback 和 provider health 契约，需要同步文档。

## 变更

- 更新兼容契约，说明 stream 读取阶段 timeout 不会在当前响应切换 provider，但会影响后续 circuit breaker 判断。
- 更新架构概览中的 provider fallback/health 说明。

## 验证

- 文档变更，无额外代码验证。
