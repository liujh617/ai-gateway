# AI Gateway Observability Design Spec

## Overview

为 AI Gateway 添加生产级可观测性能力，集成 OpenTelemetry 标准和 Grafana 监控，提供分布式追踪、指标采集和可视化能力。

---

## Goals

### Primary Goals

1. **OpenTelemetry 集成**：实现 Trace、Span、Metrics 标准采集
2. **Prometheus 指标暴露**：提供 `/metrics` endpoint 供 Prometheus 抓取
3. **Grafana Dashboard**：提供预置监控仪表盘
4. **分布式追踪**：支持 Trace ID 传播和关联

### Non-Goals

- 不实现日志聚合（已有 JSONL 审计日志）
- 不实现自定义告警规则（已有告警系统）
- 不实现 OpenTelemetry Collector 部署（由运维团队负责）

---

## Architecture

### Component Overview

```
┌─────────────────────────────────────────────────┐
│              AI Gateway Service                  │
├─────────────────────────────────────────────────┤
│  ┌──────────────┐  ┌──────────────┐           │
│  │ HTTP Handler │──│ OpenTelemetry│           │
│  └──────────────┘  │   Tracer     │           │
│                    └──────────────┘           │
│  ┌──────────────┐  ┌──────────────┐           │
│  │ Metrics      │──│ Prometheus   │           │
│  │ Collector    │  │ Exporter     │           │
│  └──────────────┘  └──────────────┘           │
└─────────────────────────────────────────────────┘
         │                      │
         │ Traces               │ Metrics
         ▼                      ▼
   ┌──────────┐          ┌──────────┐
   │ Jaeger/  │          │Prometheus│
   │ Tempo    │          │ Server   │
   └──────────┘          └──────────┘
         │                      │
         └──────────┬───────────┘
                    ▼
              ┌──────────┐
              │ Grafana  │
              │Dashboard │
              └──────────┘
```

---

## Design Details

### 1. OpenTelemetry Tracing

#### 1.1 Trace Context Propagation

**目的**：在整个请求生命周期中传播 Trace Context

**实现方式**：
- 使用 OpenTelemetry Go SDK
- 在 HTTP middleware 中提取/注入 Trace Context
- 支持 W3C Trace Context 标准

**Trace Context 格式**：
```
traceparent: 00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01
tracestate: congo=t61rcWkgMzE
```

#### 1.2 Span Creation

**自动创建的 Span**：

| Span Name | 类型 | 触发时机 | 关键属性 |
|-----------|------|---------|---------|
| `HTTP {method} {path}` | Server | HTTP 请求到达 | method, path, status_code |
| `upstream.{provider}` | Client | 调用上游 API | provider, model, tokens |
| `audit.log` | Internal | 审计日志写入 | has_pii, has_content_safety |
| `pii.detect` | Internal | PII 检测 | type, action |
| `content_safety.detect` | Internal | 内容安全检测 | category, severity |

**Span 属性规范**：

```go
// HTTP Span
span.SetAttributes(
    attribute.String("http.method", "POST"),
    attribute.String("http.url", "/v1/chat/completions"),
    attribute.Int("http.status_code", 200),
    attribute.String("http.route", "/v1/chat/completions"),
    attribute.String("user_agent", "Mozilla/5.0..."),
)

// Upstream Span
span.SetAttributes(
    attribute.String("provider", "openai"),
    attribute.String("model", "gpt-4"),
    attribute.Int("tokens.prompt", 100),
    attribute.Int("tokens.completion", 50),
    attribute.Int("tokens.total", 150),
)

// PII Detection Span
span.SetAttributes(
    attribute.String("pii.type", "phone_number"),
    attribute.String("pii.action", "alert"),
    attribute.Int("pii.count", 2),
)
```

#### 1.3 Sampling Strategy

**目的**：控制 Trace 数据量，避免性能影响

**采样策略**：
- **Head-based sampling**：在 Trace 开始时决定是否采样
- **默认采样率**：10%（可配置）
- **错误请求**：100% 采样
- **慢请求（> 5s）**：100% 采样

**配置示例**：
```json
{
  "telemetry": {
    "enabled": true,
    "tracing": {
      "enabled": true,
      "sample_rate": 0.1,
      "exporter": "otlp",
      "endpoint": "localhost:4317"
    }
  }
}
```

---

### 2. Prometheus Metrics

#### 2.1 Metrics Endpoint

**端点**：`GET /metrics`

**格式**：Prometheus text exposition format

**访问控制**：
- 仅允许内网访问（通过配置 `metrics_allowed_ips`）
- 可选：Basic Auth 保护

#### 2.2 Metrics 定义

**请求指标**：

| Metric Name | Type | Description | Labels |
|-------------|------|-------------|--------|
| `ai_gateway_requests_total` | Counter | 总请求数 | provider, model, status, method |
| `ai_gateway_request_duration_seconds` | Histogram | 请求延迟 | provider, model, method |
| `ai_gateway_request_size_bytes` | Histogram | 请求大小 | provider, model |
| `ai_gateway_response_size_bytes` | Histogram | 响应大小 | provider, model |
| `ai_gateway_active_requests` | Gauge | 活跃请求数 | provider |

**Token 指标**：

| Metric Name | Type | Description | Labels |
|-------------|------|-------------|--------|
| `ai_gateway_tokens_prompt_total` | Counter | Prompt Token 数量 | provider, model |
| `ai_gateway_tokens_completion_total` | Counter | Completion Token 数量 | provider, model |
| `ai_gateway_tokens_total` | Counter | 总 Token 数量 | provider, model |

**安全指标**：

| Metric Name | Type | Description | Labels |
|-------------|------|-------------|--------|
| `ai_gateway_pii_violations_total` | Counter | PII 违规数 | type, action |
| `ai_gateway_content_safety_violations_total` | Counter | 内容安全违规数 | category, severity |
| `ai_gateway_alerts_sent_total` | Counter | 告警发送数 | channel, severity, status |

**错误指标**：

| Metric Name | Type | Description | Labels |
|-------------|------|-------------|--------|
| `ai_gateway_errors_total` | Counter | 错误数 | provider, model, error_type |
| `ai_gateway_upstream_errors_total` | Counter | 上游错误数 | provider, error_code |

**系统指标**：

| Metric Name | Type | Description | Labels |
|-------------|------|-------------|--------|
| `ai_gateway_audit_log_size_bytes` | Gauge | 审计日志大小 | - |
| `ai_gateway_encryption_operations_total` | Counter | 加密操作数 | operation |

#### 2.3 Histogram Buckets

**请求延迟 buckets**：
```go
[]float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}
```

**请求大小 buckets**：
```go
[]float64{100, 500, 1000, 5000, 10000, 50000, 100000, 500000, 1000000}
```

---

### 3. Grafana Dashboard

#### 3.1 Dashboard Overview

**Dashboard 名称**：AI Gateway Overview

**Panel 划分**：

**Row 1: 请求概览**
- **Request Rate**: 请求速率（QPS）
- **Request Duration P50/P95/P99**: 请求延迟百分位
- **Active Requests**: 活跃请求数
- **Error Rate**: 错误率

**Row 2: Provider 分布**
- **Requests by Provider**: 各 Provider 请求数（饼图）
- **Latency by Provider**: 各 Provider 延迟对比
- **Errors by Provider**: 各 Provider 错误数

**Row 3: Model 分布**
- **Top 10 Models**: Top 10 模型请求数
- **Latency by Model**: 各模型延迟
- **Tokens by Model**: 各模型 Token 使用量

**Row 4: Token 使用**
- **Token Usage Rate**: Token 使用速率
- **Prompt vs Completion**: Prompt/Completion Token 对比
- **Token Cost Estimate**: Token 成本估算（可选）

**Row 5: 安全监控**
- **PII Violations**: PII 违规趋势
- **Content Safety Violations**: 内容安全违规趋势
- **Alerts Sent**: 告警发送数

**Row 6: 系统状态**
- **Audit Log Size**: 审计日志大小
- **Encryption Operations**: 加密操作数
- **Uptime**: 运行时间

#### 3.2 Dashboard JSON

提供完整的 Grafana Dashboard JSON 文件，可直接导入使用。

**文件位置**：`contrib/grafana-dashboard.json`

**导入方式**：
```bash
# 通过 Grafana UI 导入
# 或使用 Grafana API
curl -X POST http://grafana:3000/api/dashboards/db \
  -H "Content-Type: application/json" \
  -d @contrib/grafana-dashboard.json
```

#### 3.3 Alerting Rules

**预置告警规则**（可选）：

| Alert Name | Condition | Severity |
|------------|-----------|----------|
| HighErrorRate | Error Rate > 5% | critical |
| HighLatency | P99 Latency > 5s | warning |
| ProviderDown | Provider Error Rate > 50% | critical |
| TokenUsageSpike | Token Rate > 2x baseline | warning |

---

### 4. Configuration

#### 4.1 Telemetry Configuration

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
        "version": "1.5.0"
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

#### 4.2 Environment Variables

支持环境变量覆盖：

| Variable | Description | Default |
|----------|-------------|---------|
| `TELEMETRY_ENABLED` | 启用遥测 | `false` |
| `OTEL_TRACES_EXPORTER` | Trace 导出器类型 | `otlp` |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | OTLP endpoint | `localhost:4317` |
| `OTEL_TRACES_SAMPLER` | 采样器类型 | `traceidratio` |
| `OTEL_TRACES_SAMPLER_ARG` | 采样率 | `0.1` |
| `PROMETHEUS_METRICS_ENABLED` | 启用 Prometheus 指标 | `true` |

---

### 5. Implementation Details

#### 5.1 Package Structure

```
internal/telemetry/
├── telemetry.go          # Telemetry 初始化和管理
├── tracer.go             # OpenTelemetry Tracer 封装
├── metrics.go            # Prometheus Metrics 定义和注册
├── middleware.go         # HTTP middleware（Trace Context）
├── attributes.go         # Span/Metric 属性定义
└── config.go             # 配置结构
```

#### 5.2 Integration Points

**HTTP Middleware**：
```go
// 在 HTTP handler 中注入 Trace Context
func TraceMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Extract trace context from headers
        ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))
        
        // Start span
        tracer := otel.Tracer("ai-gateway")
        ctx, span := tracer.Start(ctx, fmt.Sprintf("HTTP %s %s", r.Method, r.URL.Path))
        defer span.End()
        
        // Add to context
        r = r.WithContext(ctx)
        
        // Wrap response writer to capture status code
        ww := &responseWriter{ResponseWriter: w}
        
        next.ServeHTTP(ww, r)
        
        // Set span attributes
        span.SetAttributes(
            attribute.Int("http.status_code", ww.statusCode),
        )
    })
}
```

**Audit Recorder Integration**：
```go
// 在审计记录时创建 Span
func (r *JSONLRecorder) Record(ctx context.Context, event *Event) error {
    ctx, span := otel.Tracer("ai-gateway").Start(ctx, "audit.log")
    defer span.End()
    
    span.SetAttributes(
        attribute.String("request_id", event.RequestID),
        attribute.Bool("has_pii", event.HasPII),
        attribute.Bool("has_content_safety", event.HasContentSafety),
    )
    
    // ... existing logic
}
```

**PII Detection Integration**：
```go
// 在 PII 检测时创建 Span
func (d *RegexDetector) Detect(ctx context.Context, text string) []PIIMatch {
    ctx, span := otel.Tracer("ai-gateway").Start(ctx, "pii.detect")
    defer span.End()
    
    matches := d.detectInternal(text)
    
    span.SetAttributes(
        attribute.Int("pii.count", len(matches)),
    )
    
    // Update metrics
    metrics.P IIDetections.Add(ctx, len(matches))
    
    return matches
}
```

---

### 6. Deployment

#### 6.1 Local Development

**启动 OpenTelemetry Collector**：
```bash
docker run -d --name otel-collector \
  -p 4317:4317 \
  -p 4318:4318 \
  otel/opentelemetry-collector:latest
```

**启动 Jaeger**（可选，用于查看 Trace）：
```bash
docker run -d --name jaeger \
  -p 16686:16686 \
  -p 14268:14268 \
  jaegertracing/all-in-one:latest
```

**启动 Prometheus**：
```bash
docker run -d --name prometheus \
  -p 9090:9090 \
  -v ./contrib/prometheus.yml:/etc/prometheus/prometheus.yml \
  prom/prometheus
```

**启动 Grafana**：
```bash
docker run -d --name grafana \
  -p 3000:3000 \
  grafana/grafana
```

#### 6.2 Production Deployment

**推荐架构**：

```
┌──────────────┐
│ AI Gateway   │
│ (Metrics)    │
└──────┬───────┘
       │
       ▼
┌──────────────┐
│ Prometheus   │──┐
└──────┬───────┘  │
       │          │
       ▼          │
┌──────────────┐  │
│ Grafana      │◄─┘
│ Dashboard    │
└──────────────┘

┌──────────────┐
│ AI Gateway   │
│ (Traces)     │
└──────┬───────┘
       │
       ▼
┌──────────────┐
│ OTLP         │
│ Collector    │
└──────┬───────┘
       │
       ▼
┌──────────────┐
│ Jaeger/Tempo │
│ (Backend)    │
└──────┬───────┘
       │
       ▼
┌──────────────┐
│ Grafana      │
│ (Query)      │
└──────────────┘
```

---

### 7. Performance Considerations

#### 7.1 Tracing Overhead

**目标**：Tracing 开销 < 5% CPU 和 < 10ms 延迟

**优化措施**：
- 使用采样策略控制数据量
- 批量导出 Trace 和 Metrics
- 异步导出（不阻塞请求处理）
- 禁用不必要的 Span 属性

#### 7.2 Metrics Cardinality

**目标**：控制 Metrics 基数，避免 Prometheus 性能问题

**限制措施**：
- Provider 和 Model 使用白名单过滤
- 高基数字段（如 user_id）不作为 Label
- 使用 Histogram 而非 Summary

#### 7.3 Memory Footprint

**预估**：
- Tracer: ~10MB
- Metrics Registry: ~5MB
- Export Buffer: ~2MB
- **总计**: ~17MB 额外内存

---

### 8. Security Considerations

#### 8.1 Trace Data Sensitivity

**风险**：Trace 可能包含敏感信息（如 PII）

**缓解措施**：
- 在 Span 属性中**不记录**原始请求/响应内容
- 仅记录元数据（如 token count、has_pii）
- Trace 数据存储在受保护的后端（如 Jaeger）

#### 8.2 Metrics Endpoint Security

**风险**：Metrics endpoint 可能泄露内部信息

**缓解措施**：
- IP 白名单限制访问
- 可选：Basic Auth 保护
- 不暴露敏感指标（如 API Key）

---

### 9. Testing

#### 9.1 Unit Tests

- Tracer initialization
- Span creation and attribute setting
- Metrics registration and recording
- Configuration validation

#### 9.2 Integration Tests

- Trace context propagation across requests
- Metrics endpoint accessibility
- Prometheus scraping
- Grafana dashboard rendering

#### 9.3 Performance Tests

- Tracing overhead benchmark
- High cardinality metrics test
- Memory leak detection

---

### 10. Documentation

#### 10.1 User Documentation

- **文件位置**: `docs/observability.md`
- **内容**:
  - 快速开始指南
  - OpenTelemetry 配置说明
  - Prometheus 集成步骤
  - Grafana Dashboard 使用说明
  - 常见问题排查

#### 10.2 Developer Documentation

- **文件位置**: `docs/developer/telemetry.md`
- **内容**:
  - 添加新的 Span
  - 添加新的 Metrics
  - 自定义 Dashboard
  - 性能优化建议

---

## Success Criteria

1. ✅ OpenTelemetry Trace 集成完成，支持 W3C Trace Context
2. ✅ Prometheus metrics endpoint (`/metrics`) 可访问
3. ✅ Grafana Dashboard 可正常渲染
4. ✅ Trace overhead < 5% CPU and < 10ms latency
5. ✅ Documentation 完整

---

## Open Questions

1. **是否需要支持 Jaeger Agent?**
   - 当前设计：仅支持 OTLP protocol
   - 如果需要，可以添加 Jaeger exporter

2. **是否需要支持自定义 Metrics?**
   - 当前设计：仅支持预定义 Metrics
   - 如果需要，可以添加 `metrics.custom` 配置项

3. **是否需要 OpenTelemetry Collector 部署?**
   - 当前设计：Gateway 直接导出到 Backend
   - 如果需要，可以添加 Collector 部署文档

---

## References

- [OpenTelemetry Go SDK](https://github.com/open-telemetry/opentelemetry-go)
- [Prometheus Client Golang](https://github.com/prometheus/client_golang)
- [Grafana Dashboard JSON Schema](https://grafana.com/docs/grafana/latest/dashboards/export-import/)
- [W3C Trace Context](https://www.w3.org/TR/trace-context/)
- [OpenTelemetry Semantic Conventions](https://opentelemetry.io/docs/reference/specification/trace/semantic_conventions/)