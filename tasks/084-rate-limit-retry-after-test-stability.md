# Task 084 - Rate limit Retry-After test stability

## 状态

Done.

## 背景

Task 083 将 `Retry-After` 从固定 60 秒改为当前固定窗口剩余秒数。API 集成测试仍断言固定 `60`，在当前快速执行路径下通常能通过，但已经和新语义不完全一致，也可能在慢环境中变得脆弱。

## 目标

- API 集成测试断言 `Retry-After` 是合法整数秒。
- 对当前 1 分钟窗口，断言取值位于 `1..60`。
- 同步修正 Task 080 中写死 `Retry-After: 60` 的历史描述。

## 验收

- `TestChatCompletionsRateLimit` 不再依赖固定 60 秒。
- WSL `Ubuntu-24.04` `make verify` 通过。
