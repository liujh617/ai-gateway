# Task 060: Stream Read Timeout

## 状态

Done

## 背景

Task 059 覆盖了流式 chat completions 建连阶段的 transport timeout。建连成功后，读取上游 SSE body 时仍可能发生 transport timeout；这类错误也应被规范化为 `context.DeadlineExceeded`，让 HTTP API 能按 provider timeout 处理。

## 范围

实现：
- 上游 SSE body 读取阶段的非 EOF transport error 复用 timeout 规范化逻辑。
- 补充 stream headers 已返回但 body 读取超时的测试。
- 同步 API spec 和 architecture overview。

暂不实现：
- 为 stream 读取阶段 timeout 暴露独立错误类型。
- 改变已开始 SSE 响应后的 handler 错误写入策略。
- 调整 provider timeout 配置项。

## 验收标准

- Stream chat completions 读取阶段 timeout 能被识别为 `context.DeadlineExceeded`。
- Stream 建连阶段 timeout 覆盖保持不变。
- 非 timeout 的 SSE 解析错误行为不变。
- `make verify`、`make build`、`make check-config-examples` 和 `make smoke` 在 WSL `Ubuntu-24.04` 中通过。
