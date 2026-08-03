# Task 101: PII数据检测

## 状态

In Progress

## 背景

在金融、政府、医疗等合规要求高的场景中，AI请求和响应可能包含敏感的PII（个人身份信息）数据。需要在审计层面检测PII数据流动，帮助组织识别和管控敏感数据暴露风险。

本任务实现基于正则表达式的PII检测能力，支持中国手机号、身份证号、银行卡号检测，为合规审计提供基础能力。

相关文档：

- [PII Detection Design](../docs/superpowers/specs/2026-08-04-pii-detection-design.md)
- [ADR: PII Detection](../docs/adr/pii-detection-design.md)

## 范围

实现：

- 正则表达式PII检测器（手机号、身份证、银行卡）
- PII检测配置支持（JSON + 环境变量）
- 审计记录器集成PII检测
- 三种处理模式：alert / reject / allow
- 完整的使用文档和合规指南
- 单元测试、集成测试、性能测试

暂不实现：

- ML-based PII检测
- 更多PII类型（邮箱、信用卡、IP地址）
- 自定义PII模式
- PII脱敏/遮蔽
- 外部DLP系统集成
- 多模态内容PII检测

## 接口行为

### 配置验证

必须校验：

- `action` 必须是 `alert`、`reject`、`allow` 之一。
- `patterns` 必须是非空数组，且每个元素是合法的模式名。
- 如果校验失败，网关启动必须失败并给出清晰错误信息。

### PII模式

支持三种模式：

1. **phone（中国手机号）**
   - 模式：`1[3-9]\d{9}`
   - 匹配：11位中国手机号
   - 准确率：> 95%

2. **id_card（中国身份证号）**
   - 模式：18位身份证号正则
   - 匹配：18位中国居民身份证号
   - 准确率：> 98%

3. **bank_card（中国银行卡号）**
   - 模式：`[1-9]\d{15,18}`
   - 匹配：16-19位银行卡号
   - 准确率：> 90%

### 检测范围

检测字段：

- `request_body.messages[].content`（用户提示）
- `response_body.choices[].message.content`（AI生成内容）

不检测：

- 元数据字段（timestamp、request_id等）
- HTTP头部
- URL参数

### 处理模式

1. **alert（告警模式）**
   - 记录日志（warning级别）
   - 添加 `pii_detected` 字段到审计事件
   - 继续处理请求
   - 不修改请求/响应

2. **reject（拒绝模式）**
   - 在发送到上游provider前检测PII
   - 如果检测到PII，返回 `400 invalid_request_error`
   - 阻止PII数据发送到AI provider
   - 最高保护级别

3. **allow（允许模式）**
   - 记录日志（info级别）
   - 不添加 `pii_detected` 字段
   - 继续处理请求
   - 最低影响级别

## 建议实现步骤

1. **检测器开发**（1周）
   - 定义 `Detector` 接口
   - 实现 `RegexDetector`
   - 定义 `PIIMatch`、`PIIDetectionResult` 结构
   - 编写完整单元测试
   - 编写性能测试

2. **配置结构修改**（3天）
   - 添加 `PIIDetectionConfig` 结构
   - 添加环境变量覆盖支持
   - 添加配置验证逻辑
   - 编写配置测试

3. **审计记录器集成**（1周）
   - 实现 `PIIAuditorRecorder` 包装器
   - 添加 `pii_detected` 字段到审计事件
   - 实现 alert/reject/allow 逻辑
   - 编写集成测试

4. **网关集成**（1周）
   - 在命令层集成PII检测器
   - 在API层实现 `action: reject`
   - 添加启动日志
   - 编写端到端测试

5. **文档完善**（1周）
   - 编写使用文档
   - 编写合规指南
   - 编写故障排查
   - 编写性能基准

## 验收标准

- 使用配置可以启用PII检测。
- 环境变量可以覆盖配置。
- 无效action导致启动失败。
- 空patterns列表导致启动失败。
- 检测器正确识别中国手机号（准确率 > 95%）。
- 检测器正确识别中国身份证号（准确率 > 98%）。
- 检测器正确识别中国银行卡号（准确率 > 90%）。
- `action: alert` 正确记录告警。
- `action: reject` 正确拒绝请求。
- `action: allow` 正确记录信息。
- 检测延迟 < 5ms (P99)。
- 所有单元测试和集成测试通过。
- 文档完整且清晰。

## 验证环境

标准验证环境：

- WSL distro: `Ubuntu-24.04`
- Repo path: `/mnt/e/code/ai-gateway`
- Shell: bash

建议验证命令：

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/ai-gateway -- bash -lc "go test ./internal/audit ./internal/config -v"
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/ai-gateway -- bash -lc "make verify"
```

## 测试清单

- `TestRegexDetector_Phone`
- `TestRegexDetector_IDCard`
- `TestRegexDetector_BankCard`
- `TestRegexDetector_MultiplePII`
- `TestRegexDetector_LargeText`
- `TestRegexDetector_NoPII`
- `TestPIIDetectionConfig_Defaults`
- `TestPIIDetectionConfig_EnvOverrides`
- `TestPIIDetectionConfig_Validation`
- `TestPIIAuditorRecorder_Alert`
- `TestPIIAuditorRecorder_Reject`
- `TestPIIAuditorRecorder_Allow`
- `TestPIIAuditorRecorder_NoPII`
- `TestGateway_PIIReject`

## 代码改动点

### 新增文件

- `internal/audit/pii_detector.go`：PII检测器接口和实现
- `internal/audit/pii_detector_test.go`：检测器测试
- `internal/audit/pii_auditor.go`：PII审计记录器包装器
- `internal/config/pii_detection_config.go`：PII检测配置
- `docs/pii-detection.md`：使用文档

### 修改文件

- `internal/config/config.go`：添加 `PIIDetectionConfig` 字段
- `cmd/gateway/main.go`：
  - 在 `buildAuditRecorder()` 中构建PII检测器
  - 包装审计记录器
  - 添加启动日志

## 文档更新

完成本任务后更新：

- [ ] 本任务状态改为 `Done`
- [ ] README.md 添加PII检测说明
- [ ] docs/pii-detection.md 完整使用文档
- [ ] docs/adr/pii-detection-design.md 架构决策记录（已有）

## 实施时间估算

- 检测器开发：1周
- 配置结构修改：3天
- 审计记录器集成：1周
- 网关集成：1周
- 文档完善：1周

**总计：约5周**

## 实施进度

### 阶段1：检测器开发（已完成）

- ✅ 定义 `Detector` 接口
- ✅ 实现 `RegexDetector`
- ✅ 实现 `PIIMatch`、`PIIDetectionResult` 结构
- ✅ 单元测试通过
- ✅ 性能测试通过
- ✅ Git提交：`bcabb41`

### 阶段2：配置结构修改（进行中）

- ⏳ 添加 `PIIDetectionConfig`
- ⏳ 环境变量支持
- ⏳ 配置验证
- ⏳ 测试编写

### 阶段3：审计记录器集成（待开始）

- ⏳ 实现 `PIIAuditorRecorder`
- ⏳ 添加 `pii_detected` 字段
- ⏳ 实现 action 逻辑
- ⏳ 集成测试

### 阶段4：网关集成（待开始）

- ⏳ 命令层集成
- ⏳ API层集成
- ⏳ 启动日志
- ⏳ 端到端测试

### 阶段5：文档完善（待开始）

- ⏳ 使用文档
- ⏳ 合规指南
- ⏳ 故障排查

**当前状态：20%完成（检测器核心已实现）**