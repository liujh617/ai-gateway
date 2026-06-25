# Task 046: Shared Upstream Base URL Validation

## 状态

Done

## 背景

Task 044 和 Task 045 已经为 OpenAI-compatible provider 明确了 `base_url` 的 URL 规则。当前配置校验和 provider 构造各自维护一份规则，后续继续扩展时容易出现行为不一致。

## 范围

实现：
- 新增共享的 upstream `base_url` 规范化和校验逻辑。
- 配置校验复用共享逻辑。
- OpenAI-compatible provider 构造复用共享逻辑。
- 保持当前错误语义：必填、HTTP(S) scheme、不能包含 query 或 fragment。
- 补充共享逻辑单元测试。
- 同步 API spec 和 architecture overview。

暂不实现：
- 支持带 query 的 provider 级默认参数。
- 重构 endpoint path join。
- 自动探测或请求上游 provider。

## 验收标准

- 配置校验和 provider 构造使用同一套 `base_url` 规则。
- `base_url` 会统一 trim 首尾空白和末尾 `/`。
- 非 HTTP(S)、缺少 host、包含 query 或 fragment 的 `base_url` 会被拒绝。
- `make verify`、`make build`、`make check-config-examples` 和 `make smoke` 在 WSL `Ubuntu-24.04` 中通过。
