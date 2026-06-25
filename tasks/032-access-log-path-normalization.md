# Task 032: Access Log Path Normalization

## 状态

Done

## 背景

Task 031 已经让 metrics 的 `path` 标签归一化，避免未知路由造成 Prometheus label cardinality 无界增长。结构化 access log 同样会进入日志索引或聚合系统，也应该避免把任意未知路径原样记录为高基数字段。

## 范围

实现：

- 提供 middleware 内部共享的 route path 归一化 helper。
- metrics 和 access log 共用同一套已知路由表。
- 已知路由的 access log `path` 字段保持原始路由路径。
- 未知路由的 access log `path` 字段统一归一化为 `/__unknown__`。
- 覆盖未知路径日志归一化测试。
- 同步 API spec 和 architecture overview。

暂不实现：

- 记录原始未知 path。
- 动态路由模板提取。
- 日志字段可配置化。

## 验收标准

- 未知路径请求的 access log `path` 字段为 `/__unknown__`。
- access log 不包含未知请求的原始 path。
- metrics 仍使用同一套 path 归一化规则。
- `make verify`、`make build`、`make check-config-examples` 和 `make smoke` 在 WSL `Ubuntu-24.04` 中通过。
