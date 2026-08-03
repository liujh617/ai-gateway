# Audit Encryption Guide

This guide explains how to enable and configure audit log encryption in AI Gateway.

## Overview

Audit encryption protects sensitive data in audit logs by encrypting request and response bodies using AES-256-GCM encryption.

### Key Features

- ✅ **Strong Encryption**: AES-256-GCM (authenticated encryption)
- ✅ **Selective Encryption**: Only encrypts request/response bodies
- ✅ **Configurable**: Enable/disable via configuration
- ✅ **Secure Key Management**: Key stored in environment variable
- ✅ **Backward Compatible**: Encryption is optional and disabled by default

---

## Quick Start

### 1. Generate Encryption Key

Use the provided key generation tool:

```bash
# Build the tool
go build -o audit-keygen ./cmd/tools/audit-keygen

# Generate a key
./audit-keygen
```

Output example:

```
export AUDIT_ENCRYPTION_KEY="your-base64-encoded-key-here"
```

### 2. Set Environment Variable

```bash
# Linux/macOS (bash/zsh)
export AUDIT_ENCRYPTION_KEY="your-base64-encoded-key-here"

# Windows PowerShell
$env:AUDIT_ENCRYPTION_KEY="your-base64-encoded-key-here"
```

### 3. Enable in Configuration

Create or edit `gateway-config.json`:

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

### 4. Start Gateway

```bash
# Set config path
export GATEWAY_CONFIG=gateway-config.json

# Start gateway
./gateway
```

Verify encryption is enabled in logs:

```
audit_encryption_enabled=true audit_encryption_algorithm=aes-256-gcm
```

---

## Configuration Reference

### AuditEncryptionConfig

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `false` | Enable/disable encryption |
| `algorithm` | string | `"aes-256-gcm"` | Encryption algorithm (only AES-256-GCM supported) |
| `key_env` | string | `""` | Environment variable name containing base64-encoded key |

### Environment Variables

| Variable | Description | Example |
|----------|-------------|---------|
| `GATEWAY_AUDIT_ENCRYPTION_ENABLED` | Override config `enabled` field | `"true"` or `"false"` |
| `GATEWAY_AUDIT_ENCRYPTION_KEY` | Override config `key_env` field | `"AUDIT_ENCRYPTION_KEY"` |
| `<key_env>` | Base64-encoded 32-byte key | Base64 string |

---

## Key Management

### Key Requirements

- **Length**: Exactly 32 bytes (256 bits)
- **Format**: Base64-encoded string
- **Source**: Cryptographically secure random generator

### Key Generation Options

#### Option 1: Use audit-keygen tool (Recommended)

```bash
./audit-keygen
```

#### Option 2: Use OpenSSL

```bash
# Generate and encode
openssl rand -base64 32
```

#### Option 3: Use Python

```python
import os
import base64

key = os.urandom(32)
print(base64.b64encode(key).decode())
```

### Key Storage

**Best Practices**:

- ✅ Store in environment variables
- ✅ Use secret management systems (AWS Secrets Manager, Azure Key Vault, HashiCorp Vault)
- ✅ Rotate keys periodically
- ✅ Limit access to authorized personnel

**Avoid**:

- ❌ Hardcoding keys in configuration files
- ❌ Committing keys to version control
- ❌ Sharing keys via email or chat
- ❌ Storing keys in plain text files

---

## Encrypted Data Format

### JSONL Record Structure

When encryption is enabled, the `body` field is replaced with encrypted data:

```json
{
  "timestamp": "2026-08-03T21:30:00Z",
  "event": "chat_completion",
  "request_id": "req-123",
  "body_encrypted": "base64-encoded-encrypted-data",
  "body": null,
  ...
}
```

### Field Changes

| Field | Unencrypted | Encrypted |
|-------|-------------|-----------|
| `body` | JSON object | `null` |
| `body_encrypted` | `null` | Base64 string |

---

## Decryption

### Decrypt Audit Logs

Use the audit-inspect tool with decryption support:

```bash
# Set key environment variable
export AUDIT_ENCRYPTION_KEY="your-base64-encoded-key-here"

# Inspect with decryption
./gateway audit-inspect audit/agent-trace.jsonl
```

**Note**: Decryption support in audit-inspect is planned for future release.

### Programmatic Decryption

```go
package main

import (
    "encoding/base64"
    "fmt"
    "open-ai-gateway/internal/audit"
)

func main() {
    // Load key
    keyB64 := "your-base64-encoded-key"
    key, _ := base64.StdEncoding.DecodeString(keyB64)
    
    // Create encryptor
    encryptor, _ := audit.NewAES256GCMEncryptor(key)
    
    // Decrypt
    encryptedData := "encrypted-base64-data"
    decrypted, err := encryptor.Decrypt(encryptedData)
    if err != nil {
        panic(err)
    }
    
    fmt.Println(string(decrypted))
}
```

---

## Performance Impact

### Benchmarks

| Operation | Latency (P99) | Throughput |
|-----------|---------------|------------|
| Encrypt 1KB | < 1ms | > 10,000 ops/s |
| Encrypt 10KB | < 5ms | > 2,000 ops/s |
| Decrypt 1KB | < 1ms | > 10,000 ops/s |
| Decrypt 10KB | < 5ms | > 2,000 ops/s |

### Overhead

- **CPU**: Negligible (< 1% per core)
- **Memory**: Minimal (< 100KB per request)
- **Latency**: < 10ms P99 for typical requests

---

## Security Considerations

### Encryption Scope

**What is encrypted**:

- ✅ Request bodies (prompts, parameters)
- ✅ Response bodies (completions, embeddings)
- ✅ Streaming chunks (each SSE event)

**What is NOT encrypted**:

- ❌ Metadata (request_id, client, model, provider)
- ❌ Timestamps
- ❌ Status codes
- ❌ Error messages

### Key Rotation

To rotate encryption keys:

1. **Generate new key**:

   ```bash
   ./audit-keygen -env AUDIT_ENCRYPTION_KEY_NEW
   ```

2. **Update configuration** to support both keys (future feature)

3. **Re-encrypt old logs** with new key (manual process)

4. **Update environment variable**:

   ```bash
   export AUDIT_ENCRYPTION_KEY="new-key-here"
   ```

5. **Restart gateway**

### Compliance

Audit encryption helps meet:

- ✅ **GDPR**: Data protection requirements
- ✅ **SOC 2**: Data encryption at rest
- ✅ **PCI DSS**: Cardholder data protection
- ✅ **HIPAA**: PHI encryption requirements

**Note**: Encryption alone may not be sufficient for full compliance. Consult compliance requirements.

---

## Troubleshooting

### Common Issues

#### Error: "encryption key environment variable is not set"

**Cause**: Environment variable not set

**Solution**:

```bash
export AUDIT_ENCRYPTION_KEY="your-key-here"
```

#### Error: "key must be 32 bytes, got X"

**Cause**: Incorrect key length

**Solution**: Regenerate key using `audit-keygen` tool

#### Error: "decode base64 key: illegal base64 data"

**Cause**: Key not properly base64-encoded

**Solution**: Use `audit-keygen` to generate proper base64 key

#### Audit logs show unencrypted data

**Cause**: Encryption not enabled in configuration

**Solution**: Check configuration:

```json
{
  "audit": {
    "encryption": {
      "enabled": true
    }
  }
}
```

### Verification

Check if encryption is working:

```bash
# View audit logs
cat audit/agent-trace.jsonl | grep "body_encrypted"
```

Should see:

```json
{"body_encrypted":"...", "body":null}
```

---

## Examples

### Docker Compose

```yaml
version: '3.8'

services:
  gateway:
    image: ai-gateway:latest
    environment:
      - GATEWAY_CONFIG=/config/gateway-config.json
      - AUDIT_ENCRYPTION_KEY=${AUDIT_ENCRYPTION_KEY}
    volumes:
      - ./config:/config
      - ./audit:/audit
```

### Kubernetes

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: audit-encryption-key
type: Opaque
data:
  key: <base64-encoded-key>
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ai-gateway
spec:
  template:
    spec:
      containers:
      - name: gateway
        image: ai-gateway:latest
        env:
        - name: AUDIT_ENCRYPTION_KEY
          valueFrom:
            secretKeyRef:
              name: audit-encryption-key
              key: key
```

### Systemd

```ini
[Unit]
Description=AI Gateway

[Service]
Type=simple
User=gateway
Environment="GATEWAY_CONFIG=/etc/gateway/config.json"
Environment="AUDIT_ENCRYPTION_KEY=your-key-here"
ExecStart=/usr/local/bin/gateway
Restart=always

[Install]
WantedBy=multi-user.target
```

---

## See Also

- [Configuration Reference](./configuration.md)
- [Audit Log Format](./audit-format.md)
- [Security Best Practices](./security.md)

---

## Changelog

### v1.0.0 (2026-08-03)

- Initial implementation
- AES-256-GCM encryption
- Environment variable key management
- audit-keygen tool