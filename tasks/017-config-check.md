# Task 017: Config Check

## 状态

Done

## 背景

真实部署中，配置错误会是最常见的启动失败原因。Task 017 增加不启动 HTTP server 的配置自检模式，让部署前可以验证配置文件和环境变量是否合理，同时避免泄露密钥。

## 范围

实现：

- `open-ai-gateway check-config`。
- `GATEWAY_CHECK_CONFIG=1`。
- `make check-config`。
- 配置自检输出 provider/model 摘要。
- 检查 `api_key_env` 是否存在。
- 缺失 `api_key_env` 只输出 warning。
- 自检输出不包含 API key 明文。
- 文档同步。

暂不实现：

- 主动探测 upstream provider。
- JSON schema 文件。
- 配置热加载。
- 管理 API。

## 验收标准

- 默认配置自检成功。
- 指定配置文件自检成功。
- `api_key_env` 缺失时输出 warning。
- 自检输出不包含 gateway API key 或 upstream API key 明文。
- `make verify`、`make build`、`make check-config` 和 `make smoke` 在 WSL `Ubuntu-24.04` 中通过。

