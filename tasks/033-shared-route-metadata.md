# Task 033: Shared Route Metadata

## 状态

Done

## 背景

Task 031 和 Task 032 分别让 metrics 与 access log 使用 path 归一化。此时已知路由表和 405 allowed method 表存在漂移风险：新增路由时如果只更新 HTTP mux，可能忘记同步 metrics、日志或 method not allowed 判断。

## 范围

实现：

- 新增 `internal/routes` 包集中维护已知 HTTP 路由和允许 method。
- 405 method not allowed 判断改为使用共享路由元数据。
- metrics path 归一化改为使用共享路由元数据。
- access log path 归一化改为使用共享路由元数据。
- 覆盖 route path normalization、allowed methods 和返回值 copy 语义测试。
- 同步 architecture overview。

暂不实现：

- 自动从 `http.ServeMux` 提取路由。
- 动态路由模板。
- CORS/OPTIONS 路由元数据。

## 验收标准

- 已知路由和允许 method 由单一内部包维护。
- 405、metrics 和 access log 复用同一套 route metadata。
- 已知路径保留原 path，未知路径归一化为 `/__unknown__`。
- `make verify`、`make build`、`make check-config-examples` 和 `make smoke` 在 WSL `Ubuntu-24.04` 中通过。
