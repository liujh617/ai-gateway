# Task 020: Request Body Limit

## 状态

Done

## 背景

网关直接接收客户端 JSON 请求，如果不限制请求体大小，超大 prompt/input 可能造成内存压力、日志排查困难或上游调用风险。Task 020 增加配置化请求体大小上限。

## 范围

实现：

- `max_request_body_bytes` 配置。
- 默认 1 MiB。
- `0` 表示关闭限制。
- chat completions 请求体限制。
- embeddings 请求体限制。
- 超限返回 `413 invalid_request_error`。
- 配置 schema、示例和文档同步。

暂不实现：

- endpoint 级不同 body limit。
- token 数量限制。
- provider 级 prompt/input 限制。

## 验收标准

- 超大 chat completions body 返回 `413`。
- 超大 embeddings body 返回 `413`。
- 负数配置启动失败。
- `make verify`、`make build`、`make check-config-examples` 和 `make smoke` 在 WSL `Ubuntu-24.04` 中通过。

