# Task 031: Metrics Path Normalization

## 状态

Done

## 背景

Task 029 和 Task 030 已经让未知路由和 method not allowed 返回统一 JSON 错误。metrics 中如果直接使用原始 request path，会让任意未知路径进入 Prometheus label，带来无界 cardinality 风险。

## 范围

实现：

- 已知路由的 metrics `path` 标签保持原始路由路径。
- 未知路由的 metrics `path` 标签统一归一化为 `/__unknown__`。
- 保持 `method` 和 `status` 标签不变。
- 覆盖未知路径聚合和已知路径 405 的测试。
- 同步 API spec 和 architecture overview。

暂不实现：

- 动态路由模板提取。
- access log path 归一化。
- Prometheus histogram bucket。

## 验收标准

- 多个未知路径的 404 请求聚合到同一个 `/__unknown__` metrics 序列。
- metrics 输出不包含未知请求的原始 path。
- 已知路径使用错误 method 时，405 metrics 仍保留该已知 path。
- `make verify`、`make build`、`make check-config-examples` 和 `make smoke` 在 WSL `Ubuntu-24.04` 中通过。
