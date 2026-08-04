# Content Safety Detection Design

## Goal

Add content safety detection capability to audit logs for identifying and alerting on potentially harmful content, ensuring AI interactions comply with content safety policies without sending data to external APIs. This supports organizations that require **data sovereignty** and cannot use cloud-based content safety services.

## Non-Goals

- This is not a comprehensive content moderation solution with ML models.
- This does not support image/video/audio content safety (text only).
- This does not use cloud-based content safety APIs (data must stay local).
- This does not provide automatic content remediation (alerting only).
- This first version does not support semantic understanding (keyword-based only).

## Configuration

Add a `content_safety` section to the top-level config:

```json
{
  "content_safety": {
    "enabled": true,
    "action": "alert",
    "categories": ["politics", "pornography", "violence", "advertising"],
    "custom_keywords": {
      "enabled": true,
      "paths": ["/etc/ai-gateway/keywords/politics.txt", "/etc/ai-gateway/keywords/custom.txt"]
    },
    "log_matches": true,
    "threshold": "medium"
  }
}
```

Defaults:

- `content_safety.enabled`: `false`
- `content_safety.action`: `"alert"` (options: `alert`, `reject`, `allow`)
- `content_safety.categories`: `["politics", "pornography", "violence", "advertising"]`
- `content_safety.custom_keywords.enabled`: `false`
- `content_safety.custom_keywords.paths`: `[]`
- `content_safety.log_matches`: `true`
- `content_safety.threshold`: `"medium"` (options: `low`, `medium`, `high`)

Environment overrides:

- `GATEWAY_CONTENT_SAFETY_ENABLED`: Override `content_safety.enabled`
- `GATEWAY_CONTENT_SAFETY_ACTION`: Override `content_safety.action`
- `GATEWAY_CONTENT_SAFETY_CATEGORIES`: Comma-separated list of categories
- `GATEWAY_CONTENT_SAFETY_THRESHOLD`: Override `content_safety.threshold`

Validation:

- `action` must be one of: `alert`, `reject`, `allow`.
- `categories` must be a non-empty array with valid category names.
- `threshold` must be one of: `low`, `medium`, `high`.
- `custom_keywords.paths` must be valid file paths (if provided).

## Supported Content Safety Categories

### Politics (politics)

Detection scope:

- Politically sensitive keywords
- Government-related terms
- Political figure names
- Ideological terms

Keyword sources:

- Built-in sensitive keyword list (Chinese politics)
- User-customizable keyword files

Accuracy:

- High precision (> 90%) for obvious political keywords.
- May have false positives for neutral discussions of government services.

### Pornography (pornography)

Detection scope:

- Sexually explicit keywords
- Adult content terminology
- Pornography-related terms

Keyword sources:

- Built-in adult keyword list
- User-customizable keyword files

Accuracy:

- High precision (> 95%) for explicit keywords.
- May miss euphemisms and slang.

### Violence (violence)

Detection scope:

- Violent language
- Terrorist-related terms
- Self-harm keywords
- Weapon-related terms

Keyword sources:

- Built-in violence keyword list
- User-customizable keyword files

Accuracy:

- High precision (> 90%) for explicit violent keywords.
- May miss context-dependent violent language.

### Advertising (advertising)

Detection scope:

- Spam keywords
- Marketing terminology
- Promotional language
- Competitor names (if configured)

Keyword sources:

- Built-in advertising keyword list
- User-customizable keyword files (e.g., competitor names)

Accuracy:

- Moderate precision (> 80%).
- May have false positives for legitimate business discussions.

## Custom Keywords

### Keyword File Format

Keyword files are plain text files with one keyword per line:

```
# This is a comment
敏感词1
敏感词2
competitor_name
```

Features:

- Lines starting with `#` are comments.
- Blank lines are ignored.
- Keywords are case-insensitive.
- Support simple wildcards: `敏感*` matches `敏感词`, `敏感内容`, etc.

### Keyword Loading

- Keywords are loaded at startup.
- Keywords are reloaded on SIGHUP (if supported).
- Invalid keyword files cause startup failure with clear error message.
- Duplicate keywords are automatically deduplicated.

### Keyword Matching

- Case-insensitive matching.
- Support wildcard matching (`*` matches any characters).
- Threshold levels:
  - `low`: Match on 1 keyword
  - `medium`: Match on 2+ keywords or 1 wildcard match
  - `high`: Match on 3+ keywords or 2 wildcard matches

## Detection Scope

Detect content safety issues in:

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

When content safety violation is detected:

- Log a warning with detected categories and keywords.
- Add `content_safety_violation` field to audit event with details.
- Continue processing the request normally.
- Do not modify request or response.

Example audit event:

```json
{
  "timestamp": "2026-08-04T02:00:00.000000000Z",
  "event": "response",
  "request_id": "req_...",
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

### Action: reject

When content safety violation is detected:

- Reject the request before sending to provider.
- Return `400 invalid_request_error` with message: "Request contains unsafe content".
- Log the detection details.
- Do not send request to upstream provider.

Rationale:

- Prevents unsafe content from being processed by AI providers.
- Strongest protection level for compliance scenarios.
- May cause user friction if detection has false positives.

### Action: allow

When content safety violation is detected:

- Log the detection (informational level).
- Do not add `content_safety_violation` field to audit event.
- Continue processing normally.

Use case:

- Audit-only mode for monitoring content safety.
- Low-friction deployment for testing.

## Storage Changes

Add content safety detection to `internal/audit`:

- Add `ContentSafetyDetector` interface:
  ```go
  type ContentSafetyDetector interface {
      Detect(ctx context.Context, text string) ContentSafetyResult
  }
  ```

- Add `ContentSafetyMatch` struct:
  ```go
  type ContentSafetyMatch struct {
      Category string // "politics", "pornography", "violence", "advertising"
      Keyword  string // matched keyword
      Position int    // position in text
  }
  ```

- Add `ContentSafetyResult` struct:
  ```go
  type ContentSafetyResult struct {
      HasViolation bool
      Matches      []ContentSafetyMatch
      ByCategory   map[string]int // count by category
  }
  ```

- Implement `KeywordDetector`:
  - Load keyword lists from files.
  - Support wildcard matching.
  - Support threshold levels.
  - Thread-safe keyword access.

- Add `ContentSafetyAuditorRecorder` wrapper:
  - Wraps `JSONLRecorder` (or `PIIAuditorRecorder`).
  - Detects content safety violations in request/response bodies.
  - Adds `content_safety_violation` field to audit event.
  - Logs violations.
  - Implements action logic (alert/reject/allow).

## Integration

Config layer loads:

- `content_safety.enabled`
- `content_safety.action`
- `content_safety.categories`
- `content_safety.custom_keywords.paths`
- `content_safety.threshold`

Command layer:

- Build content safety detector when enabled.
- Load custom keyword files.
- Wrap audit recorder with content safety detection.
- Pass to API server.
- Include content safety status in startup logs.

API layer:

- If `action` is `reject`, check content safety before forwarding request.
- If violation detected, return `400 invalid_request_error`.
- Otherwise, continue processing.

Audit layer:

- After recording request/response, detect content safety violations.
- Add `content_safety_violation` field to audit event.
- Log violations based on `log_matches` setting.

## Performance Considerations

- Keyword-based detection is very fast (< 1ms for typical request bodies).
- Detection runs asynchronously after request completion (except for `action: reject`).
- For `action: reject`, detection runs synchronously before upstream call.
- Performance impact is negligible compared to model API latency.
- No significant throughput impact.
- Keyword loading at startup is one-time cost.

## Testing

Unit and integration tests should cover:

- Config defaults keep content safety disabled.
- Config can enable content safety and set action.
- Environment variable overrides work correctly.
- Invalid action fails validation.
- Empty categories list fails validation.
- Invalid threshold fails validation.
- Keyword detector detects politics keywords correctly.
- Keyword detector detects pornography keywords correctly.
- Keyword detector detects violence keywords correctly.
- Keyword detector detects advertising keywords correctly.
- Keyword detector handles custom keyword files.
- Keyword detector handles wildcard matching.
- Keyword detector handles threshold levels correctly.
- Content safety detection handles large texts (> 10KB).
- Content safety detection handles empty/nil bodies.
- ContentSafetyAuditorRecorder adds `content_safety_violation` field.
- `action: alert` logs warnings.
- `action: reject` blocks requests with violations.
- `action: allow` logs informationally.
- Performance: detection < 1ms P99.

Test utilities:

- Add `TestKeywordDetector` with comprehensive test cases.
- Add `TestContentSafetyAuditorRecorder` with all actions.
- Add performance tests for large texts.

Verification commands:

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/ai-gateway -- bash -lc "go test ./internal/audit -v -run TestKeywordDetector"
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/ai-gateway -- bash -lc "go test ./internal/config -v -run TestContentSafety"
```

## Security and Compliance Notes

### Data Sovereignty

- All content safety detection happens locally.
- No data is sent to external APIs.
- Meets data sovereignty requirements for regulated industries.
- Suitable for air-gapped deployment.

### Privacy Considerations

- Matched keywords are logged by default.
- Logs may contain sensitive context (configure `log_matches` carefully).
- Consider encrypting content safety logs separately.

### False Positives and Negatives

- Keyword-based detection has higher false positives than ML-based.
- Keywords may miss context-dependent violations.
- Test detection rules against your specific use cases.
- Use custom keyword files to tune for your domain.

### Deployment Recommendation

- Start with `action: alert` to understand violation patterns.
- Tune keywords and thresholds based on alerts.
- Move to `action: reject` when confident in detection accuracy.

## Implementation Phases

### Phase 1: Core Detection (2 weeks)

- Define `ContentSafetyDetector` interface.
- Implement `KeywordDetector` with built-in keyword lists.
- Implement `ContentSafetyMatch` and `ContentSafetyResult`.
- Add comprehensive unit tests.
- Add performance benchmarks.

### Phase 2: Custom Keywords (1 week)

- Implement keyword file loading.
- Implement wildcard matching.
- Implement threshold levels.
- Add tests for custom keywords.

### Phase 3: Configuration (1 week)

- Add `ContentSafetyConfig` to config.
- Add environment variable support.
- Add validation logic.
- Add config tests.

### Phase 4: Audit Integration (1 week)

- Implement `ContentSafetyAuditorRecorder` wrapper.
- Add `content_safety_violation` field to audit event.
- Implement action logic.
- Add integration tests.

### Phase 5: Gateway Integration (1 week)

- Integrate with command layer.
- Implement `action: reject` in API layer.
- Add startup logs.
- Add end-to-end tests.

### Phase 6: Documentation (1 week)

- Add usage documentation.
- Add deployment examples.
- Add troubleshooting guide.
- Add compliance guidance.

Total estimated time: 6 weeks.

## Open Questions Deferred

- ML-based content safety detection for higher accuracy.
- Image/video/audio content safety.
- Integration with existing DLP systems.
- Content redaction/masking.
- Real-time keyword file updates (currently requires restart).
- Multi-language support beyond Chinese.