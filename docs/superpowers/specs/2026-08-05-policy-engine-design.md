# Policy Engine 设计

## 背景

当前 AI Gateway 已完成 MVP 全部功能（审计加密、PII 检测、内容安全检测、告警、Web Dashboard）并进入企业架构演进阶段。按照既定路线 **Observability → Policy Engine → Security → RBAC**，Observability 阶段已完成核心集成（Phases 1-4），现进入 **Policy Engine** 阶段。

### 问题：当前控制逻辑的碎片化

当前平台的"控制"能力分散在多个独立组件中：

| 控制点 | 当前实现 | 决策来源 | 问题 |
|--------|---------|---------|------|
| API Key 鉴权 | `middleware/auth.go` | 配置文件硬编码 | 无法动态授权、无策略表达力 |
| 模型访问白名单 | `clientModels` map | 配置文件 | 无法按用户/场景/时段动态控制 |
| PII 检测 | `PIIAuditorRecorder` | 检测器内部 | 仅 alert/reject/allow 三态，无组合规则 |
| 内容安全 | `ContentSafetyAuditorRecorder` | 检测器内部 | 同上 |
| 限流 | `RateLimiter` | 配置文件 | 全局/客户端粒度，无法策略驱动 |
| 告警路由 | `AlertManager` | 配置文件 | 静态路由，无策略决策 |

这些控制点各自独立，缺乏统一的策略语言和决策入口。企业场景需要：
- "财务部门的 API Key 在工作时间外禁止访问 GPT-4"
- "包含身份证号的请求必须脱敏后才能转发给上游模型"
- "标记为 'high_risk' 的用户每天最多调用 10 次"
- "所有输出必须经过内容安全检查才能返回给客户端"

这类**声明式、可组合、可审计**的治理需求，需要一个统一的 Policy Engine。

## 目标

### 核心目标

1. **统一策略决策入口**：所有控制点（鉴权、路由、检测、限流、输出）通过 Policy Engine 做决策
2. **声明式策略语言**：使用 Rego（OPA 原生策略语言）表达复杂治理规则
3. **数据不出域**：策略评估完全在本地内存完成，不依赖外部服务
4. **策略热加载**：修改策略文件后自动生效，无需重启网关
5. **完整审计**：每次策略决策记录评估过程和结果
6. **可观测**：策略评估延迟、命中率、拒绝率等指标接入 Prometheus

### 非目标（Non-Goals）

- ❌ **不实现 RBAC**（用户/角色/权限模型留给下一阶段）
- ❌ **不替换现有检测器**（PII/内容安全作为策略的数据输入源，而非被替换）
- ❌ **不做策略可视化编辑器**（MVP 通过文件管理 Rego 策略）
- ❌ **不做分布式策略分发**（单节点本地评估，不做集群同步）

## 架构定位

Policy Engine 在企业架构中的位置：

```
                    Client Request
                         │
                         ▼
              ┌─────────────────────┐
              │   HTTP Middleware    │
              └──────────┬──────────┘
                         │
                         ▼
              ┌─────────────────────┐
              │   Policy Engine      │ ◄── Rego 策略库（本地）
              │   (OPA Embedded)     │ ◄── 策略数据（配置/运行时）
              │                      │
              │   决策点：            │
              │   1. allow/deny      │
              │   2. transform       │
              │   3. rate_limit      │
              │   4. route           │
              └──────────┬──────────┘
                         │
              ┌──────────┴──────────┐
              │                     │
              ▼                     ▼
    ┌─────────────────┐   ┌─────────────────┐
    │  PII Detection   │   │ Content Safety   │
    │  (数据输入)       │   │  (数据输入)       │
    └────────┬────────┘   └────────┬────────┘
             │                     │
             └──────────┬──────────┘
                        │
                        ▼
              ┌─────────────────────┐
              │   Model Router       │
              │   (策略驱动路由)      │
              └──────────┬──────────┘
                         │
                         ▼
                   Upstream Model
```

### 决策钩子（Decision Hooks）

Policy Engine 在请求生命周期中设置 **5 个决策点**：

| 钩子 | 触发时机 | 典型策略 | 决策结果 |
|------|---------|---------|---------|
| `authz.request` | 鉴权后、路由前 | API Key 是否允许访问此 model | allow / deny(403) |
| `preprocess.request` | 路由后、转发前 | PII 检测后是否脱敏/拒绝 | allow / deny / transform |
| `check.upstream_response` | 收到上游响应后 | 输出内容安全检查 | allow / deny / redact |
| `enforce.rate_limit` | 每次请求 | 动态限流策略（超出配置） | allow / throttle(429) |
| `route.select` | 路由决策时 | 基于策略选择 provider/model | route_hint |

## 技术选型

### 为什么选 OPA + Rego

| 维度 | OPA/Rego | 自研 DSL | CEL (Google) | Casbin |
|------|----------|---------|--------------|--------|
| 策略表达能力 | 极强（图灵完备-ish） | 取决于设计 | 中等 | 中等（ACL/RBAC 强） |
| Go 嵌入 | ✅ 原生库 | ✅ | ✅ | ✅ |
| 生态 | CNCF 毕业项目 | 无 | K8s 标准 | 活跃 |
| 数据不出域 | ✅ 纯本地评估 | ✅ | ✅ | ✅ |
| 学习曲线 | 中等（Rego 语法） | 低 | 低 | 中 |
| 测试工具 | ✅ 内置 | 需自研 | 弱 | 弱 |
| 性能 | <1ms 典型策略 | 取决于实现 | <1ms | <1ms |

**结论**：选择 **OPA Embedded 模式**（`github.com/open-policy-agent/opa/rego`），原因：
1. CNCF 毕业项目，企业治理标准
2. Rego 表达力足以覆盖未来 RBAC/ABAC 场景
3. 纯 Go 嵌入，无外部进程依赖
4. 内置策略测试框架

### 不选独立 OPA Server 的原因

| 模式 | 优点 | 缺点 |
|------|------|------|
| **Embedded（选定）** | 零额外部署、低延迟、数据不出域 | 策略分发需自行管理 |
| 独立 OPA Server | 标准化 API、集中管理 | 额外组件、网络延迟、需维护 |

MVP 选择 Embedded，单二进制部署，符合"集成到网关"原则。

## 策略模型

### 输入（Input）

每次策略评估时，Policy Engine 将以下上下文构建为 `input` 文档：

```json
{
  "request": {
    "method": "POST",
    "path": "/v1/chat/completions",
    "model": "gpt-4",
    "body": { "...": "请求体" },
    "headers": {
      "authorization": "Bearer sk-****",
      "x-agent-trace-id": "trace-123"
    }
  },
  "client": {
    "name": "finance-app",
    "api_key_id": "key-001",
    "ip": "10.0.1.5"
  },
  "time": {
    "timestamp": "2026-08-05T14:30:00Z",
    "hour": 14,
    "weekday": "Tuesday"
  },
  "detection": {
    "pii": {
      "detected": true,
      "types": ["phone", "id_card"],
      "matches": [
        { "type": "phone", "value_masked": "138****5678" }
      ]
    },
    "content_safety": {
      "violated": false,
      "categories": []
    }
  },
  "config": {
    "model_provider": "openai",
    "model_capabilities": ["chat", "function_calling"]
  }
}
```

### 输出（Decision）

策略评估输出统一的 `Decision` 结构：

```json
{
  "allow": true,
  "deny_reason": "",
  "actions": [
    {
      "type": "redact_pii",
      "config": { "fields": ["body.messages"] }
    }
  ],
  "rate_limit": {
    "requests_per_minute": 10,
    "reason": "high_risk_client"
  },
  "route_hint": {
    "prefer_provider": "local-llm",
    "reason": "data_sovereignty"
  },
  "metadata": {
    "policy_id": "finance-restricted",
    "policy_version": "1.0.0",
    "evaluated_rules": 5
  }
}
```

### 内置策略包

MVP 提供 4 个开箱即用的策略包：

#### 1. `core/authz.rego` — 基础访问控制

```rego
package core.authz

default allow := true

# 拒绝已禁用的 client
deny[msg] {
    input.client.name == disabled_client
    msg := sprintf("client '%s' is disabled", [disabled_client])
}

# 工作时间限制
deny[msg] {
    not is_work_hour
    input.client.name in restricted_offhours_clients
    msg := sprintf("client '%s' not allowed outside work hours", [input.client.name])
}

is_work_hour {
    input.time.hour >= 9
    input.time.hour < 18
    input.time.weekday != "Saturday"
    input.time.weekday != "Sunday"
}
```

#### 2. `core/pii_policy.rego` — PII 处理策略

```rego
package core.pii_policy

# 身份证号必须脱敏
actions[{"type": "redact_pii", "config": {"types": ["id_card"]}}] {
    input.detection.pii.types[_] == "id_card"
}

# 银行卡号直接拒绝
deny[msg] {
    input.detection.pii.types[_] == "bank_card"
    msg := "bank card number detected: request denied"
}

# 高风险 client 的手机号也拒绝
deny[msg] {
    input.detection.pii.types[_] == "phone"
    input.client.name in high_risk_clients
    msg := "phone number not allowed for high-risk client"
}
```

#### 3. `core/model_access.rego` — 模型访问策略

```rego
package core.model_access

# 财务部门只能用指定模型
deny[msg] {
    input.client.name in finance_clients
    input.request.model in restricted_models
    msg := sprintf("finance client cannot access model '%s'", [input.request.model])
}

# 低优先级 client 降级到便宜模型
route_hint := {"prefer_provider": "local-llm"} {
    input.client.name in low_priority_clients
    input.request.model == "gpt-4"
}
```

#### 4. `core/dynamic_rate_limit.rego` — 动态限流策略

```rego
package core.dynamic_rate_limit

# 高风险 client 限制 10 req/min
rate_limit := {"requests_per_minute": 10} {
    input.client.name in high_risk_clients
}

# 普通 client 限制 100 req/min（可被配置覆盖）
rate_limit := {"requests_per_minute": 100} {
    not input.client.name in high_risk_clients
}
```

## 配置

### PolicyEngineConfig

在主配置中新增 `policy_engine` 段：

```json
{
  "policy_engine": {
    "enabled": true,
    "policy_dir": "/etc/ai-gateway/policies",
    "decision_hooks": [
      "authz.request",
      "preprocess.request",
      "check.upstream_response",
      "enforce.rate_limit",
      "route.select"
    ],
    "data_dir": "/etc/ai-gateway/policy-data",
    "default_decision": "allow",
    "decision_timeout_ms": 50,
    "hot_reload": true,
    "deny_status_code": 403,
    "metrics_enabled": true
  }
}
```

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `enabled` | bool | false | 是否启用 Policy Engine |
| `policy_dir` | string | `/etc/ai-gateway/policies` | Rego 策略文件目录 |
| `decision_hooks` | []string | 全部 5 个 | 启用的决策钩子 |
| `data_dir` | string | `/etc/ai-gateway/policy-data` | 策略数据（JSON）目录 |
| `default_decision` | string | "allow" | 策略评估超时/出错时的默认决策 |
| `decision_timeout_ms` | int | 50 | 单次评估超时（毫秒） |
| `hot_reload` | bool | true | 策略文件变更自动重载 |
| `deny_status_code` | int | 403 | deny 时返回的 HTTP 状态码 |
| `metrics_enabled` | bool | true | 是否记录 Prometheus 指标 |

### 环境变量覆盖

```
GATEWAY_POLICY_ENGINE_ENABLED=true
GATEWAY_POLICY_ENGINE_POLICY_DIR=/etc/ai-gateway/policies
GATEWAY_POLICY_ENGINE_DEFAULT_DECISION=deny
GATEWAY_POLICY_ENGINE_DECISION_TIMEOUT_MS=100
```

### 策略数据文件

`data_dir` 下放置 JSON 数据文件，供 Rego 策略引用：

```json
// /etc/ai-gateway/policy-data/clients.json
{
  "high_risk_clients": ["temp-app-001", "external-vendor"],
  "finance_clients": ["finance-bot", "accounting-ai"],
  "restricted_offhours_clients": ["intern-app"]
}
```

```json
// /etc/ai-gateway/policy-data/models.json
{
  "restricted_models": ["gpt-4", "claude-opus"],
  "low_cost_models": ["gpt-3.5-turbo", "deepseek-chat"]
}
```

## 接口设计

### PolicyEngine 接口

```go
// internal/policy/engine.go

package policy

// DecisionHook 标识策略评估的决策点
type DecisionHook string

const (
    HookAuthzRequest        DecisionHook = "authz.request"
    HookPreprocessRequest   DecisionHook = "preprocess.request"
    HookCheckResponse       DecisionHook = "check.upstream_response"
    HookEnforceRateLimit    DecisionHook = "enforce.rate_limit"
    HookRouteSelect         DecisionHook = "route.select"
)

// Input 策略评估输入
type Input struct {
    Request   RequestInput   `json:"request"`
    Client    ClientInput    `json:"client"`
    Time      TimeInput      `json:"time"`
    Detection DetectionInput `json:"detection"`
    Config    ConfigInput    `json:"config"`
}

// Decision 策略评估结果
type Decision struct {
    Allow      bool           `json:"allow"`
    DenyReason string         `json:"deny_reason,omitempty"`
    Actions    []Action       `json:"actions,omitempty"`
    RateLimit  *RateLimitHint `json:"rate_limit,omitempty"`
    RouteHint  *RouteHint     `json:"route_hint,omitempty"`
    Metadata   DecisionMeta   `json:"metadata"`
}

type Action struct {
    Type   string                 `json:"type"`
    Config map[string]interface{} `json:"config,omitempty"`
}

type RateLimitHint struct {
    RequestsPerMinute int    `json:"requests_per_minute"`
    Reason            string `json:"reason,omitempty"`
}

type RouteHint struct {
    PreferProvider string `json:"prefer_provider,omitempty"`
    PreferModel    string `json:"prefer_model,omitempty"`
    Reason         string `json:"reason,omitempty"`
}

type DecisionMeta struct {
    PolicyID        string `json:"policy_id,omitempty"`
    PolicyVersion   string `json:"policy_version,omitempty"`
    EvaluatedRules  int    `json:"evaluated_rules,omitempty"`
    EvaluationMs    int64  `json:"evaluation_ms,omitempty"`
}

// Engine 策略引擎接口
type Engine interface {
    // Evaluate 在指定钩子上评估策略
    Evaluate(ctx context.Context, hook DecisionHook, input Input) (*Decision, error)

    // Reload 热重载策略
    Reload() error

    // Close 释放资源
    Close() error
}
```

### NoopEngine

当 Policy Engine 禁用时，返回默认 allow 决策：

```go
type NoopEngine struct{}

func (NoopEngine) Evaluate(ctx context.Context, hook DecisionHook, input Input) (*Decision, error) {
    return &Decision{Allow: true}, nil
}
```

## 集成点

### 1. HTTP Middleware（鉴权后）

在 `cmd/gateway/main.go` 中构建 PolicyEngine，并作为中间件插入：

```go
// 新增 policy middleware
func PolicyMiddleware(engine policy.Engine, hook policy.DecisionHook) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            input := buildInputFromRequest(r)
            decision, err := engine.Evaluate(r.Context(), hook, input)
            if err != nil || !decision.Allow {
                writeDenyResponse(w, decision, err)
                return
            }
            next.ServeHTTP(w, r)
        })
    }
}
```

### 2. PII/Content Safety 集成

检测完成后触发 `preprocess.request` 钩子：

```go
// PII 检测后
if result.PIIDetected {
    input.Detection.PII = buildPIIInput(result)
    decision, _ := engine.Evaluate(ctx, policy.HookPreprocessRequest, input)
    if !decision.Allow {
        return denyResponse(decision)
    }
    // 执行 actions（如 redact_pii）
    applyActions(decision.Actions, requestBody)
}
```

### 3. 响应检查

上游响应返回后触发 `check.upstream_response` 钩子，可用于输出内容安全策略。

### 4. 路由决策

路由选择时触发 `route.select` 钩子，PolicyEngine 的 `RouteHint` 可影响 provider 选择。

## 策略热加载

### 文件监听

使用 `fsnotify` 监听 `policy_dir` 和 `data_dir`：

```go
watcher, _ := fsnotify.NewWatcher()
watcher.Add(cfg.PolicyDir)
watcher.Add(cfg.DataDir)

go func() {
    for event := range watcher.Events {
        if event.Op&fsnotify.Write != 0 || event.Op&fsnotify.Create != 0 {
            engine.Reload()  // 重新编译所有 Rego 策略
        }
    }
}()
```

### Reload 安全保证

- Reload 在后台 goroutine 中执行，不阻塞请求
- 新策略编译成功后原子替换旧策略
- 编译失败时保留旧策略，记录错误日志和 metric
- Reload 期间请求继续使用旧策略评估

## 可观测性

### Prometheus 指标（新增 8 个）

| 指标 | 类型 | Labels | 说明 |
|------|------|--------|------|
| `policy_evaluations_total` | Counter | hook, decision | 策略评估总次数 |
| `policy_evaluation_duration_seconds` | Histogram | hook | 策略评估延迟 |
| `policy_denials_total` | Counter | hook, policy_id | 策略拒绝次数 |
| `policy_evaluations_in_progress` | Gauge | hook | 当前评估中的请求数 |
| `policy_reloads_total` | Counter | status | 策略重载次数 |
| `policy_reload_duration_seconds` | Histogram | - | 策略重载耗时 |
| `policy_evaluation_errors_total` | Counter | hook, error_type | 评估错误次数 |
| `policy_rule_evaluations_total` | Counter | policy_id | 各策略规则评估次数 |

### OpenTelemetry Span

```go
ctx, span := tracer.Start(ctx, "policy.evaluate",
    trace.WithAttributes(
        attribute.String("policy.hook", string(hook)),
        attribute.Bool("policy.allow", decision.Allow),
    ),
)
defer span.End()
```

## 安全考虑

### 1. 策略评估超时

- 默认 50ms 超时，防止恶意策略导致请求堆积
- 超时后按 `default_decision` 处理（默认 allow，安全场景可设为 deny）

### 2. 策略隔离

- Rego 策略在沙箱中评估，无法执行系统调用
- 策略无法直接访问文件系统或网络
- `input` 文档显式传入，策略无法读取未提供的数据

### 3. 数据脱敏

- `input` 中的 PII 数据在传入策略前已脱敏
- 策略看到的是 `138****5678` 而非原始手机号
- 策略评估日志不记录完整请求体

### 4. 策略审计

- 每次 deny 决策写入审计日志（包含 policy_id 和 deny_reason）
- 策略文件变更记录到审计日志
- 策略 Reload 失败告警

## 测试计划

### 单元测试

- `engine_test.go`：Engine 接口实现测试
- `rego_policy_test.go`：内置策略包测试
- `config_test.go`：配置解析和验证
- `hot_reload_test.go`：热重载逻辑测试

### 策略测试（Rego 自带）

每个 `.rego` 策略文件配套 `_test.rego`：

```rego
package core.authz

test_work_hour_allowed {
    allow with input as {
        "time": {"hour": 10, "weekday": "Monday"},
        "client": {"name": "normal-app"}
    }
}

test_offhour_denied {
    not allow with input as {
        "time": {"hour": 22, "weekday": "Sunday"},
        "client": {"name": "intern-app"}
    }
}
```

### 集成测试

- 端到端：请求 → Policy 拒绝 → 403 响应
- 端到端：PII 检测 → 策略触发脱敏 → 上游收到脱敏请求
- 热重载：修改策略文件 → 新请求按新策略评估

### 性能测试

- 基准：单次评估 < 1ms（P99）
- 基准：热重载 < 100ms
- 压力：1000 req/s 下策略评估开销 < 5% CPU

## 实施阶段

| 阶段 | 内容 | 产出 | 依赖 |
|------|------|------|------|
| **1** | 核心接口与 OPA 集成 | `internal/policy/engine.go`、`types.go`、`noop.go` | OPA Go SDK |
| **2** | 配置结构与验证 | `internal/config/policy_engine_config.go` | 阶段 1 |
| **3** | 内置策略包 | `policies/core/*.rego` + 测试 | 阶段 1 |
| **4** | 策略热加载 | `internal/policy/reloader.go`、fsnotify | 阶段 1 |
| **5** | Middleware 集成 | `internal/middleware/policy.go` | 阶段 1-2 |
| **6** | 检测器集成 | PII/内容安全钩子触发 | 阶段 5 |
| **7** | 可观测性 | Prometheus 指标 + OTel Span | 阶段 1 |
| **8** | Dashboard API | 策略管理端点 | 阶段 1-4 |
| **9** | 文档与示例 | `docs/policy-engine.md` | 全部 |

## 依赖

### 新增依赖

- `github.com/open-policy-agent/opa` v0.65+ — OPA Go SDK
- `github.com/fsnotify/fsnotify` v1.7+ — 文件监听

### 现有依赖复用

- `github.com/prometheus/client_golang` — 指标
- `go.opentelemetry.io/otel` — 链路追踪
- `github.com/go-chi/chi/v5` — HTTP 路由

## 风险与缓解

| 风险 | 影响 | 缓解措施 |
|------|------|---------|
| Rego 学习曲线陡峭 | 策略编写困难 | 提供内置策略模板 + 文档 |
| OPA 评估性能瓶颈 | 请求延迟增加 | 50ms 超时 + 缓存常用决策 |
| 策略错误导致全量拒绝 | 服务不可用 | default_decision=allow + 策略预测试 |
| 策略文件并发修改 | Reload 竞态 | 原子替换 + RWMutex |
