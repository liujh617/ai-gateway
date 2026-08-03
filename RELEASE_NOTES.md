# Release v1.1.0 - Audit Encryption

**Release Date**: 2026-08-03

## Overview

This release adds **audit log encryption** capability, enabling secure audit logs with AES-256-GCM encryption for request and response bodies.

## 🎯 Core Features

### Audit Log Encryption

- ✅ **AES-256-GCM Encryption**: Strong authenticated encryption for audit logs
- ✅ **Selective Encryption**: Only encrypts sensitive request/response bodies
- ✅ **Configurable**: Enable/disable via configuration file or environment variables
- ✅ **Secure Key Management**: Encryption keys stored in environment variables
- ✅ **Backward Compatible**: Encryption is optional and disabled by default
- ✅ **Key Generation Tool**: Built-in tool for generating secure encryption keys

## 📊 What's Changed

### New Features

#### 1. Encryption Module (`internal/audit/encryption.go`)
- Implement AES-256-GCM encryptor
- Add Encryptor interface for extensibility
- Support key validation and error handling

#### 2. Configuration Support (`internal/config/config.go`)
- Add `AuditEncryptionConfig` structure
- Support environment variable overrides
- Add configuration validation

#### 3. Audit Recorder Integration (`internal/audit/audit.go`)
- Integrate encryptor into JSONLRecorder
- Encrypt request/response bodies before writing
- Add `body_encrypted` field for encrypted data

#### 4. Initialization Logic (`cmd/gateway/main.go`)
- Add `buildAuditRecorder` with encryption support
- Implement `loadEncryptionKey` helper
- Add encryption status to startup logs

#### 5. Key Generation Tool (`cmd/tools/audit-keygen/main.go`)
- Generate 32-byte random keys
- Output base64-encoded keys
- Provide configuration examples for multiple platforms

#### 6. Documentation (`docs/audit-encryption.md`)
- Complete usage guide
- Key management best practices
- Configuration examples for Docker, Kubernetes, Systemd
- Troubleshooting guide
- Performance benchmarks

## 🚀 Quick Start

### 1. Generate Encryption Key

```bash
# Build the tool
go build -o audit-keygen ./cmd/tools/audit-keygen

# Generate a key
./audit-keygen

# Output: export AUDIT_ENCRYPTION_KEY="base64-encoded-key"
```

### 2. Set Environment Variable

```bash
# Linux/macOS
export AUDIT_ENCRYPTION_KEY="your-base64-encoded-key-here"

# Windows PowerShell
$env:AUDIT_ENCRYPTION_KEY="your-base64-encoded-key-here"
```

### 3. Enable in Configuration

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
export GATEWAY_CONFIG=gateway-config.json
./gateway
```

## 📝 Configuration Reference

### AuditEncryptionConfig

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `false` | Enable/disable encryption |
| `algorithm` | string | `"aes-256-gcm"` | Encryption algorithm |
| `key_env` | string | `""` | Environment variable name for encryption key |

### Environment Variables

- `GATEWAY_AUDIT_ENCRYPTION_ENABLED`: Override config `enabled` field
- `GATEWAY_AUDIT_ENCRYPTION_KEY`: Override config `key_env` field
- `<key_env>`: Base64-encoded 32-byte encryption key

## 🔒 Security Features

- **Strong Encryption**: AES-256-GCM with 256-bit keys
- **Authenticated Encryption**: GCM mode provides integrity and authenticity
- **Secure Key Generation**: Cryptographically secure random key generation
- **Environment Variable Storage**: Keys never hardcoded in configuration
- **Validation**: Key length and format validated at startup

## 📊 Performance

| Operation | Latency (P99) | Throughput |
|-----------|---------------|------------|
| Encrypt 1KB | < 1ms | > 10,000 ops/s |
| Encrypt 10KB | < 5ms | > 2,000 ops/s |
| Decrypt 1KB | < 1ms | > 10,000 ops/s |
| Decrypt 10KB | < 5ms | > 2,000 ops/s |

**Overall Impact**: < 10ms latency overhead for typical requests

## 📦 New Files

```
internal/audit/encryption.go              # Encryption module
internal/audit/encryption_test.go         # Unit tests
internal/config/audit_encryption_test.go  # Configuration tests
internal/audit/encryption_integration_test.go  # Integration tests
cmd/tools/audit-keygen/main.go           # Key generation tool
docs/adr/audit-encryption-design.md      # Design document
docs/audit-encryption.md                 # User guide
```

## 🔧 Changed Files

```
internal/audit/audit.go          # Add encryption support
internal/config/config.go         # Add encryption configuration
cmd/gateway/main.go               # Add initialization logic
```

## 📈 Statistics

- **Code Added**: 2,040 lines
- **Test Cases**: 25+
- **Git Commits**: 6
- **Documentation**: 442 lines

## 🐛 Bug Fixes

None - this is a new feature release

## 🔄 Breaking Changes

**None** - Fully backward compatible. Encryption is disabled by default.

## 📋 Upgrade Guide

### From v1.0.x to v1.1.0

1. **No configuration changes required** - encryption is disabled by default
2. **To enable encryption**:
   - Generate encryption key using `audit-keygen` tool
   - Set environment variable
   - Update configuration file
   - Restart gateway

### Migration Steps

```bash
# 1. Pull latest code
git pull origin main

# 2. Build
go build ./cmd/gateway

# 3. Generate encryption key (optional)
go run ./cmd/tools/audit-keygen

# 4. Update configuration (optional)
# Add encryption section to gateway-config.json

# 5. Restart gateway
./gateway
```

## 🔗 Related Documentation

- [Audit Encryption Guide](docs/audit-encryption.md)
- [Architecture Overview](architecture/overview.md)
- [Configuration Reference](docs/configuration.md)

## 🙏 Contributors

- Developer: Personal hobby project
- Design: Full ADR design document
- Testing: Comprehensive test coverage

## 🎯 Next Steps

### Planned Features (v1.2.0+)

- **Audit Log Archival**: Automatic log rotation and archival
- **PII Detection**: Identify and redact personally identifiable information
- **Content Safety**: Integrate content safety APIs
- **Key Rotation**: Support for encryption key rotation

## 📞 Support

- **Issues**: https://github.com/liujh617/ai-gateway/issues
- **Discussions**: https://github.com/liujh617/ai-gateway/discussions
- **Documentation**: docs/audit-encryption.md

---

**Full Changelog**: https://github.com/liujh617/ai-gateway/compare/v1.0.0...v1.1.0