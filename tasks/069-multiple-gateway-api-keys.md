# Task 069: Multiple Gateway API Keys

## 状态

Done

## 背景

网关此前只支持单个客户端 Bearer token。随着限流、成本和后续多租户能力推进，需要先支持多个 gateway API key，作为后续按客户端归集指标、配额和审计的基础。

## 范围

实现：

- 保留 `api_key` 单 key 配置，兼容早期配置。
- 新增 `api_keys` 字符串数组配置。
- 当 `api_keys` 非空时，任一列表 token 都可以通过 Bearer 鉴权。
- 支持 `GATEWAY_API_KEYS` 逗号分隔环境变量覆盖多 key 列表。
- 保留 `GATEWAY_API_KEY` 单 key 覆盖。
- `check-config` 只输出 gateway API key 数量，不输出 key 明文。
- 鉴权比较使用常量时间比较。
- 更新兼容契约、架构概览、README 和 JSON Schema。

暂不实现：

- API key 名称、租户 ID 或权限范围。
- 按 API key 拆分 metrics。
- 按 API key 配置不同模型权限。
- API key 热加载。
- API key 哈希存储。

## 验收标准

- `api_key` 仍可正常鉴权。
- `api_keys` 中任一 token 都可正常鉴权。
- 未知 token 返回 `401 authentication_error`。
- 公开路由不需要 Bearer token。
- 空白、重复或带首尾空白的 `api_keys` 被配置校验拒绝。
- `check-config` 不泄露 gateway API key 明文。
- `make verify`、`make build`、`make check-config-examples`、`make smoke` 和无 key 的 `make smoke-deepseek` 在 WSL `Ubuntu-24.04` 中通过。
