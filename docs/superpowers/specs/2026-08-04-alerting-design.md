# Alerting System Design

## Goal

Add a flexible alerting system to notify administrators when security violations are detected (PII leaks, content safety violations, suspicious patterns). This completes the risk management loop: Audit → Detect → **Alert**, enabling timely response to AI security incidents.

## Non-Goals

- This is not a real-time streaming alert system (alerts are processed asynchronously).
- This does not include automatic incident remediation (notification only).
- This does not integrate with ticketing systems (e.g., Jira, ServiceNow) in MVP.
- This does not support SMS alerts in MVP (may add later).
- This does not provide alert analytics dashboards (audit logs serve this purpose).

## Alert Sources

Alerts are triggered by:

1. **PII Detection Violations**
   - PII detected in request/response
   - Action mode: `alert` or `reject`

2. **Content Safety Violations**
   - Unsafe content detected (politics/pornography/violence/advertising)
   - Action mode: `alert` or `reject`

3. **Audit Anomalies** (Future)
   - Unusual request patterns
   - High error rates
   - Suspicious user behavior

## Configuration

Add an `alerting` section to the top-level config:

```json
{
  "alerting": {
    "enabled": true,
    "channels": [
      {
        "type": "webhook",
        "name": "security-team",
        "enabled": true,
        "config": {
          "url": "https://hooks.example.com/alert",
          "method": "POST",
          "headers": {
            "Authorization": "Bearer token123"
          },
          "timeout_seconds": 10
        }
      },
      {
        "type": "email",
        "name": "security-email",
        "enabled": true,
        "config": {
          "smtp_host": "smtp.example.com",
          "smtp_port": 587,
          "smtp_user": "alerts@example.com",
          "smtp_password": "${SMTP_PASSWORD}",
          "from": "alerts@example.com",
          "to": ["security-team@example.com"]
        }
      },
      {
        "type": "slack",
        "name": "security-slack",
        "enabled": true,
        "config": {
          "webhook_url": "https://hooks.slack.com/services/T00/B00/XXX",
          "channel": "#security-alerts",
          "username": "AI Gateway Alert"
        }
      }
    ],
    "rules": [
      {
        "name": "pii-alert",
        "enabled": true,
        "source": "pii_detection",
        "severity": "high",
        "cooldown_minutes": 5,
        "channels": ["security-team", "security-email"]
      },
      {
        "name": "content-safety-alert",
        "enabled": true,
        "source": "content_safety",
        "severity": "medium",
        "cooldown_minutes": 10,
        "channels": ["security-team", "security-slack"]
      }
    ],
    "rate_limit": {
      "enabled": true,
      "max_alerts_per_minute": 10,
      "burst_size": 20
    },
    "retry": {
      "enabled": true,
      "max_retries": 3,
      "retry_delay_seconds": 5
    },
    "storage": {
      "enabled": true,
      "path": "data/alerts.jsonl",
      "max_file_bytes": 104857600
    }
  }
}
```

Defaults:

- `alerting.enabled`: `false`
- `alerting.channels`: `[]` (must be configured)
- `alerting.rules`: `[]` (must be configured)
- `alerting.rate_limit.enabled`: `true`
- `alerting.rate_limit.max_alerts_per_minute`: `10`
- `alerting.rate_limit.burst_size`: `20`
- `alerting.retry.enabled`: `true`
- `alerting.retry.max_retries`: `3`
- `alerting.retry.retry_delay_seconds`: `5`
- `alerting.storage.enabled`: `true`
- `alerting.storage.path`: `"data/alerts.jsonl"`
- `alerting.storage.max_file_bytes`: `104857600` (100MB)

Environment overrides:

- `GATEWAY_ALERTING_ENABLED`: Override `alerting.enabled`
- `GATEWAY_ALERTING_RATE_LIMIT`: Override `alerting.rate_limit.max_alerts_per_minute`

Validation:

- `channels` must be a non-empty array if `enabled` is true.
- Each channel must have a valid `type` (webhook, email, slack).
- Each channel must have required config fields for its type.
- `rules` must be a non-empty array if `enabled` is true.
- Each rule must reference existing channel names.

## Alert Structure

```go
type Alert struct {
    ID           string                 `json:"id"`
    Timestamp    time.Time              `json:"timestamp"`
    Source       string                 `json:"source"`        // "pii_detection", "content_safety"
    Severity     string                 `json:"severity"`      // "low", "medium", "high", "critical"
    Title        string                 `json:"title"`
    Description  string                 `json:"description"`
    RequestID    string                 `json:"request_id"`
    TraceID      string                 `json:"trace_id"`
    Client       string                 `json:"client"`
    Provider     string                 `json:"provider"`
    Model        string                 `json:"model"`
    Details      map[string]interface{} `json:"details"`       // Source-specific details
    Status       string                 `json:"status"`        // "pending", "sent", "failed"
    SentAt       *time.Time             `json:"sent_at,omitempty"`
    Error        string                 `json:"error,omitempty"`
    Channels     []string               `json:"channels"`      // Channels notified
    RetryCount   int                    `json:"retry_count"`
}
```

Example alert:

```json
{
  "id": "alert-20260804-001",
  "timestamp": "2026-08-04T21:30:00Z",
  "source": "pii_detection",
  "severity": "high",
  "title": "PII Detected in Request",
  "description": "Phone number detected in user request",
  "request_id": "req-abc123",
  "trace_id": "trace-xyz789",
  "client": "app-server-1",
  "provider": "openai",
  "model": "gpt-4",
  "details": {
    "type": "phone_number",
    "count": 2,
    "action": "alert",
    "sample": "138****5678"
  },
  "status": "sent",
  "sent_at": "2026-08-04T21:30:01Z",
  "channels": ["security-team", "security-email"],
  "retry_count": 0
}
```

## Alert Channels

### Webhook Channel

Supports generic HTTP webhook notifications.

Configuration:

```json
{
  "type": "webhook",
  "name": "custom-webhook",
  "enabled": true,
  "config": {
    "url": "https://api.example.com/alerts",
    "method": "POST",
    "headers": {
      "Authorization": "Bearer token123",
      "Content-Type": "application/json"
    },
    "timeout_seconds": 10
  }
}
```

Payload format:

```json
{
  "id": "alert-20260804-001",
  "timestamp": "2026-08-04T21:30:00Z",
  "source": "pii_detection",
  "severity": "high",
  "title": "PII Detected in Request",
  "description": "Phone number detected in user request",
  "details": {
    "type": "phone_number",
    "count": 2
  }
}
```

Response handling:

- Status codes `200-299`: Success
- Other status codes: Failure, will retry
- Timeout after `timeout_seconds`

### Email Channel

Supports SMTP email notifications.

Configuration:

```json
{
  "type": "email",
  "name": "security-email",
  "enabled": true,
  "config": {
    "smtp_host": "smtp.example.com",
    "smtp_port": 587,
    "smtp_user": "alerts@example.com",
    "smtp_password": "${SMTP_PASSWORD}",
    "from": "AI Gateway Alerts <alerts@example.com>",
    "to": ["security-team@example.com", "oncall@example.com"],
    "subject_template": "[{{.Severity}}] {{.Title}}"
  }
}
```

Email format:

```
Subject: [HIGH] PII Detected in Request

Alert ID: alert-20260804-001
Timestamp: 2026-08-04 21:30:00 UTC
Source: PII Detection
Severity: HIGH

Title: PII Detected in Request
Description: Phone number detected in user request

Details:
- Type: phone_number
- Count: 2
- Action: alert
- Sample: 138****5678

Request Context:
- Request ID: req-abc123
- Trace ID: trace-xyz789
- Client: app-server-1
- Provider: openai
- Model: gpt-4

---
This is an automated alert from AI Gateway.
```

### Slack Channel

Supports Slack incoming webhook notifications.

Configuration:

```json
{
  "type": "slack",
  "name": "security-slack",
  "enabled": true,
  "config": {
    "webhook_url": "https://hooks.slack.com/services/T00/B00/XXX",
    "channel": "#security-alerts",
    "username": "AI Gateway Alert",
    "icon_emoji": ":warning:"
  }
}
```

Message format:

```json
{
  "channel": "#security-alerts",
  "username": "AI Gateway Alert",
  "icon_emoji": ":warning:",
  "attachments": [
    {
      "color": "danger",
      "title": "[HIGH] PII Detected in Request",
      "text": "Phone number detected in user request",
      "fields": [
        {"title": "Source", "value": "PII Detection", "short": true},
        {"title": "Severity", "value": "HIGH", "short": true},
        {"title": "Request ID", "value": "req-abc123", "short": true},
        {"title": "Trace ID", "value": "trace-xyz789", "short": true}
      ],
      "footer": "AI Gateway Alert",
      "ts": 1722807000
    }
  ]
}
```

## Alert Rules

Alert rules define when and how alerts are sent.

### Rule Configuration

```go
type AlertRule struct {
    Name            string   `json:"name"`
    Enabled         bool     `json:"enabled"`
    Source          string   `json:"source"`          // "pii_detection", "content_safety"
    Severity        string   `json:"severity"`        // "low", "medium", "high", "critical"
    CooldownMinutes int      `json:"cooldown_minutes"` // Prevent alert spam
    Channels        []string `json:"channels"`        // Channel names to notify
    Filters         map[string]interface{} `json:"filters,omitempty"` // Optional filters
}
```

### Severity Levels

- `low`: Informational, no immediate action required
- `medium`: Potential issue, may require investigation
- `high`: Significant issue, requires prompt attention
- `critical`: Severe issue, requires immediate action

### Cooldown Mechanism

Prevents alert spam by suppressing duplicate alerts for the same source within a cooldown period.

Example:

- Rule: `pii-alert` with `cooldown_minutes: 5`
- First PII violation at `21:30:00`: Alert sent
- Second PII violation at `21:32:00`: Alert suppressed (within cooldown)
- Third PII violation at `21:36:00`: Alert sent (cooldown expired)

### Filtering (Optional)

Rules can filter alerts based on:

```json
{
  "name": "high-severity-pii",
  "source": "pii_detection",
  "severity": "high",
  "filters": {
    "min_count": 3,
    "providers": ["openai", "anthropic"],
    "clients": ["production-*"]
  }
}
```

## Alert Flow

### 1. Alert Generation

```
PII Detection → PIIAuditor → AlertManager.CreateAlert()
Content Safety → ContentSafetyAuditor → AlertManager.CreateAlert()
```

### 2. Alert Processing

```
AlertManager.CreateAlert()
  ↓
Check Rate Limit
  ↓ (passed)
Check Cooldown
  ↓ (not in cooldown)
Create Alert Record
  ↓
Queue for Delivery
  ↓
AlertWorker.ProcessQueue()
```

### 3. Alert Delivery

```
AlertWorker.ProcessQueue()
  ↓
For each channel:
  ↓
Channel.Send(alert)
  ↓
Success? → Mark as "sent"
Failure? → Retry or mark as "failed"
  ↓
Log to storage
```

## Rate Limiting

Prevents alert storms.

Configuration:

```json
{
  "rate_limit": {
    "enabled": true,
    "max_alerts_per_minute": 10,
    "burst_size": 20
  }
}
```

Implementation:

- Token bucket algorithm
- Global rate limit across all sources
- Excess alerts are logged but not sent

## Retry Mechanism

Handles transient delivery failures.

Configuration:

```json
{
  "retry": {
    "enabled": true,
    "max_retries": 3,
    "retry_delay_seconds": 5
  }
}
```

Retry logic:

- Retry on: network errors, timeout, 5xx status codes
- No retry on: invalid configuration, authentication errors, 4xx status codes
- Exponential backoff: `delay = base_delay * 2^retry_count`

## Alert Storage

Persists all alerts for audit and analysis.

Configuration:

```json
{
  "storage": {
    "enabled": true,
    "path": "data/alerts.jsonl",
    "max_file_bytes": 104857600
  }
}
```

Format:

- JSONL (JSON Lines) format
- One alert per line
- Same rotation logic as audit logs

Example file:

```json
{"id":"alert-001","timestamp":"2026-08-04T21:30:00Z","source":"pii_detection","severity":"high","title":"PII Detected","status":"sent"}
{"id":"alert-002","timestamp":"2026-08-04T21:35:00Z","source":"content_safety","severity":"medium","title":"Unsafe Content","status":"sent"}
```

## Integration Points

### 1. PII Detection Integration

```go
// In PIIAuditor
if result.HasPII {
    // Create alert
    if a.alertManager != nil {
        alert := &Alert{
            Source:   "pii_detection",
            Severity: "high",
            Title:    "PII Detected in Request",
            Details: map[string]interface{}{
                "type":   result.Types,
                "count":  result.Count,
                "action": a.action,
            },
        }
        a.alertManager.CreateAlert(ctx, alert)
    }
}
```

### 2. Content Safety Integration

```go
// In ContentSafetyAuditor
if result.HasViolation {
    // Create alert
    if a.alertManager != nil {
        alert := &Alert{
            Source:   "content_safety",
            Severity: severityMap[result.Severity],
            Title:    "Content Safety Violation",
            Details: map[string]interface{}{
                "categories": result.Categories,
                "matches":    result.Matches,
            },
        }
        a.alertManager.CreateAlert(ctx, alert)
    }
}
```

### 3. Gateway Startup

```go
// In cmd/gateway/main.go
func buildAlertManager(cfg *config.Config) (*alert.AlertManager, error) {
    if !cfg.Alerting.Enabled {
        return nil, nil
    }

    // Build channels
    channels := make(map[string]alert.Channel)
    for _, ch := range cfg.Alerting.Channels {
        switch ch.Type {
        case "webhook":
            channels[ch.Name] = alert.NewWebhookChannel(ch.Config)
        case "email":
            channels[ch.Name] = alert.NewEmailChannel(ch.Config)
        case "slack":
            channels[ch.Name] = alert.NewSlackChannel(ch.Config)
        }
    }

    // Build alert manager
    return alert.NewAlertManager(alert.AlertManagerOptions{
        Channels:  channels,
        Rules:     cfg.Alerting.Rules,
        RateLimit: cfg.Alerting.RateLimit,
        Storage:   cfg.Alerting.Storage,
    })
}
```

## Testing

### Unit Tests

- Channel implementations (webhook, email, slack)
- Alert manager logic
- Rate limiting
- Cooldown mechanism
- Retry logic

### Integration Tests

- End-to-end alert flow
- PII detection → alert generation
- Content safety → alert generation
- Alert delivery to channels

### Performance Tests

- Alert throughput: > 100 alerts/second
- Channel send latency: < 500ms P99
- Memory overhead: < 50MB

## Security Considerations

1. **Sensitive Data**: Never include raw PII in alert messages (use masked samples)
2. **Authentication**: Secure webhook endpoints with tokens
3. **Encryption**: Use TLS for all external communications
4. **Access Control**: Restrict alert configuration to administrators
5. **Audit Trail**: All alert actions logged to audit system

## Monitoring

Key metrics:

- `alert_total`: Total alerts generated
- `alert_sent_total`: Alerts successfully sent
- `alert_failed_total`: Alerts failed to send
- `alert_channel_latency_ms`: Channel send latency
- `alert_rate_limit_hits`: Rate limit triggered count

## Deployment Recommendations

### MVP Deployment

```json
{
  "alerting": {
    "enabled": true,
    "channels": [
      {
        "type": "webhook",
        "name": "security-webhook",
        "config": {
          "url": "https://api.company.com/security/alerts",
          "method": "POST",
          "headers": {
            "Authorization": "Bearer ${SECURITY_WEBHOOK_TOKEN}"
          }
        }
      }
    ],
    "rules": [
      {
        "name": "pii-alert",
        "source": "pii_detection",
        "severity": "high",
        "cooldown_minutes": 5,
        "channels": ["security-webhook"]
      },
      {
        "name": "content-safety-alert",
        "source": "content_safety",
        "severity": "medium",
        "cooldown_minutes": 10,
        "channels": ["security-webhook"]
      }
    ]
  }
}
```

### Production Deployment

- Multiple channels for redundancy (webhook + email + Slack)
- Lower rate limits for high-traffic environments
- Enable alert storage for audit trail
- Monitor alert delivery metrics

## Future Enhancements

1. **More Channels**: WeChat Work, Microsoft Teams, PagerDuty
2. **Alert Analytics**: Dashboard for alert trends
3. **Auto-Remediation**: Automatic response to certain alerts
4. **ML-Based Detection**: Anomaly detection for unusual patterns
5. **Escalation Policies**: Automatic escalation for critical alerts