# Task 075 - Access log client fallback labels

## 背景

Task 070 已经为已鉴权 gateway client 的 access log 添加 `client` 字段，并让 HTTP metrics 对公开路由和鉴权失败使用稳定标签。access log 对公开路由、鉴权失败和无网关鉴权配置的请求还缺少同样稳定的 client 归因。

## 目标

- 公开路由 access log 记录 `client=public`。
- 鉴权失败 access log 记录 `client=unauthenticated`。
- 未配置 gateway API key 时 access log 记录 `client=unconfigured`。
- 不记录 Authorization header 或 API key 明文。
- 文档同步说明 access log 的特殊 client 标签。

## 验收

- middleware 测试覆盖三种特殊 client 标签。
- 既有 metrics client 标签行为不变。
- WSL `Ubuntu-24.04` 完整验证通过。

## 状态

Done.
