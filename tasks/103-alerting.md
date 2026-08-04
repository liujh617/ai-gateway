# Task 103: 告警系统

## 状态

Done

## 背景

在AI风险管控场景中，检测到风险后需要及时通知管理员，形成"审计 → 检测 → **告警**"的完整闭环。告警系统支持多种通知渠道（Webhook、Email、Slack），确保安全事件得到及时响应。

核心价值：
- **及时响应**：检测到PII泄露、内容安全违规时立即通知
- **多渠道支持**：Webhook、Email、Slack等灵活选择
- **防止告警风暴**：速率限制和冷却机制
- **可追溯**：所有告警记录持久化存储

相关文档：

- [Alerting System Design](../docs/superpowers/specs/2026-08-04-alerting-design.md)

## 范围

实现：

- 告警管理器（AlertManager）
- 三种告警渠道：Webhook、Email、Slack
- 告警规则配置（来源、严重级别、冷却时间）
- 速率限制（防止告警风暴）
- 重试机制（处理临时故障）
- 告警存储（JSONL格式持久化）
- PII检测集成告警
- 内容安全检测集成告警
- 完整的单元测试和集成测试

暂不实现：

- SMS短信告警
- 自动修复/响应机制
- 告警分析仪表板
- 工单系统集成（Jira、ServiceNow）
- 告警升级策略

## 接口行为

### 配置验证

必须校验：

- `channels` 必须是非空数组（如果enabled为true）。
- 每个channel的`type`必须是`webhook`、`email`、`slack`之一。
- 每个channel必须有对应type所需的配置字段。
- `rules` 必须是非空数组（如果enabled为true）。
- 每个rule引用的channel必须存在。
- 如果校验失败，网关启动必须失败并给出清晰错误信息。

### 告警渠道

#### Webhook Channel

- 支持自定义HTTP endpoint
- 支持自定义headers（如Authorization）
- 超时时间可配置（默认10秒）
- 成功：状态码200-299
- 失败：其他状态码或超时，触发重试

#### Email Channel

- 支持SMTP发送
- 支持TLS加密
- 支持自定义邮件模板
- 收件人可配置多个
- 环境变量支持密码（`${SMTP_PASSWORD}`）

#### Slack Channel

- 支持Slack Incoming Webhook
- 支持自定义channel、username、icon
- 使用Attachment格式美化消息
- 根据严重级别使用不同颜色（high=danger, medium=warning, low=good）

### 告警规则

支持配置：

- `name`: 规则名称
- `source`: 告警来源（`pii_detection`、`content_safety`）
- `severity`: 严重级别（`low`、`medium`、`high`、`critical`）
- `cooldown_minutes`: 冷却时间（防止重复告警）
- `channels`: 通知渠道列表
- `filters`: 可选过滤条件

### 速率限制

- 全局速率限制（跨所有来源）
- Token bucket算法
- 配置：`max_alerts_per_minute`（默认10）
- 超出限制的告警被丢弃（但记录到日志）

### 重试机制

- 处理临时故障（网络错误、超时、5xx状态码）
- 不重试永久性错误（认证失败、配置错误、4xx状态码）
- 最大重试次数可配置（默认3次）
- 指数退避：`delay = base_delay * 2^retry_count`

### 告警存储

- JSONL格式（每行一个告警）
- 文件轮转逻辑（与审计日志相同）
- 持久化所有告警（包括发送失败的）
- 用于审计和分析

## 验收标准

### 功能测试

- [ ] Webhook channel成功发送告警
- [ ] Email channel成功发送告警
- [ ] Slack channel成功发送告警
- [ ] 速率限制生效（超出限制的告警被丢弃）
- [ ] 冷却机制生效（冷却期内的重复告警被抑制）
- [ ] 重试机制生效（临时故障后重试成功）
- [ ] 告警存储成功写入JSONL文件
- [ ] PII检测触发告警
- [ ] 内容安全检测触发告警

### 性能测试

- [ ] 告警生成延迟 < 5ms P99
- [ ] Webhook发送延迟 < 500ms P99
- [ ] 内存占用 < 50MB（包括队列和缓存）

### 错误处理

- [ ] 配置校验失败时网关启动失败
- [ ] Webhook超时返回错误
- [ ] Email认证失败返回错误
- [ ] Slack webhook错误返回错误
- [ ] 存储写入失败记录错误

## 实施计划

### 阶段1：告警核心结构（1天）

- 定义Alert结构体
- 定义AlertManager接口
- 定义Channel接口
- 定义AlertRule结构体

### 阶段2：告警渠道实现（2天）

- 实现WebhookChannel
- 实现EmailChannel
- 实现SlackChannel
- 单元测试

### 阶段3：告警管理器（2天）

- 实现AlertManager
- 实现速率限制
- 实现冷却机制
- 实现重试逻辑
- 单元测试

### 阶段4：告警存储（1天）

- 实现AlertStorage（JSONL写入）
- 文件轮转逻辑
- 单元测试

### 阶段5：集成和配置（2天）

- 配置结构（AlertingConfig）
- PII检测集成
- 内容安全检测集成
- 网关启动集成
- 环境变量支持

### 阶段6：文档和测试（1天）

- 使用文档
- 部署指南
- 集成测试
- 性能测试

## 测试清单

### 单元测试

- `TestWebhookChannel_Send`: 测试webhook发送
- `TestEmailChannel_Send`: 测试email发送
- `TestSlackChannel_Send`: 测试slack发送
- `TestAlertManager_CreateAlert`: 测试告警创建
- `TestAlertManager_RateLimit`: 测试速率限制
- `TestAlertManager_Cooldown`: 测试冷却机制
- `TestAlertManager_Retry`: 测试重试逻辑
- `TestAlertStorage_Write`: 测试告警存储

### 集成测试

- `TestPIIDetection_Alerting`: PII检测 → 告警流程
- `TestContentSafety_Alerting`: 内容安全 → 告警流程
- `TestEndToEnd_Alerting`: 端到端告警流程

### 性能测试

- `BenchmarkAlertManager_CreateAlert`: 告警生成性能
- `BenchmarkWebhookChannel_Send`: Webhook发送性能

## 风险和依赖

### 风险

1. **外部依赖**: Webhook、Email、Slack依赖外部服务，可能不可用
   - 缓解：重试机制、超时控制、降级策略

2. **告警风暴**: 大量告警可能淹没系统
   - 缓解：速率限制、冷却机制

3. **敏感信息泄露**: 告警可能包含PII样本
   - 缓解：仅记录脱敏后的样本（如 `138****5678`）

### 依赖

- PII检测功能（已完成）
- 内容安全检测功能（已完成）
- 审计日志系统（已完成）
- 配置系统（已完成）

## 时间估算

- 阶段1：1天
- 阶段2：2天
- 阶段3：2天
- 阶段4：1天
- 阶段5：2天
- 阶段6：1天

**总计：约9天（1.5周）**

## 成功指标

- 告警生成延迟 < 5ms P99
- 告警发送延迟 < 500ms P99
- 告警成功率 > 99%（在渠道可用时）
- 内存占用 < 50MB
- 零告警丢失（在存储可用时）