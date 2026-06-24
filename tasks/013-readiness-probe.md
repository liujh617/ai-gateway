# Task 013: Readiness Probe

## 状态

Done

## 背景

Task 012 增加了容器化入口。容器部署通常需要区分 liveness 和 readiness：`/healthz` 表示进程仍然活着，`/readyz` 表示网关已经加载配置并准备接收业务流量。

## 范围

实现：

- `GET /readyz`。
- `/readyz` 不需要 Bearer token。
- `/readyz` 不参与 rate limit。
- router 暴露模型路由数量。
- readiness 至少检查一个可见模型路由。
- smoke test 覆盖 `/readyz`。
- 文档同步。

暂不实现：

- 主动探测上游 provider。
- provider 级 readiness。
- 后台健康检查缓存。
- Kubernetes manifests。

## 验收标准

- 已加载模型时 `/readyz` 返回 `200` 和 `status=ready`。
- 无模型时 `/readyz` 返回 `503` 和 `status=not_ready`。
- `/readyz` 无 Authorization header 可访问。
- `/readyz` 不受 rate limit 影响。
- `make verify` 和 `make smoke` 在 WSL `Ubuntu-24.04` 中通过。

