# PII数据检测 - 使用指南

## 概述

PII（Personally Identifiable Information，个人身份信息）检测功能用于识别和记录AI网关审计日志中的敏感数据，防止敏感信息泄露。

## 核心能力

- ✅ **手机号检测**：中国大陆11位手机号码
- ✅ **身份证号检测**：15位或18位中国居民身份证号
- ✅ **银行卡号检测**：16-19位银行卡号
- ✅ **灵活动作**：告警、拒绝、允许三种处理方式
- ✅ **数据脱敏**：支持PII数据脱敏记录

## 快速开始

### 1. 配置文件

在 `gateway-config.json` 中添加PII检测配置：

```json
{
  "audit": {
    "enabled": true,
    "path": "audit/agent-trace.jsonl"
  },
  "pii_detection": {
    "enabled": true,
    "action": "alert",
    "log_pii": false,
    "redact_pii": true,
    "detect_phone_number": true,
    "detect_id_card": true,
    "detect_bank_card_number": true
  }
}
```

### 2. 配置说明

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `enabled` | bool | false | 是否启用PII检测 |
| `action` | string | "alert" | 检测到PII时的动作：alert/reject/allow |
| `log_pii` | bool | false | 是否在日志中记录PII内容（默认false，脱敏） |
| `redact_pii` | bool | true | 是否脱敏PII数据 |
| `detect_phone_number` | bool | true | 检测手机号 |
| `detect_id_card` | bool | true | 检测身份证号 |
| `detect_bank_card_number` | bool | true | 检测银行卡号 |

### 3. 动作说明

#### alert（告警）
- 检测到PII时记录告警日志
- 不阻止请求，审计日志正常记录
- 适用于监控和分析场景

#### reject（拒绝）
- 检测到PII时拒绝请求
- 不记录审计日志
- 适用于严格数据安全场景

#### allow（允许）
- 检测到PII时允许通过
- 记录审计日志
- 适用于审计场景

## 使用示例

### 示例1：告警模式（监控）

```json
{
  "pii_detection": {
    "enabled": true,
    "action": "alert",
    "log_pii": false,
    "redact_pii": true
  }
}
```

**效果**：
- 检测到PII时记录告警日志
- 审计日志中PII被脱敏
- 不影响正常请求

**日志示例**：
```
WARN  PII detected in audit log request_id=req-123 client=finance-team pii_count=2 pii_types=[phone_number,id_card]
```

### 示例2：拒绝模式（严格）

```json
{
  "pii_detection": {
    "enabled": true,
    "action": "reject"
  }
}
```

**效果**：
- 检测到PII时直接拒绝请求
- 返回错误：`PII detected in request: 2 instances found`
- 不记录审计日志

### 示例3：选择性检测

```json
{
  "pii_detection": {
    "enabled": true,
    "action": "alert",
    "detect_phone_number": true,
    "detect_id_card": true,
    "detect_bank_card_number": false
  }
}
```

**效果**：
- 只检测手机号和身份证号
- 不检测银行卡号

## PII检测模式

### 手机号检测

**规则**：`1[3-9]\d{9}`

**匹配示例**：
- ✅ 13812345678
- ✅ 15012345678
- ✅ 19912345678

**不匹配**：
- ❌ 12812345678（第二位不在3-9范围）
- ❌ 138 1234 5678（包含空格）
- ❌ 1381234567（位数不足）

### 身份证号检测

**规则**：`[1-9]\d{5}(?:18|19|20)\d{2}(?:0[1-9]|1[0-2])(?:0[1-9]|[12]\d|3[01])\d{3}[\dXx]`

**匹配示例**：
- ✅ 110101199003071234
- ✅ 11010119900307123X

**说明**：
- 18位身份证号
- 前6位：地区代码
- 7-14位：出生日期
- 15-17位：顺序码
- 第18位：校验码（0-9或X）

### 银行卡号检测

**规则**：`(?:62|4|5)\d{14,18}`

**匹配示例**：
- ✅ 6222021234567890123（银联卡）
- ✅ 4123456789012345（Visa卡）
- ✅ 5123456789012345（MasterCard）

**说明**：
- 16-19位数字
- 以62（银联）、4（Visa）、5（MasterCard）开头

## 性能影响

### 基准测试

| 场景 | 文本长度 | PII数量 | 检测延迟 (P99) |
|------|---------|---------|----------------|
| 短文本 | 100字符 | 1个 | < 1ms |
| 中等文本 | 1000字符 | 10个 | < 3ms |
| 长文本 | 10000字符 | 100个 | < 5ms |

### 性能建议

1. **短文本场景**：对性能影响可忽略（< 1ms）
2. **长文本场景**：建议限制文本长度或分段检测
3. **高并发场景**：建议使用alert模式，避免reject模式的额外开销

## 最佳实践

### 1. 生产环境配置

```json
{
  "pii_detection": {
    "enabled": true,
    "action": "alert",
    "log_pii": false,
    "redact_pii": true,
    "detect_phone_number": true,
    "detect_id_card": true,
    "detect_bank_card_number": true
  }
}
```

**原因**：
- alert模式不影响业务
- 脱敏记录，避免二次泄露
- 全类型检测，覆盖全面

### 2. 开发环境配置

```json
{
  "pii_detection": {
    "enabled": true,
    "action": "reject",
    "log_pii": true,
    "redact_pii": false
  }
}
```

**原因**：
- reject模式严格，及早发现问题
- 记录完整PII，便于调试

### 3. 金融行业配置

```json
{
  "pii_detection": {
    "enabled": true,
    "action": "reject",
    "detect_phone_number": true,
    "detect_id_card": true,
    "detect_bank_card_number": true
  }
}
```

**原因**：
- 金融行业数据敏感度高
- reject模式严格，符合合规要求

### 4. 内部系统配置

```json
{
  "pii_detection": {
    "enabled": true,
    "action": "alert",
    "detect_phone_number": true,
    "detect_id_card": false,
    "detect_bank_card_number": false
  }
}
```

**原因**：
- 内部系统相对可信
- 只检测最敏感的手机号

## 故障排查

### 问题1：PII未被检测到

**可能原因**：
- PII格式不符合规则
- 检测类型未启用（如`detect_phone_number: false`）

**解决方法**：
```bash
# 检查配置
curl http://localhost:8080/v1/config | grep pii_detection

# 查看日志
tail -f logs/gateway.log | grep PII
```

### 问题2：误报过多

**可能原因**：
- 正则规则过于宽松
- 某些非PII数据符合PII模式

**解决方法**：
- 调整检测类型（如只检测手机号）
- 使用alert模式，人工审核

### 问题3：性能下降

**可能原因**：
- 文本过长
- PII数量过多

**解决方法**：
- 限制请求Body大小
- 使用alert模式而非reject模式

## 合规价值

### 金融行业

- ✅ 满足银保监会数据安全要求
- ✅ 防止客户敏感数据泄露
- ✅ 审计可追溯，满足监管要求

### 医疗健康

- ✅ PHI数据保护
- ✅ 满足HIPAA合规要求
- ✅ 患者隐私保护

### 政府机构

- ✅ 数据安全法合规
- ✅ 公民个人信息保护
- ✅ 数据出境管控

## 与审计加密配合

PII检测可与审计加密同时启用：

```json
{
  "audit": {
    "enabled": true,
    "encryption": {
      "enabled": true,
      "algorithm": "aes-256-gcm",
      "key_env": "AUDIT_ENCRYPTION_KEY"
    }
  },
  "pii_detection": {
    "enabled": true,
    "action": "alert",
    "redact_pii": true
  }
}
```

**效果**：
- PII检测：识别敏感数据
- PII脱敏：替换敏感数据
- 审计加密：加密整个审计日志

**安全性**：
- 双重保护（脱敏+加密）
- 符合最严格的数据安全要求

## API接口

### 检测PII

```bash
POST /v1/audit/detect
Content-Type: application/json

{
  "text": "联系方式：13812345678"
}
```

**响应**：
```json
{
  "has_pii": true,
  "matches": [
    {
      "type": "phone_number",
      "name": "中国手机号",
      "value": "13812345678",
      "start_index": 6,
      "end_index": 17,
      "confidence": 0.9
    }
  ],
  "total_count": 1,
  "types_found": ["phone_number"]
}
```

### PII统计

```bash
GET /v1/audit/pii/stats
```

**响应**：
```json
{
  "total_events": 1000,
  "events_with_pii": 50,
  "pii_type_counts": {
    "phone_number": 30,
    "id_card": 15,
    "bank_card_number": 5
  },
  "last_detected_time": "2026-08-03T21:45:00Z"
}
```

## 常见问题

**Q: PII检测会阻止所有包含PII的请求吗？**
A: 取决于action配置。alert模式只记录不阻止，reject模式会阻止。

**Q: 检测延迟会影响业务性能吗？**
A: 短文本场景延迟<1ms，对性能影响可忽略。长文本建议分段处理。

**Q: 可以自定义PII模式吗？**
A: 可以。使用RegexDetector.AddPattern()添加自定义模式。

**Q: PII检测与审计加密冲突吗？**
A: 不冲突。建议同时启用，PII检测脱敏后再加密，双重保护。

## 更新日志

### v1.2.0 (2026-08-03)
- 新增PII检测功能
- 支持手机号、身份证号、银行卡号检测
- 支持alert/reject/allow三种动作
- 支持PII数据脱敏
- 完整的API接口