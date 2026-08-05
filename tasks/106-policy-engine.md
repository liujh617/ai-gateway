# Task 106: Policy Engine

## 状态

- **状态**：📋 设计完成，待实施
- **优先级**：高
- **预计时间**：15 天（约 3 周）
- **分支**：`feature/policy-engine`（待创建）
- **版本**：`v1.7.0-policy-engine`

## 背景

详见设计文档：`docs/superpowers/specs/2026-08-05-policy-engine-design.md`

当前平台的控制能力分散在鉴权、检测、限流等多个组件中，缺乏统一的策略决策入口。Policy Engine 引入 OPA Embedded + Rego 策略语言，提供统一的、声明式的、可审计的治理层。

## 范围

### 包含

- OPA Embedded 集成（纯 Go 库，无外部进程）
- Rego 策略引擎接口与实现
- 5 个决策钩子（authz / preprocess / response_check / rate_limit / route_select）
- 4 个内置策略包（authz / pii_policy / model_access / dynamic_rate_limit）
- 策略热加载（fsnotify）
- 配置结构（PolicyEngineConfig）
- HTTP Middleware 集成
- PII/内容安全检测器集成
- 8 个 Prometheus 指标
- OpenTelemetry Span
- Dashboard API（策略列表/测试/重载）
- 完整文档和示例

### 不包含

- RBAC（用户/角色/权限模型）→ 下阶段
- 策略可视化编辑器
- 分布式策略分发

## 验收标准

- [ ] Policy Engine 可通过配置启用/禁用
- [ ] 5 个决策钩子均可触发策略评估
- [ ] 内置 4 个策略包通过 Rego 单元测试
- [ ] 策略文件修改后 5 秒内自动热加载
- [ ] 策略评估延迟 P99 < 1ms
- [ ] 策略评估超时按 default_decision 处理
- [ ] deny 决策返回正确的 HTTP 状态码和原因
- [ ] 所有策略评估记录 Prometheus 指标
- [ ] Dashboard 可查看策略列表和评估统计
- [ ] 完整文档 `docs/policy-engine.md`

## 实施进度

### 阶段 1：核心接口与 OPA 集成

**产出**：`internal/policy/engine.go`、`types.go`、`noop.go`

- [ ] 定义 `Engine` 接口
- [ ] 定义 `Input`、`Decision`、`Action` 等类型
- [ ] 实现 `NoopEngine`（禁用时使用）
- [ ] 实现 `OPAEngine`（基于 OPA Go SDK）
  - [ ] Rego 查询编译与缓存
  - [ ] 策略文件加载
  - [ ] 数据文件加载
  - [ ] 评估超时控制
- [ ] 单元测试

### 阶段 2：配置结构

**产出**：`internal/config/policy_engine_config.go`

- [ ] 定义 `PolicyEngineConfig` 结构
- [ ] 添加到主 `Config` 结构
- [ ] 环境变量覆盖
- [ ] 配置验证
- [ ] 默认值（disabled）
- [ ] 单元测试

### 阶段 3：内置策略包

**产出**：`policies/core/*.rego` + `policies/core/*_test.rego`

- [ ] `authz.rego` — 基础访问控制
- [ ] `pii_policy.rego` — PII 处理策略
- [ ] `model_access.rego` — 模型访问策略
- [ ] `dynamic_rate_limit.rego` — 动态限流
- [ ] 策略数据文件示例（`policies/data/*.json`）
- [ ] Rego 单元测试（每个策略文件配套 `_test.rego`）

### 阶段 4：策略热加载

**产出**：`internal/policy/reloader.go`

- [ ] fsnotify 文件监听
- [ ] 原子策略替换（RWMutex）
- [ ] 编译失败回退（保留旧策略）
- [ ] Reload 事件日志和指标
- [ ] 单元测试

### 阶段 5：HTTP Middleware 集成

**产出**：`internal/middleware/policy.go`

- [ ] `PolicyMiddleware` 实现
- [ ] `buildInputFromRequest` — 从 HTTP 请求构建 Input
- [ ] `writeDenyResponse` — 构造拒绝响应
- [ ] 集成到 chi middleware chain
- [ ] `cmd/gateway/main.go` 中构建 Engine 并注册
- [ ] 集成测试

### 阶段 6：检测器集成

**产出**：修改 `internal/audit/pii_auditor.go`、`content_safety_auditor.go`

- [ ] PII 检测后触发 `preprocess.request` 钩子
- [ ] 内容安全检测后触发 `preprocess.request` 钩子
- [ ] 执行 Decision.Actions（如 redact_pii）
- [ ] 响应返回前触发 `check.upstream_response` 钩子
- [ ] 集成测试

### 阶段 7：可观测性

**产出**：`internal/policy/metrics.go`、修改 `telemetry/metrics.go`

- [ ] 新增 8 个 Prometheus 指标
- [ ] RecordEvaluation 辅助函数
- [ ] OpenTelemetry Span 集成
- [ ] Reload 指标
- [ ] 单元测试

### 阶段 8：Dashboard API

**产出**：修改 `internal/dashboard/api.go`、`config_api.go`

- [ ] `GET /api/v1/policies` — 策略列表
- [ ] `GET /api/v1/policies/:id` — 策略详情
- [ ] `POST /api/v1/policies/test` — 策略测试评估
- [ ] `POST /api/v1/policies/reload` — 手动重载
- [ ] `GET /api/v1/policies/stats` — 策略评估统计
- [ ] 单元测试

### 阶段 9：文档与示例

**产出**：`docs/policy-engine.md`

- [ ] 概述与架构
- [ ] 快速开始（启用 Policy Engine）
- [ ] Rego 策略编写指南
- [ ] 内置策略包说明
- [ ] 决策钩子详解
- [ ] 配置参考
- [ ] 故障排查
- [ ] 最佳实践

## 测试清单

### 单元测试

- [ ] `engine_test.go`：Engine 接口测试
- [ ] `config_test.go`：配置解析测试
- [ ] `reloader_test.go`：热重载测试
- [ ] `middleware_test.go`：中间件测试
- [ ] `metrics_test.go`：指标测试

### 策略测试

- [ ] `authz_test.rego`：访问控制策略测试
- [ ] `pii_policy_test.rego`：PII 策略测试
- [ ] `model_access_test.rego`：模型访问测试
- [ ] `dynamic_rate_limit_test.rego`：限流策略测试

### 集成测试

- [ ] 策略拒绝请求 → 403
- [ ] PII 检测 → 策略触发脱敏
- [ ] 策略热重载 → 新策略生效
- [ ] 策略评估超时 → default_decision
- [ ] 禁用 Policy Engine → 全部放行

### 性能测试

- [ ] 单次评估基准 < 1ms P99
- [ ] 热重载基准 < 100ms
- [ ] 1000 req/s 压力测试

## 依赖

### 新增

- `github.com/open-policy-agent/opa` v0.65+
- `github.com/fsnotify/fsnotify` v1.7+

## 风险

| 风险 | 缓解 |
|------|------|
| Rego 学习曲线 | 提供模板和文档 |
| OPA 评估性能 | 超时 + 决策缓存 |
| 策略误写导致拒绝 | default=allow + 预测试 |
