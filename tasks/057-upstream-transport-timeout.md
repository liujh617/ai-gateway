# Task 057: Upstream Transport Timeout

## 状态

Done

## 背景

HTTP API 已经把 `context.DeadlineExceeded` 映射为 `504 provider timeout`。OpenAI-compatible provider 使用 `http.Client` 调用上游时，transport 层可能返回实现了 `net.Error.Timeout()` 的错误；这些错误也应该被统一识别为上游超时，而不是普通 provider error。

## 范围

实现：
- OpenAI-compatible provider 识别 `context.DeadlineExceeded` 和 `net.Error.Timeout()`。
- 上游 transport timeout 统一包装为可被 `errors.Is(err, context.DeadlineExceeded)` 识别的错误。
- 覆盖 chat completions 上游 transport timeout 测试。
- 同步 API spec 和 architecture overview。

暂不实现：
- 为不同 transport timeout 暴露独立错误类型。
- 改变 HTTP API 的 `504 server_error` 响应格式。
- 调整 provider timeout 配置项。

## 验收标准

- OpenAI-compatible provider 的 transport timeout 能被 `errors.Is(err, context.DeadlineExceeded)` 识别。
- HTTP API 仍可将 provider timeout 映射为 `504 server_error`。
- 非 timeout transport error 仍按普通 provider error 处理。
- `make verify`、`make build`、`make check-config-examples` 和 `make smoke` 在 WSL `Ubuntu-24.04` 中通过。
