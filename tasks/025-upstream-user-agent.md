# Task 025: Upstream User-Agent

## 状态

Done

## 背景

OpenAI-compatible 上游服务通常会记录请求头。稳定的 `User-Agent` 可以帮助上游日志、限流策略和问题排查识别调用来源，并把具体网关版本和请求行为关联起来。

## 范围

实现：

- 为版本包新增 `UserAgent()`。
- User-Agent 格式为 `open-ai-gateway/<version>`。
- 版本为空时回退到 `dev`。
- 版本中的非法 product token 字符规范化为 `_`。
- OpenAI-compatible provider 所有上游请求统一设置 `User-Agent`。
- 覆盖 chat completions、stream chat completions 和 embeddings 上游请求测试。
- 同步 API spec、架构说明和部署文档。

暂不实现：

- 可配置 User-Agent。
- 多 product token 组合，例如运行环境或平台信息。
- 将 User-Agent 透出到响应。

## 验收标准

- 非流式 chat 上游请求包含稳定 `User-Agent`。
- 流式 chat 上游请求包含稳定 `User-Agent`。
- embeddings 上游请求包含稳定 `User-Agent`。
- `make verify`、`make build`、`make check-config-examples` 和 `make smoke` 在 WSL `Ubuntu-24.04` 中通过。
