# Task 077 - Rate limit bucket pruning

## 状态

Done.

## 背景

Task 071 已支持全局和 per-client 的 in-memory rate limiter。限流 key 优先使用 gateway client name，但在未配置鉴权或异常路径中仍可能退回 token、remote address 或 anonymous。长时间运行时，如果旧 key 对应的 bucket 永不清理，内存占用会随着不同调用方累积。

## 目标

- Rate limiter 在处理请求时清理过期 bucket。
- 清理不改变当前固定窗口限流语义。
- 窗口内的 bucket 必须保留。
- 清理逻辑保持进程内、标准库实现，不引入后台 goroutine。

## 验收

- 过期 bucket 会在后续请求触发时被删除。
- 窗口内 bucket 不会被误删。
- WSL `Ubuntu-24.04` `make verify` 通过。
