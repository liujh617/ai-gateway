# Task 053: SSE Field Without Colon

## 状态

Done

## 背景

SSE 规则中，没有冒号的 field line 表示字段名存在但值为空。当前 parser 会直接忽略这类行，虽然常见 OpenAI-compatible 流通常使用 `data: ...`，但补齐该规则可以让 provider adapter 的 SSE 行为更贴近协议。

## 范围

实现：
- 没有冒号的 SSE field line 按空值处理。
- 保留 `data:` 多行合并、comment、`event`、`id`、`retry` 忽略规则。
- 补充空 `data` field line 测试。
- 同步 API spec 和 architecture overview。

暂不实现：
- 支持自定义 SSE event 类型分发。
- 暴露空 data event 给上层 handler。
- 调整 `[DONE]` 结束语义。

## 验收标准

- `data` 后跟空行会被视为空 payload event，并由 `Next` 跳过。
- 后续 `data: [DONE]` 仍正常结束流。
- 现有多行 `data:` 和 metadata 行为不变。
- `make verify`、`make build`、`make check-config-examples` 和 `make smoke` 在 WSL `Ubuntu-24.04` 中通过。
