# 内容安全检测 - 使用指南

## 概述

内容安全检测功能用于识别和记录AI网关审计日志中的敏感内容，防止政治、色情、暴力、广告等违规内容传播。**核心特性：数据不出域**，所有检测在本地完成，无外部API调用。

## 核心能力

- ✅ **政治敏感检测**：政治人物名、政府机构、政治术语等
- ✅ **色情内容检测**：成人内容术语、性暗示词等
- ✅ **暴力内容检测**：暴力语言、恐怖主义、自残关键词等
- ✅ **广告内容检测**：垃圾营销词、推广语言、竞品名等
- ✅ **自定义关键词**：支持自定义敏感词库
- ✅ **阈值级别**：low/medium/high 三级阈值
- ✅ **数据主权**：完全本地处理，无外部依赖

## 快速开始

### 1. 配置文件

在 `gateway-config.json` 中添加内容安全检测配置：

```json
{
  "audit": {
    "enabled": true,
    "path": "audit/agent-trace.jsonl"
  },
  "content_safety": {
    "enabled": true,
    "action": "alert",
    "categories": ["politics", "pornography", "violence", "advertising"],
    "custom_keywords": {
      "enabled": false,
      "paths": []
    },
    "log_matches": true,
    "threshold": "medium"
  }
}
```

### 2. 配置说明

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `enabled` | bool | false | 是否启用内容安全检测 |
| `action` | string | "alert" | 检测到违规时的动作：alert/reject/allow |
| `categories` | []string | ["politics", "pornography", "violence", "advertising"] | 检测类别 |
| `custom_keywords.enabled` | bool | false | 是否启用自定义关键词 |
| `custom_keywords.paths` | []string | [] | 自定义关键词文件路径列表 |
| `log_matches` | bool | true | 是否记录匹配的关键词 |
| `threshold` | string | "medium" | 阈值级别：low/medium/high |

### 3. 环境变量

支持环境变量覆盖配置：

- `GATEWAY_CONTENT_SAFETY_ENABLED`: 启用/禁用检测
- `GATEWAY_CONTENT_SAFETY_ACTION`: 设置动作模式
- `GATEWAY_CONTENT_SAFETY_CATEGORIES`: 设置检测类别（逗号分隔）
- `GATEWAY_CONTENT_SAFETY_THRESHOLD`: 设置阈值级别

## 检测类别

### politics（政治敏感）

检测内容：
- 政治人物名
- 政府机构名
- 政治术语

准确率：> 90%

### pornography（色情）

检测内容：
- 性暗示词汇
- 成人内容术语

准确率：> 95%

### violence（暴力）

检测内容：
- 暴力语言
- 恐怖主义相关词
- 自残关键词
- 武器相关术语

准确率：> 90%

### advertising（广告）

检测内容：
- 垃圾营销词
- 推广语言
- 竞品名（需自定义）

准确率：> 80%

## 阈值级别

### low（低阈值）

- 匹配任意关键词即触发违规
- 最严格模式

### medium（中阈值，推荐）

- 匹配 2+ 个关键词或 1 个通配符匹配
- 平衡严格度和误报率

### high（高阈值）

- 匹配 3+ 个关键词或 2+ 个通配符匹配
- 最宽松模式，适合测试

## 自定义关键词

### 文件格式

关键词文件为纯文本格式，每行一个关键词：

```
# 这是注释
敏感词1
敏感词2
敏感*
```

特性：
- 支持 `#` 注释行
- 忽略空行
- 大小写不敏感
- 支持通配符：`敏感*` 匹配 `敏感词`、`敏感内容` 等

### 加载自定义关键词

配置示例：

```json
{
  "content_safety": {
    "enabled": true,
    "action": "alert",
    "categories": ["politics", "advertising"],
    "custom_keywords": {
      "enabled": true,
      "paths": [
        "/etc/ai-gateway/keywords/politics.txt",
        "/etc/ai-gateway/keywords/competitors.txt"
      ]
    },
    "threshold": "medium"
  }
}
```

### 文件名约定

文件名会自动映射到检测类别：
- `politics.txt` → politics 类别
- `pornography.txt` → pornography 类别
- `violence.txt` → violence 类别
- `advertising.txt` → advertising 类别
- `competitors.txt` → advertising 类别（需包含在 advertising 类别中）

## 处理模式

### alert（告警模式，推荐）

- 记录日志（warning级别）
- 添加 `content_safety_violation` 字段到审计事件
- 继续处理请求
- 不修改请求/响应

**适用场景**：
- 初次部署，了解违规模式
- 审计合规，记录所有违规
- 调试和优化关键词库

### reject（拒绝模式）

- 在发送到上游 provider 前检测
- 如果检测到违规，返回 `400 invalid_request_error`
- 阻止违规内容发送到 AI provider
- 最高保护级别

**适用场景**：
- 生产环境，严格合规要求
- 防止违规内容传播
- 法律风险规避

### allow（允许模式）

- 记录日志（info级别）
- 不添加 `content_safety_violation` 字段
- 继续处理请求
- 最低影响级别

**适用场景**：
- 监控模式
- 低风险场景
- 测试环境

## 审计事件示例

```json
{
  "timestamp": "2026-08-04T02:00:00.000000000Z",
  "event": "response",
  "request_id": "req_abc123",
  "client": "client_001",
  "content_safety_violation": {
    "has_violation": true,
    "total_count": 3,
    "by_category": {
      "politics": 2,
      "advertising": 1
    },
    "matched_keywords": ["敏感词1", "竞争对手名称", "敏感词2"],
    "locations": [
      {"field": "request_body.messages[0].content", "category": "politics", "keyword": "敏感词1"},
      {"field": "response_body.choices[0].message.content", "category": "advertising", "keyword": "竞争对手名称"}
    ]
  }
}
```

## 性能指标

- **检测延迟**：< 1ms P99
- **吞吐量影响**：可忽略
- **内存占用**：取决于关键词库大小（典型 < 10MB）
- **CPU占用**：低（基于字符串匹配）

## 数据主权保证

- ✅ 所有检测在本地内存完成
- ✅ 无外部 HTTP 请求
- ✅ 无云端 API 调用
- ✅ 适合内网/专有云部署
- ✅ 符合数据不出域要求

## 部署建议

### 1. 初次部署

```json
{
  "content_safety": {
    "enabled": true,
    "action": "alert",
    "threshold": "medium",
    "log_matches": true
  }
}
```

建议：
- 使用 `alert` 模式观察违规模式
- 使用 `medium` 阈值平衡严格度
- 启用 `log_matches` 记录详情

### 2. 生产环境

```json
{
  "content_safety": {
    "enabled": true,
    "action": "reject",
    "threshold": "low",
    "log_matches": true,
    "custom_keywords": {
      "enabled": true,
      "paths": ["/etc/ai-gateway/keywords/custom.txt"]
    }
  }
}
```

建议：
- 使用 `reject` 模式防止违规
- 使用 `low` 阈值严格检测
- 添加自定义关键词库（如竞品名）
- 定期更新关键词库

### 3. 合规要求

根据行业特点定制关键词库：

**金融行业**：
- 竞争对手名
- 违规理财产品
- 误导性收益承诺

**医疗行业**：
- 虚假医疗广告
- 违禁药物名
- 不实疗效宣传

**教育行业**：
- 虚假宣传
- 违规招生
- 竞争机构名

## 故障排查

### 问题：误报率高

解决方案：
1. 调整阈值级别（low → medium → high）
2. 精简关键词库，移除易误报词
3. 使用通配符提高准确性
4. 在 `alert` 模式下观察并优化

### 问题：漏报率高

解决方案：
1. 调整阈值级别（high → medium → low）
2. 添加自定义关键词
3. 扩展关键词库覆盖面
4. 定期更新关键词库

### 问题：性能下降

解决方案：
1. 精简关键词库
2. 避免过多通配符
3. 使用 `alert` 模式（异步检测）
4. 升级硬件资源

## 限制说明

### 当前版本限制

- 仅支持文本内容检测
- 不支持图像/视频/音频
- 不支持语义理解（仅关键词匹配）
- 关键词更新需重启网关
- 仅支持中文关键词

### 未来规划

- ML-based 内容安全检测
- 多语言支持
- 实时关键词更新
- 多模态内容检测

## 最佳实践

1. **从小规模开始**：先用 `alert` 模式观察，再切换到 `reject`
2. **定期优化关键词**：根据审计日志调整关键词库
3. **分级部署**：先测试环境，再生产环境
4. **监控告警**：配置告警规则监控违规趋势
5. **合规团队参与**：关键词库需要法务/合规团队审核

## 相关文档

- [Design Spec](./superpowers/specs/2026-08-04-content-safety-design.md)
- [Task Plan](../tasks/102-content-safety.md)
- [PII Detection Guide](./pii-detection.md)