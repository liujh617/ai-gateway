# Task 061: Trae + DeepSeek Local Proxy

## 状态

Done

## 背景

用户希望在没有 OpenAI API key、但可以使用自己的 DeepSeek API key 的情况下，将 Trae 或其他 OpenAI-compatible 客户端接入本地 `open-ai-gateway`。现有 OpenAI-compatible provider 已经具备转发 DeepSeek Chat API 的基础能力，因此本任务聚焦本地部署配置和文档。

## 范围

实现：

- 新增 DeepSeek OpenAI-compatible 示例配置。
- 记录 Trae/DeepSeek/本地网关的请求链路和 key 分层。
- 将 DeepSeek 示例配置纳入配置示例自检。
- 在 README 中补充 DeepSeek/Trae 文档入口。

暂不实现：

- Trae 专有配置自动写入。
- DeepSeek 专有参数转换。
- 非 OpenAI-compatible 接口兼容层。
- Responses API 兼容层。

## 验收标准

- `config.deepseek.example.json` 可以被当前配置加载器接受。
- 文档明确区分网关 API key 和 DeepSeek API key。
- 文档给出 WSL `Ubuntu-24.04` 下的构建、启动和 curl 验证命令。
- `make verify`、`make build`、`make check-config-examples` 和 `make smoke` 在 WSL `Ubuntu-24.04` 中通过。
