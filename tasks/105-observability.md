# Task 105: Observability (OpenTelemetry + Grafana)

## 状态

Todo

## 目标

为 AI Gateway 添加生产级可观测性能力，集成 OpenTelemetry 标准和 Grafana 监控，提供分布式追踪、指标采集和可视化能力。

## 设计规范

- **Spec**: `docs/superpowers/specs/2026-08-05-observability-design.md`

## 核心功能

### OpenTelemetry Tracing

- Trace Context Propagation（W3C标准）
- Span Creation（HTTP、Upstream、Audit、PII、Content Safety）
- Sampling Strategy（10%默认，错误100%，慢请求100%）
- OTLP Exporter集成

### Prometheus Metrics

- `/metrics` endpoint
- 请求指标（rate、duration、size、active）
- Token指标（prompt、completion、total）
- 安全指标（pii、content_safety、alerts）
- 错误指标（errors、upstream_errors）
- 系统指标（audit_log_size、encryption_operations）

### Grafana Dashboard

- 预置Dashboard JSON
- 请求概览（rate、latency、errors）
- Provider/Model分布
- Token使用
- 安全监控
- 系统状态

## 实施计划

### 阶段 1：OpenTelemetry集成（3天）

**文件**：
- `internal/telemetry/telemetry.go` - Telemetry初始化
- `internal/telemetry/tracer.go` - Tracer封装
- `internal/telemetry/config.go` - 配置结构
- `internal/telemetry/middleware.go` - HTTP middleware

**任务**：
- [ ] 添加OpenTelemetry依赖（go.opentelemetry.io/otel）
- [ ] 实现Telemetry初始化函数
- [ ] 实现Tracer封装（Start、End、SetAttributes）
- [ ] 实现HTTP middleware（Trace Context提取/注入）
- [ ] 实现配置结构和验证
- [ ] 添加环境变量支持
- [ ] 添加采样策略（traceidratio）
- [ ] 添加单元测试

### 阶段 2：Prometheus Metrics（2天）

**文件**：
- `internal/telemetry/metrics.go` - Metrics定义和注册
- `cmd/gateway/metrics_handler.go` - `/metrics` endpoint

**任务**：
- [ ] 添加Prometheus client依赖（github.com/prometheus/client_golang）
- [ ] 定义请求指标（Counter、Histogram、Gauge）
- [ ] 定义Token指标
- [ ] 定义安全指标
- [ ] 定义错误和系统指标
- [ ] 注册Metrics到Prometheus Registry
- [ ] 实现`/metrics` endpoint（IP白名单）
- [ ] 添加Metrics中间件（自动记录）
- [ ] 添加单元测试

### 阶段 3：Span集成（2天）

**文件**：
- `internal/audit/telemetry.go` - 审计Span集成
- `internal/audit/pii_telemetry.go` - PII检测Span
- `internal/audit/content_safety_telemetry.go` - 内容安全Span
- `cmd/gateway/upstream_telemetry.go` - 上游调用Span

**任务**：
- [ ] 审计日志记录Span（has_pii、has_content_safety）
- [ ] PII检测Span（type、action、count）
- [ ] 内容安全检测Span（category、severity）
- [ ] 上游调用Span（provider、model、tokens）
- [ ] 集成到现有代码路径
- [ ] 添加单元测试

### 阶段 4：Metrics集成（2天）

**文件**：
- `internal/audit/metrics.go` - 审计Metrics
- `internal/audit/pii_metrics.go` - PII检测Metrics
- `internal/audit/content_safety_metrics.go` - 内容安全Metrics
- `cmd/gateway/request_metrics.go` - 请求Metrics

**任务**：
- [ ] 审计Metrics集成（encryption_operations）
- [ ] PII检测Metrics集成（pii_violations_total）
- [ ] 内容安全Metrics集成（content_safety_violations_total）
- [ ] 请求Metrics集成（requests_total、duration、size）
- [ ] Token Metrics集成（tokens_prompt、completion、total）
- [ ] 错误Metrics集成（errors_total、upstream_errors_total）
- [ ] 添加集成测试

### 阶段 5：Grafana Dashboard（1天）

**文件**：
- `contrib/grafana-dashboard.json` - Dashboard JSON
- `contrib/prometheus.yml` - Prometheus配置示例
- `docs/observability.md` - 使用文档

**任务**：
- [ ] 创建Grafana Dashboard JSON（6 Rows）
  - [ ] Row 1: 请求概览（rate、latency、active、errors）
  - [ ] Row 2: Provider分布（requests、latency、errors by provider）
  - [ ] Row 3: Model分布（top 10、latency、tokens by model）
  - [ ] Row 4: Token使用（rate、prompt vs completion、cost）
  - [ ] Row 5: 安全监控（pii、content safety、alerts）
  - [ ] Row 6: 系统状态（audit log、encryption、uptime）
- [ ] 创建Prometheus配置示例
- [ ] 创建使用文档（快速开始、配置、排查）
- [ ] 测试Dashboard导入和渲染

### 阶段 6：配置和集成（1天）

**文件**：
- `internal/config/telemetry_config.go` - Telemetry配置
- `cmd/gateway/main.go` - 集成Telemetry初始化

**任务**：
- [ ] 添加TelemetryConfig结构（tracing、metrics）
- [ ] 集成到主配置（config.json）
- [ ] 在main.go中初始化Telemetry
- [ ] 添加启动日志（telemetry status）
- [ ] 添加环境变量覆盖
- [ ] 添加配置验证
- [ ] 更新README（observability section）

### 阶段 7：文档和测试（1天）

**文件**：
- `docs/observability.md` - 用户文档
- `docs/developer/telemetry.md` - 开发者文档
- `README.md` - 更新observability section

**任务**：
- [ ] 完善用户文档
  - [ ] 快速开始指南
  - [ ] OpenTelemetry配置说明
  - [ ] Prometheus集成步骤
  - [ ] Grafana Dashboard使用说明
  - [ ] 常见问题排查
- [ ] 完善开发者文档
  - [ ] 添加新的Span
  - [ ] 添加新的Metrics
  - [ ] 自定义Dashboard
  - [ ] 性能优化建议
- [ ] 添加性能测试
  - [ ] Tracing overhead benchmark
  - [ ] Memory leak detection
- [ ] 端到端测试
  - [ ] Trace context propagation
  - [ ] Prometheus scraping
  - [ ] Grafana dashboard rendering

## 配置结构

添加到 `config.json`：

```json
{
  "telemetry": {
    "enabled": true,
    "tracing": {
      "enabled": true,
      "sample_rate": 0.1,
      "exporter": "otlp",
      "endpoint": "localhost:4317",
      "service_name": "ai-gateway",
      "attributes": {
        "environment": "production",
        "version": "1.6.0"
      }
    },
    "metrics": {
      "enabled": true,
      "endpoint": "/metrics",
      "allowed_ips": ["10.0.0.0/8", "172.16.0.0/12"],
      "namespace": "ai_gateway"
    }
  }
}
```

## 依赖

### Go Modules

```go
require (
    go.opentelemetry.io/otel v1.21.0
    go.opentelemetry.io/otel/trace v1.21.0
    go.opentelemetry.io/otel/sdk v1.21.0
    go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.21.0
    go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.21.0
    github.com/prometheus/client_golang v1.17.0
)
```

### External Services

- OpenTelemetry Collector (可选)
- Jaeger / Tempo (Trace backend)
- Prometheus Server
- Grafana

## 性能目标

- Trace overhead: < 5% CPU and < 10ms latency
- Metrics overhead: < 2% CPU
- Memory footprint: < 20MB additional
- Metrics cardinality: < 1000 unique label combinations

## 成功标准

- OpenTelemetry Trace 集成完成，支持 W3C Trace Context
- Prometheus metrics endpoint (`/metrics`) 可访问
- Grafana Dashboard 可正常渲染
- Trace overhead < 5% CPU and < 10ms latency
- Documentation 完整
- Performance tests passing
- Integration tests passing

## 风险和缓解

| 风险 | 影响 | 缓解措施 |
|------|------|---------|
| Tracing性能开销过大 | 性能 | 使用采样策略、异步导出、性能测试 |
| Metrics基数爆炸 | 性能 | 限制Label、使用白名单、Histogram替代Summary |
| 内存泄漏 | 稳定性 | 添加内存泄漏测试、定期Review代码 |
| 配置复杂 | 可用性 | 提供默认配置、环境变量覆盖、详细文档 |

## 预计时间

**总计**: 12 天（约 2.5 周）

- 阶段 1: 3 天
- 阶段 2: 2 天
- 阶段 3: 2 天
- 阶段 4: 2 天
- 阶段 5: 1 天
- 阶段 6: 1 天
- 阶段 7: 1 天

## 下一步

开始阶段 1：OpenTelemetry集成