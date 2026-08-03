# PII Detection Design

## Goal

Add PII (Personally Identifiable Information) detection capability to audit logs for identifying and alerting on sensitive data exposure, supporting compliance requirements in regulated industries (finance, healthcare, government).

## Non-Goals

- This is not a comprehensive data loss prevention (DLP) solution.
- This does not support all possible PII types (only Chinese phone, ID card, bank card initially).
- This does not use ML-based PII detection (regex only for simplicity and performance).
- This does not provide automatic remediation (alerting only).
- This first version does not support custom PII patterns.

## Configuration

Add a `pii_detection` section to the top-level config:

```json
{
  "pii_detection": {
    "enabled": true,
    "action": "alert",
    "patterns": ["phone", "id_card", "bank_card"],
    "log_matches": true
  }
}
```

Defaults:

- `pii_detection.enabled`: `false`
- `pii_detection.action`: `"alert"` (options: `alert`, `reject`, `allow`)
- `pii_detection.patterns`: `["phone", "id_card", "bank_card"]`
- `pii_detection.log_matches`: `true`

Environment overrides:

- `GATEWAY_PII_DETECTION_ENABLED`: Override `pii_detection.enabled`
- `GATEWAY_PII_DETECTION_ACTION`: Override `pii_detection.action`
- `GATEWAY_PII_DETECTION_PATTERNS`: Comma-separated list of patterns

Validation:

- `action` must be one of: `alert`, `reject`, `allow`.
- `patterns` must be a non-empty array with valid pattern names.

## Supported PII Patterns

### Chinese Phone Number (phone)

Pattern: `1[3-9]\d{9}`

Matches:

- 11-digit Chinese mobile phone numbers
- Examples: `13912345678`, `18800001111`

Accuracy:

- High precision (> 95%) for Chinese phone numbers.
- May have false positives for other 11-digit numbers starting with 1.

### Chinese ID Card Number (id_card)

Pattern: `[1-9]\d{5}(18|19|20)\d{2}(0[1-9]|1[0-2])(0[1-9]|[12]\d|3[01])\d{3}[\dXx]`

Matches:

- 18-digit Chinese resident ID card numbers
- Format: region (6) + birth date (8) + sequence (3) + checksum (1)
- Examples: `110101199003073218`, `44030519850101234X`

Accuracy:

- Very high precision (> 98%) for valid ID card numbers.
- Validates region code, date, and checksum format.

### Chinese Bank Card Number (bank_card)

Pattern: `[1-9]\d{15,18}`

Matches:

- 16-19 digit bank card numbers starting with non-zero
- Examples: `6225881234567890`, `6217001234567891234`

Accuracy:

- Moderate precision (> 90%).
- May have false positives for other long numeric sequences.

## Detection Scope

Detect PII in:

- `request_body` (client request JSON)
  - `messages[].content` fields
  - User prompts
  - Tool call arguments

- `response_body` (provider response JSON)
  - `choices[].message.content` fields
  - AI-generated completions
  - Tool call results

Not detected:

- Metadata fields (timestamp, request_id, provider, etc.)
- HTTP headers
- URL query parameters

## Behavior

### Action: alert (default)

When PII is detected:

- Log a warning with detected PII types and locations.
- Add `pii_detected` field to audit event with details.
- Continue processing the request normally.
- Do not modify request or response.

Example audit event:

```json
{
  "timestamp": "2026-08-04T01:23:45.000000000Z",
  "event": "response",
  "request_id": "req_...",
  "pii_detected": {
    "has_pii": true,
    "total_count": 3,
    "by_type": {
      "phone": 1,
      "id_card": 1,
      "bank_card": 1
    },
    "locations": [
      {"field": "request_body.messages[0].content", "type": "phone", "count": 1},
      {"field": "response_body.choices[0].message.content", "type": "id_card", "count": 1},
      {"field": "response_body.choices[0].message.content", "type": "bank_card", "count": 1}
    ]
  }
}
```

### Action: reject

When PII is detected:

- Reject the request before sending to provider.
- Return `400 invalid_request_error` with message: "Request contains PII data".
- Log the detection details.
- Do not send request to upstream provider.

Rationale:

- Prevents PII data from being sent to AI providers.
- Strongest protection level for compliance scenarios.
- May cause user friction if PII detection has false positives.

### Action: allow

When PII is detected:

- Log the detection (informational level).
- Do not add `pii_detected` field to audit event.
- Continue processing normally.

Use case:

- Audit-only mode for monitoring PII flows.
- Low-friction deployment for testing.

## Storage Changes

Add PII detection to `internal/audit`:

- Add `Detector` interface:
  ```go
  type Detector interface {
      Detect(ctx context.Context, text string) PIIDetectionResult
  }
  ```

- Add `PIIMatch` struct:
  ```go
  type PIIMatch struct {
      Type     string // "phone", "id_card", "bank_card"
      Value    string // matched value (can be redacted)
      Start    int    // start position in text
      End      int    // end position in text
  }
  ```

- Add `PIIDetectionResult` struct:
  ```go
  type PIIDetectionResult struct {
      HasPII  bool
      Matches []PIIMatch
      ByType  map[string]int // count by type
  }
  ```

- Implement `RegexDetector`:
  - Compile regex patterns for each PII type.
  - Scan text and return all matches.
  - Support configurable pattern list.

- Add `PIIAuditorRecorder` wrapper:
  - Wraps `JSONLRecorder`.
  - Detects PII in request/response bodies.
  - Adds `pii_detected` field to audit event.
  - Logs PII detections.
  - Implements action logic (alert/reject/allow).

## Integration

Config layer loads:

- `pii_detection.enabled`
- `pii_detection.action`
- `pii_detection.patterns`
- `pii_detection.log_matches`

Command layer:

- Build PII detector when enabled.
- Wrap audit recorder with PII detection.
- Pass to API server.
- Include PII detection status in startup logs.

API layer:

- If `action` is `reject`, check PII before forwarding request.
- If PII detected, return `400 invalid_request_error`.
- Otherwise, continue processing.

Audit layer:

- After recording request/response, detect PII in bodies.
- Add `pii_detected` field to audit event.
- Log PII detections based on `log_matches` setting.

## Performance Considerations

- Regex-based detection is fast (< 5ms for typical request bodies).
- Detection runs asynchronously after request completion (except for `action: reject`).
- For `action: reject`, detection runs synchronously before upstream call.
- Performance impact is negligible compared to model API latency.
- No significant throughput impact.

## Testing

Unit and integration tests should cover:

- Config defaults keep PII detection disabled.
- Config can enable PII detection and set action.
- Environment variable overrides work correctly.
- Invalid action fails validation.
- Empty patterns list fails validation.
- Regex detector detects Chinese phone numbers correctly.
- Regex detector detects Chinese ID cards correctly.
- Regex detector detects Chinese bank cards correctly.
- PII detection handles large texts (> 10KB).
- PII detection handles empty/nil bodies.
- PIIAuditorRecorder adds `pii_detected` field.
- `action: alert` logs warnings.
- `action: reject` blocks requests with PII.
- `action: allow` logs informationally.
- Performance: detection < 5ms P99.

Test utilities:

- Add `TestRegexDetector` with comprehensive test cases.
- Add `TestPIIAuditorRecorder` with all actions.
- Add performance tests for large texts.

Verification commands:

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/ai-gateway -- bash -lc "go test ./internal/audit -v -run TestRegexDetector"
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/ai-gateway -- bash -lc "go test ./internal/config -v -run TestPIIDetection"
```

## Security and Compliance Notes

### Data Protection

- PII detection helps identify sensitive data exposure.
- Supports compliance with data protection laws (GDPR, China Data Security Law).
- Helps organizations track and audit PII flows.

### Privacy Considerations

- Matched PII values are logged by default (can be redacted).
- Logs may contain sensitive data (configure `log_matches` carefully).
- Consider encrypting PII detection logs separately.

### False Positives and Negatives

- Regex-based detection has higher false positives than ML-based.
- Regular expressions may miss obfuscated PII.
- Test detection rules against your specific data patterns.

### Deployment Recommendation

- Start with `action: alert` to understand PII flows.
- Tune patterns and thresholds based on alerts.
- Move to `action: reject` when confident in detection accuracy.

## Implementation Phases

### Phase 1: Core Detection (2 weeks)

- Define `Detector` interface.
- Implement `RegexDetector` with regex patterns.
- Implement `PIIMatch` and `PIIDetectionResult`.
- Add comprehensive unit tests.
- Add performance benchmarks.

### Phase 2: Configuration (1 week)

- Add `PIIDetectionConfig` to config.
- Add environment variable support.
- Add validation logic.
- Add config tests.

### Phase 3: Audit Integration (1 week)

- Implement `PIIAuditorRecorder` wrapper.
- Add `pii_detected` field to audit event.
- Implement action logic.
- Add integration tests.

### Phase 4: Gateway Integration (1 week)

- Integrate with command layer.
- Implement `action: reject` in API layer.
- Add startup logs.
- Add end-to-end tests.

### Phase 5: Documentation (1 week)

- Add usage documentation.
- Add deployment examples.
- Add troubleshooting guide.
- Add compliance guidance.

Total estimated time: 6 weeks.

## Open Questions Deferred

- ML-based PII detection for higher accuracy.
- Additional PII types (email addresses, credit cards, IP addresses).
- Custom PII patterns.
- PII redaction/masking.
- Integration with external DLP systems.
- PII detection in images/PDFs (multimodal).