# Task 054: SSE Leading BOM

## 状态

Done

## 背景

SSE 流按 UTF-8 解析。部分上游可能在 stream 第一行开头带 UTF-8 BOM；如果不处理，首个 `data:` 字段会被解析成带 BOM 的未知字段，导致第一条 chunk 被跳过或后续行为异常。

## 范围

实现：
- 忽略上游 SSE stream 第一行开头的 UTF-8 BOM。
- 仅处理 stream 第一行，不影响后续 payload 内容。
- 补充 leading BOM 流式读取测试。
- 同步 API spec 和 architecture overview。

暂不实现：
- 清理后续行或 payload 内部的 BOM。
- 对非 UTF-8 字节流做转码。
- 修改非流式 JSON 响应解析行为。

## 验收标准

- `\ufeffdata: {...}` 作为 stream 首行时能正常解析 chunk。
- 后续 `data: [DONE]` 仍正常结束流。
- 现有 SSE 多行、metadata、大小限制和终止规则不变。
- `make verify`、`make build`、`make check-config-examples` 和 `make smoke` 在 WSL `Ubuntu-24.04` 中通过。
