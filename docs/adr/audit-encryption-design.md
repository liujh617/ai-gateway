# 审计日志加密设计方案

## 目标

为ai-gateway的审计日志添加加密能力，防止审计日志泄露导致的二次风险。

## 核心需求

1. **加密字段**：request_body和response_body（包含敏感数据）
2. **加密算法**：AES-256-GCM（认证加密，安全性高）
3. **密钥管理**：环境变量配置，易于管理
4. **性能要求**：加密/解密延迟 < 10ms（P99）
5. **向后兼容**：不破坏现有审计日志格式

---

## 当前实现分析

### 审计日志结构

**位置**：`internal/audit/audit.go`

**Event结构**：
```go
type Event struct {
	Timestamp          time.Time       `json:"timestamp"`
	Event              string          `json:"event"`
	RequestID          string          `json:"request_id,omitempty"`
	TraceID            string          `json:"trace_id,omitempty"`
	Path               string          `json:"path,omitempty"`
	Client             string          `json:"client,omitempty"`
	ExternalModel      string          `json:"external_model,omitempty"`
	PreviousResponseID string          `json:"previous_response_id,omitempty"`
	Provider           string          `json:"provider,omitempty"`
	UpstreamModel      string          `json:"upstream_model,omitempty"`
	Status             int             `json:"status,omitempty"`
	DurationMS         int64           `json:"duration_ms,omitempty"`
	Body               json.RawMessage `json:"body,omitempty"`  // ← 需要加密
	Error              string          `json:"error,omitempty"`
}
```

**写入流程**（JSONLRecorder.Record）：
```
Event → json.Marshal(event) → Write to file
```

---

## 加密设计方案

### 方案一：加密Body字段（推荐）

**实现点**：在json.Marshal之前，加密Body字段

**优点**：
- ✅ 只加密敏感字段（性能更好）
- ✅ 其他字段仍可读（便于调试）
- ✅ 向后兼容（JSONL格式不变）

**实现步骤**：
1. 在Record方法中，判断加密是否启用
2. 如果启用，加密event.Body
3. 将加密后的数据替换event.Body
4. 继续执行json.Marshal和写入

**加密后Event示例**：
```json
{
  "timestamp": "2026-08-03T21:05:00Z",
  "event": "request",
  "request_id": "req-xxx",
  "client": "finance-team",
  "provider": "openai",
  "body_encrypted": "base64-encoded-ciphertext",
  "body_encryption": "aes-256-gcm"
}
```

---

### 方案二：加密整个JSONL行（不推荐）

**实现点**：在Write之前，加密整行

**缺点**：
- ❌ 整个审计日志不可读（调试困难）
- ❌ 性能开销大（加密整个Event）
- ❌ JSONL格式破坏

---

## 详细设计（方案一）

### 1. 配置结构

**文件**：`internal/config/config.go`

**新增配置**：
```go
type AuditConfig struct {
	Enabled      bool   `json:"enabled"`
	Path         string `json:"path"`
	MaxFileBytes int64  `json:"max_file_bytes"`

	// 新增
	Encryption   AuditEncryptionConfig `json:"encryption"`
}

type AuditEncryptionConfig struct {
	Enabled    bool   `json:"enabled"`
	Algorithm  string `json:"algorithm"`  // "aes-256-gcm"
	KeyEnv     string `json:"key_env"`    // 环境变量名
}
```

**配置示例**：
```json
{
  "audit": {
    "enabled": true,
    "path": "audit/agent-trace.jsonl",
    "encryption": {
      "enabled": true,
      "algorithm": "aes-256-gcm",
      "key_env": "AUDIT_ENCRYPTION_KEY"
    }
  }
}
```

---

### 2. 加密模块

**新文件**：`internal/audit/encryption.go`

**核心接口**：
```go
package audit

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
)

// Encryptor 审计日志加密器接口
type Encryptor interface {
	Encrypt(plaintext []byte) (ciphertext string, err error)
	Decrypt(ciphertext string) (plaintext []byte, err error)
}

// AES256GCMEncryptor AES-256-GCM加密器
type AES256GCMEncryptor struct {
	key []byte
}

func NewAES256GCMEncryptor(key string) (*AES256GCMEncryptor, error) {
	if len(key) != 32 {
		return nil, errors.New("key must be 32 bytes")
	}
	return &AES256GCMEncryptor{key: []byte(key)}, nil
}

func (e *AES256GCMEncryptor) Encrypt(plaintext []byte) (string, error) {
	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func (e *AES256GCMEncryptor) Decrypt(ciphertext string) ([]byte, error) {
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(e.key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}
```

---

### 3. 修改JSONLRecorder

**文件**：`internal/audit/audit.go`

**修改点**：
1. 添加encryptor字段
2. 在Record方法中加密Body字段
3. 在Close方法中清理资源

**改动代码**：
```go
type JSONLRecorder struct {
	mu           sync.Mutex
	path         string
	file         *os.File
	currentBytes int64
	maxFileBytes int64
	logger       *slog.Logger
	now          func() time.Time

	// 新增
	encryptor    Encryptor  // 加密器
}

func NewJSONLRecorderWithOptions(path string, options JSONLRecorderOptions, encryptor Encryptor) (*JSONLRecorder, error) {
	// ... 现有代码 ...

	return &JSONLRecorder{
		path:         path,
		file:         file,
		currentBytes: info.Size(),
		maxFileBytes: options.MaxFileBytes,
		logger:       slog.Default(),
		now:          time.Now,
		encryptor:    encryptor,  // 新增
	}, nil
}

func (r *JSONLRecorder) Record(ctx context.Context, event Event) {
	if r == nil || r.file == nil {
		return
	}

	// 新增：加密Body字段
	if r.encryptor != nil && len(event.Body) > 0 {
		encrypted, err := r.encryptor.Encrypt(event.Body)
		if err != nil {
			r.logger.Debug("failed to encrypt audit body", "error", err)
			// 继续记录，但不加密（降级处理）
		} else {
			event.Body = json.RawMessage(encrypted)
		}
	}

	// ... 现有代码 ...
}
```

---

### 4. 初始化逻辑

**文件**：`cmd/gateway/main.go`（或对应的初始化入口）

**改动点**：
```go
// 从环境变量读取加密密钥
var encryptor audit.Encryptor
if cfg.Audit.Encryption.Enabled {
	key := os.Getenv(cfg.Audit.Encryption.KeyEnv)
	if key == "" {
		return fmt.Errorf("audit encryption enabled but key not found in env %s", cfg.Audit.Encryption.KeyEnv)
	}

	enc, err := audit.NewAES256GCMEncryptor(key)
	if err != nil {
		return fmt.Errorf("failed to create encryptor: %w", err)
	}
	encryptor = enc
}

// 创建审计记录器
recorder, err := audit.NewJSONLRecorderWithOptions(
	cfg.Audit.Path,
	audit.JSONLRecorderOptions{MaxFileBytes: cfg.Audit.MaxFileBytes},
	encryptor,  // 传入加密器
)
```

---

### 5. 密钥生成工具

**新文件**：`cmd/tools/generate-audit-key.go`

**功能**：生成32字节的随机密钥

```go
package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
)

func main() {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		fmt.Fprintf(os.Stderr, "failed to generate key: %v\n", err)
		os.Exit(1)
	}

	encoded := base64.StdEncoding.EncodeToString(key)
	fmt.Println("# Audit encryption key (32 bytes, base64 encoded)")
	fmt.Printf("export AUDIT_ENCRYPTION_KEY=%q\n", encoded)
}
```

**使用**：
```bash
go run cmd/tools/generate-audit-key.go
# 输出：export AUDIT_ENCRYPMENT_KEY="..."
```

---

## 测试计划

### 单元测试

**文件**：`internal/audit/encryption_test.go`

**测试用例**：
1. ✅ 加密/解密正确性
2. ✅ 密钥长度错误处理
3. ✅ 解密错误密钥处理
4. ✅ 解密错误格式处理
5. ✅ 加密性能测试（< 10ms）
6. ✅ 并发加密安全性

### 集成测试

**文件**：`internal/audit/audit_test.go`

**测试用例**：
1. ✅ 启用加密后，审计日志中Body字段为加密字符串
2. ✅ 未启用加密时，审计日志保持原样
3. ✅ 加密密钥缺失时，启动失败
4. ✅ 加密失败时，降级记录（不加密）

---

## 性能估算

**AES-256-GCM性能**：
- 加密速度：~1GB/s（现代CPU）
- 1KB数据加密延迟：~1μs
- 预期审计日志Body大小：1-10KB
- **预期延迟**：< 10μs（远低于10ms要求）

---

## 向后兼容性

### 兼容策略

**不破坏现有行为**：
- ✅ 加密默认禁用（enabled: false）
- ✅ 现有配置文件无需修改
- ✅ 现有审计日志仍可读取（未加密的）

**升级路径**：
1. 现有部署：继续使用未加密审计日志
2. 新部署：启用加密（配置encryption.enabled=true）
3. 密钥管理：使用环境变量或密钥管理服务

---

## 安全考虑

### 密钥安全

**要求**：
- ✅ 密钥不写入配置文件（只用环境变量）
- ✅ 密钥不记录到日志
- ✅ 密钥定期轮换（建议每90天）

**密钥轮换**：
- 支持多个密钥（primary/secondary）
- 逐步迁移到新密钥
- 保留旧密钥解密历史数据

### 加密安全

**AES-256-GCM特性**：
- ✅ 认证加密（防篡改）
- ✅ 每次加密使用随机nonce
- ✅ 密文包含nonce（无需额外存储）

---

## 实施步骤

### 第1步：创建加密模块（1天）

- [ ] 创建 `internal/audit/encryption.go`
- [ ] 实现AES-256-GCM加密/解密
- [ ] 编写单元测试
- [ ] 性能测试（确保 < 10ms）

### 第2步：修改配置结构（半天）

- [ ] 修改 `internal/config/config.go`
- [ ] 添加AuditEncryptionConfig
- [ ] 编写配置测试

### 第3步：修改审计记录器（1天）

- [ ] 修改 `internal/audit/audit.go`
- [ ] 添加encryptor字段
- [ ] 在Record方法中加密Body
- [ ] 编写集成测试

### 第4步：初始化逻辑（半天）

- [ ] 修改初始化代码
- [ ] 从环境变量读取密钥
- [ ] 创建加密器并注入

### 第5步：工具与文档（半天）

- [ ] 创建密钥生成工具
- [ ] 编写配置文档
- [ ] 更新README示例

---

## 里程碑

- **第1周**：完成加密模块和单元测试
- **第2周**：完成集成、初始化和端到端测试
- **第3周**：完成文档、性能优化和验收

---

## 验收标准

### 功能验收

- ✅ 启用加密后，审计日志中Body字段为base64加密字符串
- ✅ 可使用正确密钥解密审计日志
- ✅ 错误密钥无法解密
- ✅ 未启用加密时，审计日志保持原样

### 性能验收

- ✅ 加密延迟 < 10ms（P99）
- ✅ 解密延迟 < 10ms（P99）
- ✅ 并发1000次加密无错误

### 安全验收

- ✅ 密钥不写入配置文件
- ✅ 密钥不记录到日志
- ✅ 加密后的数据不可直接读取

---

## 后续扩展（非MVP）

- 密钥轮换机制
- 多密钥支持
- 密钥管理服务集成（Vault、AWS KMS）
- 审计日志加密归档
- 审计日志解密查询工具