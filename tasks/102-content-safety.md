# Task 102: 内容安全检测

## 状态

Done

## 背景

在企业AI风险管控场景中，许多组织要求**数据不出域**，不能使用云端内容安全API。需要提供本地化的内容安全检测能力，帮助组织识别和管控AI交互中的敏感内容。

本任务实现基于关键词的内容安全检测能力，支持政治、色情、暴力、广告等类别检测，支持自定义关键词库，确保数据完全本地处理。

核心原则：
- **数据不出域**：所有检测在本地完成，无外部API调用
- **可定制**：用户可自定义敏感词库
- **低延迟**：关键词检测 < 1ms

相关文档：

- [Content Safety Detection Design](../docs/superpowers/specs/2026-08-04-content-safety-design.md)

## 范围

实现：

- 关键词内容安全检测器（政治、色情、暴力、广告）
- 自定义关键词库支持（文件加载、通配符匹配）
- 内容安全配置支持（JSON + 环境变量）
- 审计记录器集成内容安全检测
- 三种处理模式：alert / reject / allow
- 阈值级别支持：low / medium / high
- 完整的使用文档和合规指南
- 单元测试、集成测试、性能测试

暂不实现：

- ML-based 内容安全检测
- 图像/视频/音频内容安全
- 云端内容安全API集成
- 内容自动脱敏/遮蔽
- 多语言支持（仅中文）

## 接口行为

### 配置验证

必须校验：

- `action` 必须是 `alert`、`reject`、`allow` 之一。
- `categories` 必须是非空数组，且每个元素是合法的类别名。
- `threshold` 必须是 `low`、`medium`、`high` 之一。
- `custom_keywords.paths` 必须是有效的文件路径（如果提供）。
- 如果校验失败，网关启动必须失败并给出清晰错误信息。

### 内容安全类别

支持四种类别：

1. **politics（政治敏感）**
   - 检测：政治敏感词、政府相关词、政治人物名
   - 准确率：> 90%

2. **pornography（色情）**
   - 检测：性暗示词、成人内容术语
   - 准确率：> 95%

3. **violence（暴力）**
   - 检测：暴力语言、恐怖主义相关词、自残关键词
   - 准确率：> 90%

4. **advertising（广告）**
   - 检测：垃圾营销词、推广语言、竞争对手名（可配置）
   - 准确率：> 80%

### 自定义关键词库

**文件格式：**

```
# 这是注释
敏感词1
敏感词2
竞争对手名
```

**特性：**

- 支持 `#` 注释行
- 忽略空行
- 大小写不敏感
- 支持简单通配符：`敏感*` 匹配 `敏感词`、`敏感内容` 等

**阈值级别：**

- `low`：匹配 1 个关键词即触发
- `medium`：匹配 2+ 个关键词或 1 个通配符匹配
- `high`：匹配 3+ 个关键词或 2+ 个通配符匹配

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
   - 添加 `content_safety_violation` 字段到审计事件
   - 继续处理请求
   - 不修改请求/响应

2. **reject（拒绝模式）**
   - 在发送到上游provider前检测
   - 如果检测到违规，返回 `400 invalid_request_error`
   - 阻止敏感内容发送到AI provider
   - 最高保护级别

3. **allow（允许模式）**
   - 记录日志（info级别）
   - 不添加 `content_safety_violation` 字段
   - 继续处理请求
   - 最低影响级别

## 建议实现步骤

1. **关键词检测器开发**（1周）
   - 定义 `ContentSafetyDetector` 接口
   - 实现 `KeywordDetector`
   - 定义 `ContentSafetyMatch`、`ContentSafetyResult` 结构
   - 实现内置关键词列表（政治、色情、暴力、广告）
   - 编写完整单元测试
   - 编写性能测试

2. **自定义关键词库**（1周）
   - 实现关键词文件加载
   - 实现通配符匹配
   - 实现阈值级别逻辑
   - 编写测试

3. **配置结构修改**（3天）
   - 添加 `ContentSafetyConfig` 结构
   - 添加环境变量覆盖支持
   - 添加配置验证逻辑
   - 编写配置测试

4. **审计记录器集成**（1周）
   - 实现 `ContentSafetyAuditorRecorder` 包装器
   - 添加 `content_safety_violation` 字段到审计事件
   - 实现 alert/reject/allow 逻辑
   - 编写集成测试

5. **网关集成**（1周）
   - 在命令层集成内容安全检测器
   - 在API层实现 `action: reject`
   - 添加启动日志
   - 编写端到端测试

6. **文档完善**（1周）
   - 编写使用文档
   - 编写合规指南
   - 编写故障排查
   - 编写性能基准

## 验收标准

- 使用配置可以启用内容安全检测。
- 环境变量可以覆盖配置。
- 无效action导致启动失败。
- 空categories列表导致启动失败。
- 无效threshold导致启动失败。
- 检测器正确识别政治敏感词（准确率 > 90%）。
- 检测器正确识别色情词（准确率 > 95%）。
- 检测器正确识别暴力词（准确率 > 90%）。
- 检测器正确识别广告词（准确率 > 80%）。
- 自定义关键词库正确加载。
- 通配符匹配正确工作。
- 阈值级别正确应用。
- `action: alert` 正确记录告警。
- `action: reject` 正确拒绝请求。
- `action: allow` 正确记录信息。
- 检测延迟 < 1ms (P99)。
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

- `TestKeywordDetector_Politics`
- `TestKeywordDetector_Pornography`
- `TestKeywordDetector_Violence`
- `TestKeywordDetector_Advertising`
- `TestKeywordDetector_CustomKeywords`
- `TestKeywordDetector_WildcardMatching`
- `TestKeywordDetector_ThresholdLevels`
- `TestKeywordDetector_LargeText`
- `TestKeywordDetector_NoViolation`
- `TestContentSafetyConfig_Defaults`
- `TestContentSafetyConfig_EnvOverrides`
- `TestContentSafetyConfig_Validation`
- `TestContentSafetyAuditorRecorder_Alert`
- `TestContentSafetyAuditorRecorder_Reject`
- `TestContentSafetyAuditorRecorder_Allow`
- `TestContentSafetyAuditorRecorder_NoViolation`
- `TestGateway_ContentSafetyReject`

## 代码改动点

### 新增文件

- `internal/audit/content_safety_detector.go`：内容安全检测器接口和实现
- `internal/audit/content_safety_detector_test.go`：检测器测试
- `internal/audit/content_safety_auditor.go`：内容安全审计记录器包装器
- `internal/audit/keyword_loader.go`：关键词文件加载器
- `internal/audit/keywords/`：内置关键词列表目录
  - `politics.txt`
  - `pornography.txt`
  - `violence.txt`
  - `advertising.txt`
- `internal/config/content_safety_config.go`：内容安全检测配置
- `docs/content-safety.md`：使用文档

### 修改文件

- `internal/config/config.go`：添加 `ContentSafetyConfig` 字段
- `cmd/gateway/main.go`：
  - 在 `buildAuditRecorder()` 中构建内容安全检测器
  - 包装审计记录器
  - 添加启动日志

## 文档更新

完成本任务后更新：

- [ ] 本任务状态改为 `Done`
- [ ] README.md 添加内容安全检测说明
- [ ] docs/content-safety.md 完整使用文档

## 实施时间估算

- 关键词检测器开发：1周
- 自定义关键词库：1周
- 配置结构修改：3天
- 审计记录器集成：1周
- 网关集成：1周
- 文档完善：1周

**总计：约5周**

## 实施进度

### 阶段0：设计文档（进行中）

- ✅ Design Spec
- ✅ Task Plan
- ⏳ Git提交

### 阶段1：检测器开发（待开始）

- [ ] 定义 `ContentSafetyDetector` 接口
- [ ] 实现 `KeywordDetector`
- [ ] 实现 `ContentSafetyMatch`、`ContentSafetyResult` 结构
- [ ] 内置关键词列表
- [ ] 单元测试
- [ ] 性能测试

### 阶段2：自定义关键词库（待开始）

- [ ] 关键词文件加载
- [ ] 通配符匹配
- [ ] 阈值级别
- [ ] 测试

### 阶段3：配置结构修改（待开始）

- [ ] 添加 `ContentSafetyConfig`
- [ ] 环境变量支持
- [ ] 配置验证
- [ ] 测试

### 阶段4：审计记录器集成（待开始）

- [ ] 实现 `ContentSafetyAuditorRecorder`
- [ ] 添加 `content_safety_violation` 字段
- [ ] 实现 action 逻辑
- [ ] 集成测试

### 阶段5：网关集成（待开始）

- [ ] 命令层集成
- [ ] API层集成
- [ ] 启动日志
- [ ] 端到端测试

### 阶段6：文档完善（待开始）

- [ ] 使用文档
- [ ] 合规指南
- [ ] 故障排查

**当前状态：0%完成，设计文档已创建**