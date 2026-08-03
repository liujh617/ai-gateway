# Audit Encryption Design

## Goal

Add optional AES-256-GCM encryption for audit log request and response bodies to protect sensitive data while maintaining audit completeness and supporting compliance requirements.

## Non-Goals

- This is not a full audit log security solution.
- This does not encrypt the entire audit file, only sensitive body fields.
- This does not provide key management service integration (AWS KMS, GCP KMS, etc.).
- This first version does not support key rotation.
- This does not encrypt metadata fields like timestamp, request_id, event, provider, etc.

## Configuration

Add an `encryption` subsection to the `audit` configuration:

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

Defaults:

- `audit.encryption.enabled`: `false`
- `audit.encryption.algorithm`: `"aes-256-gcm"` (only supported algorithm)
- `audit.encryption.key_env`: `"AUDIT_ENCRYPTION_KEY"`

Environment variables:

- `GATEWAY_AUDIT_ENCRYPTION_ENABLED`: Override `audit.encryption.enabled`
- `GATEWAY_AUDIT_ENCRYPTION_KEY`: Base64-encoded 32-byte encryption key

Validation:

- If `encryption.enabled` is `true`, `key_env` must be non-empty.
- The key from environment variable must be exactly 32 bytes when base64-decoded.
- If validation fails, gateway startup must fail with a clear error message.

## Encryption Scope

Encrypted fields:

- `request_body` (the entire request JSON body)
- `response_body` (the entire response JSON body)

Not encrypted:

- `timestamp`
- `event`
- `request_id`
- `trace_id`
- `path`
- `client`
- `provider`
- `model`
- `status`
- `duration_ms`

Rationale:

- Metadata fields are non-sensitive and useful for queries/filtering.
- Body fields contain user prompts, completions, tool schemas, and potentially sensitive data.
- Selective encryption balances security with operational visibility.

## Event Format Changes

When encryption is enabled, the audit event includes an additional boolean field:

```json
{
  "timestamp": "2026-08-03T01:23:45.000000000Z",
  "event": "response",
  "request_id": "req_...",
  "trace_id": "agent-session-...",
  "body_encrypted": true,
  "request_body": "aes-256-gcm:ciphertext-base64",
  "response_body": "aes-256-gcm:ciphertext-base64",
  ...
}
```

Field rules:

- `body_encrypted`: `true` if bodies are encrypted, `false` or omitted otherwise.
- `request_body` / `response_body`: base64-encoded ciphertext prefixed with `aes-256-gcm:` when encrypted.

## Encryption Algorithm

Use AES-256-GCM (Galois/Counter Mode):

- Key length: 32 bytes (256 bits)
- Nonce: 12 bytes, randomly generated per encryption
- Authentication tag: 16 bytes, appended to ciphertext
- Ciphertext format: `nonce(12) + ciphertext + tag(16)`

Why AES-256-GCM:

- Strong confidentiality and integrity guarantees.
- Authenticated encryption prevents tampering.
- Widely supported in Go standard library (`crypto/aes`, `crypto/cipher`).
- Recommended by NIST for protecting sensitive data.

## Storage Changes

Add encryption support to `internal/audit`:

- Add `Encryptor` interface:
  ```go
  type Encryptor interface {
      Encrypt(plaintext []byte) (ciphertext []byte, err error)
      Decrypt(ciphertext []byte) (plaintext []byte, err error)
  }
  ```

- Add `AES256GCMEncryptor` implementation:
  - Constructor: `NewAES256GCMEncryptor(key []byte) (*AES256GCMEncryptor, error)`
  - Key validation: must be exactly 32 bytes.
  - Random nonce generation per encryption.

- Add `NoopEncryptor` (no-op implementation for disabled encryption).

- Modify `JSONLRecorder`:
  - Add `encryptor Encryptor` field.
  - In `Record()` method, encrypt `request_body` and `response_body` before marshaling.
  - Add `body_encrypted` field to event.

- Modify `NewJSONLRecorderWithOptions()`:
  - Add optional `encryptor` parameter.
  - Default to `NoopEncryptor` if not provided.

## Integration

Config layer loads:

- `audit.encryption.enabled`
- `audit.encryption.algorithm`
- `audit.encryption.key_env`

Command layer:

- Load encryption key from environment variable (base64-decoded).
- Validate key length.
- Build `Encryptor` when enabled.
- Pass it to `NewJSONLRecorderWithOptions()`.
- Include encryption status in startup logs.
- Fail startup if key is missing or invalid when encryption is enabled.

Key loading:

- Read `key_env` value (e.g., `AUDIT_ENCRYPTION_KEY`).
- Read the environment variable with that name.
- Base64-decode to obtain the 32-byte key.
- Fail with clear error if:
  - Environment variable is empty or missing.
  - Base64 decode fails.
  - Result is not exactly 32 bytes.

## Security Notes

### Key Management

- **Never hard-code keys in configuration files.**
- **Never log the key value.**
- Use environment variables or secret management systems.
- Generate keys using cryptographically secure random generators.
- Rotate keys periodically (future work).

### Encryption Failures

- Encryption failures should be logged but not fail the request.
- Fail-open behavior ensures audit completeness.
- Log encryption errors with request metadata for debugging.

### Performance

- AES-256-GCM is fast (< 1ms for typical request bodies).
- Encryption overhead is negligible compared to model API latency.
- No significant throughput impact.

### Compliance Benefits

- Helps meet data protection requirements (GDPR, data security laws).
- Protects sensitive prompts and completions at rest.
- Supports private deployment scenarios where audit logs might be accessed by unauthorized parties.

## Testing

Unit and integration tests should cover:

- Config defaults keep encryption disabled.
- Config can enable encryption and set key_env.
- Environment variable overrides work correctly.
- Invalid key length fails validation.
- Missing key_env when encryption enabled fails startup.
- AES-256-GCM encryptor encrypts and decrypts correctly.
- Encryption produces different ciphertexts for same plaintext (random nonce).
- Invalid ciphertext decryption fails gracefully.
- JSONLRecorder with encryption writes `body_encrypted: true`.
- JSONLRecorder without encryption omits `body_encrypted` or sets to `false`.
- Large body encryption works correctly (> 10KB).
- Encryption failure does not fail the request.

Test utilities:

- Add `TestAES256GCMEncryptor` with comprehensive test cases.
- Add integration test for encrypted audit recording.
- Add performance test for 1000 iterations.

Verification commands:

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/ai-gateway -- bash -lc "go test ./internal/audit -v -run TestAES256GCMEncryptor"
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/ai-gateway -- bash -lc "go test ./internal/config -v -run TestAuditEncryption"
```

## Key Generation Tool

Provide a command-line tool for generating encryption keys:

```bash
go run ./cmd/tools/audit-keygen
```

Output:

```
Generated AES-256 encryption key (base64-encoded):
SWsf3k9fj39fj3jf9fj3f9j3f9j3f9j3f9j3f9j3f9k=

Configuration example:
export AUDIT_ENCRYPTION_KEY="SWsf3k9fj39fj3jf9fj3f9j3f9j3f9j3f9j3f9k="
```

Tool implementation:

- Use `crypto/rand` to generate 32 random bytes.
- Base64-encode the result.
- Print configuration examples for shell, PowerShell, Docker.

## Implementation Phases

### Phase 1: Core Encryption (2 weeks)

- Implement `Encryptor` interface.
- Implement `AES256GCMEncryptor`.
- Implement `NoopEncryptor`.
- Add comprehensive unit tests.
- Add performance benchmarks.

### Phase 2: Configuration (1 week)

- Add `AuditEncryptionConfig` to config.
- Add environment variable support.
- Add validation logic.
- Add config tests.

### Phase 3: Audit Integration (1 week)

- Modify `JSONLRecorder` to support encryption.
- Add `body_encrypted` field.
- Handle encryption errors gracefully.
- Add integration tests.

### Phase 4: Initialization (1 week)

- Add key loading logic to command layer.
- Integrate with `buildAuditRecorder()`.
- Add startup log messages.
- Add end-to-end tests.

### Phase 5: Tooling and Documentation (1 week)

- Implement `audit-keygen` tool.
- Add usage documentation.
- Add deployment examples (Docker, Kubernetes, Systemd).
- Add troubleshooting guide.

Total estimated time: 6 weeks.

## Open Questions Deferred

- Key rotation mechanism.
- Integration with cloud KMS (AWS KMS, GCP KMS, Azure Key Vault).
- Audit log decryption tool for offline analysis.
- Encryption at rest for archived audit files.
- Multi-tenant key management.