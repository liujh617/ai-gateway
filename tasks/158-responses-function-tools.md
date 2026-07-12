# Task 158 - Responses Function Tools

## 状态

Done.

## 目标

- 支持 Responses function definitions、strict、常用 tool choice 和 parallel calls。
- 支持无状态 function call/output 两轮关联。
- 支持非流式 function Items 和 typed arguments streaming。
- 复用现有 provider 接口、fallback、熔断、audit 和 metrics。
- 增加离线 `make smoke-responses-tools` 并接入 `release-check`。

## 验收

- 单个和多个 function calls 可正确转换。
- function output 必须关联同一 input 中更早的 call ID。
- 不支持工具和无效结构返回稳定 400。
- 纯文本 Responses 行为不回归。
- WSL `make verify`、`make smoke-responses-tools` 和 `make release-check` 通过。
