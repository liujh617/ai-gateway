# Task 100: 审计日志加密

## 状态

In Progress

## 背景

当前审计日志以明文JSONL格式存储完整的请求和响应体。对于需要合规和数据保护的场景（金融、政府、医疗），审计日志本身可能包含敏感数据（用户提示、AI生成内容、工具调用等），需要加密保护。

本任务实现可选的AES-256-GCM加密，保护审计日志中的敏感字段，支持私有化部署和合规要求。

相关文档：

- [Audit Encryption Design](../docs/superpowers/specs/2026-08-03-audit-encryption-design.md)
- [ADR: Audit Encryption](../docs/adr/audit-encryption-design.md)

## 范围

实现：

- AES-256-GCM加密器接口和实现
- 加密配置支持（JSON + 环境变量）
- 审计记录器集成加密功能
- 密钥生成工具（audit-keygen）
- 完整的使用文档和部署示例
- 单元测试、集成测试、性能测试

暂不实现：

- 密钥轮转机制
- 云KMS集成（AWS KMS, GCP KMS）
- 审计日志解密工具
- 归档日志加密
- 多租户密钥管理

## 接口行为

### 配置验证

必须校验：

- 如果 `encryption.enabled` 为 `true`，`key_env` 必须非空。
- 环境变量值必须是合法的base64字符串。
- Base64解码后的密钥必须正好32字节。
- 如果校验失败，网关启动必须失败并给出清晰错误信息。

### 加密范围

加密字段：

- `request_body`（完整请求体JSON）
- `response_body`（完整响应体JSON）

不加密字段：

- `timestamp`、`event`、`request_id`、`trace_id`等元数据字段

加密后格式：

- 添加 `body_encrypted: true` 字段
- 加密字段前缀 `aes-256-gcm:` + base64密文

### 加密行为

- 每次加密生成随机12字节nonce。
- 密文格式：`nonce(12) + ciphertext + tag(16)`。
- 相同明文每次加密产生不同密文。
- 加密失败时记录错误但不失败请求（fail-open）。

## 建议实现步骤

1. **加密模块开发**（1周）
   - 定义 `Encryptor` 接口
   - 实现 `AES256GCMEncryptor`
   - 实现 `NoopEncryptor`
   - 编写完整单元测试
   - 编写性能测试

2. **配置结构修改**（3天）
   - 添加 `AuditEncryptionConfig` 结构
   - 添加环境变量覆盖支持
   - 添加配置验证逻辑
   - 编写配置测试

3. **审计记录器集成**（3天）
   - 修改 `JSONLRecorder` 添加 `encryptor` 字段
   - 在 `Record()` 方法中加密 Body 字段
   - 添加 `body_encrypted` 字段
   - 编写集成测试

4. **初始化逻辑**（3天）
   - 在 `buildAuditRecorder()` 中加载密钥
   - 构建加密器实例
   - 注入到审计记录器
   - 添加启动日志
   - 编写端到端测试

5. **密钥生成工具**（2天）
   - 实现 `cmd/tools/audit-keygen`
   - 输出base64编码密钥
   - 提供配置示例
   - 编写工具测试

6. **文档完善**（3天）
   - 编写使用指南（docs/audit-encryption.md）
   - 编写部署示例（Docker、Kubernetes、Systemd）
   - 编写故障排查指南
   - 编写性能基准

## 验收标准

- 使用配置可以启用审计加密。
- 环境变量可以覆盖配置。
- 无效密钥长度导致启动失败。
- 加密后的审计日志包含 `body_encrypted: true`。
- 加密密文可以正确解密还原。
- 加密失败不导致请求失败。
- 加密延迟 < 10ms (P99)。
- 所有单元测试和集成测试通过。
- 密钥生成工具正常工作。
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

- `TestAES256GCMEncryptor_EncryptDecrypt`
- `TestAES256GCMEncryptor_InvalidKey`
- `TestAES256GCMEncryptor_RandomNonce`
- `TestAuditEncryptionConfig_Defaults`
- `TestAuditEncryptionConfig_EnvOverrides`
- `TestAuditEncryptionConfig_Validation`
- `TestJSONLRecorder_Encryption`
- `TestJSONLRecorder_EncryptionFailure`
- `TestJSONLRecorder_LargeBodyEncryption`
- `TestBuildAuditRecorder_EncryptionEnabled`
- `TestBuildAuditRecorder_MissingKey`
- `TestAuditKeygen`

## 代码改动点

### 新增文件

- `internal/audit/encryption.go`：加密器接口和实现
- `internal/audit/encryption_test.go`：加密器测试
- `internal/config/audit_encryption_config.go`：加密配置（可选，或直接修改config.go）
- `cmd/tools/audit-keygen/main.go`：密钥生成工具
- `docs/audit-encryption.md`：使用文档

### 修改文件

- `internal/config/config.go`：添加 `AuditEncryptionConfig` 字段
- `internal/audit/audit.go`：
  - 添加 `Encryptor` 字段到 `JSONLRecorder`
  - 添加 `body_encrypted` 字段到 `Event`
  - 在 `Record()` 方法中加密 Body
- `cmd/gateway/main.go`：
  - 在 `buildAuditRecorder()` 中加载密钥
  - 构建加密器
  - 注入到审计记录器

## 文档更新

完成本任务后更新：

- [ ] 本任务状态改为 `Done`
- [ ] README.md 添加审计加密说明
- [ ] docs/audit-encryption.md 完整使用文档
- [ ] docs/adr/audit-encryption-design.md 架构决策记录（已有）

## 实施时间估算

- 加密模块开发：1周
- 配置结构修改：3天
- 审计记录器集成：3天
- 初始化逻辑：3天
- 密钥生成工具：2天
- 文档完善：3天

**总计：约3周**

## 实施进度

### 阶段1：加密模块（已完成）

- ✅ 定义 `Encryptor` 接口
- ✅ 实现 `AES256GCMEncryptor`
- ✅ 实现 `NoopEncryptor`
- ✅ 单元测试通过
- ✅ 性能测试通过
- ✅ Git提交：`746da07`

### 阶段2：配置结构（已完成）

- ✅ 添加 `AuditEncryptionConfig`
- ✅ 环境变量支持
- ✅ 配置验证
- ✅ 测试通过
- ✅ Git提交：`d82570f`

### 阶段3：审计记录器集成（已完成）

- ✅ 修改 `JSONLRecorder`
- ✅ 加密 Body 字段
- ✅ 添加 `body_encrypted` 字段
- ✅ 集成测试通过
- ✅ Git提交：`375c2c9`

### 阶段4：初始化逻辑（已完成）

- ✅ 密钥加载
- ✅ 加密器构建
- ✅ 启动日志
- ✅ Git提交：`2ecd47b`

### 阶段5：密钥工具（已完成）

- ✅ `audit-keygen` 工具
- ✅ Git提交：`45c2a27`

### 阶段6：文档完善（已完成）

- ✅ 使用文档
- ✅ Git提交：`a117036`

**当前状态：100%完成，已合并到feature分支，等待发布**