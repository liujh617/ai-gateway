# Task 090 - Local verification metrics check

## 状态

Done.

## 背景

近期新增了 rate limit rejection、provider circuit open、provider fallback、provider health、token 和 cost 等多类 metrics。`docs/local-verification.md` 只说明了如何 curl `/metrics`，没有给出常见指标名检查方式，手工验证时不容易快速确认当前暴露面。

## 目标

- 在本地验证文档中增加常见 metrics 名称检查命令。
- 明确 rate limit、fallback 和 circuit-open 指标只有触发对应行为后才会出现。
- 不改变运行时代码或 smoke test 行为。

## 验收

- `docs/local-verification.md` 包含常见 metrics 名称 grep 命令。
- WSL `Ubuntu-24.04` `make verify` 通过。
