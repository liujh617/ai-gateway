# Alerting System - Usage Guide

## Overview

The alerting system notifies administrators when security violations are detected (PII leaks, content safety violations). This completes the risk management loop: **Audit → Detect → Alert**, enabling timely response to AI security incidents.

## Core Capabilities

- ✅ **Multi-Channel Support**: Webhook, Email, Slack
- ✅ **Alert Rules**: Route different violations to different channels
- ✅ **Severity Levels**: low, medium, high, critical
- ✅ **Rate Limiting**: Prevent alert storms (Token bucket algorithm)
- ✅ **Cooldown Mechanism**: Suppress duplicate alerts
- ✅ **Retry Logic**: Handle transient failures with exponential backoff
- ✅ **Alert Storage**: Persistent JSONL storage for audit trail

## Quick Start

### 1. Configuration File

Add `alerting` section to `gateway-config.json`:

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
      }
    ],
    "rules": [
      {
        "name": "pii-alert",
        "enabled": true,
        "source": "pii_detection",
        "severity": "high",
        "cooldown_minutes": 5,
        "channels": ["security-team"]
      },
      {
        "name": "content-safety-alert",
        "enabled": true,
        "source": "content_safety",
        "severity": "medium",
        "cooldown_minutes": 10,
        "channels": ["security-team"]
      }
    ]
  }
}
```

### 2. Configuration Reference

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | false | Enable/disable alerting |
| `channels` | []ChannelConfig | [] | Alert delivery channels |
| `rules` | []AlertRule | [] | Alert routing rules |
| `rate_limit.enabled` | bool | true | Enable rate limiting |
| `rate_limit.max_alerts_per_minute` | int | 10 | Max alerts per minute |
| `rate_limit.burst_size` | int | 20 | Burst capacity |
| `retry.enabled` | bool | true | Enable retry on failure |
| `retry.max_retries` | int | 3 | Max retry attempts |
| `retry.retry_delay_seconds` | int | 5 | Base retry delay |
| `storage.enabled` | bool | true | Enable alert persistence |
| `storage.path` | string | "data/alerts.jsonl" | Storage file path |
| `storage.max_file_bytes` | int64 | 100MB | Max file size |

## Alert Channels

### Webhook Channel

Send alerts to custom HTTP endpoints.

**Configuration:**

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

**Required fields:**
- `url`: Webhook endpoint URL

**Optional fields:**
- `method`: HTTP method (default: POST)
- `headers`: Custom headers
- `timeout_seconds`: Request timeout (default: 10)

**Payload format:**

```json
{
  "id": "alert-abc123",
  "timestamp": "2026-08-04T21:30:00Z",
  "source": "pii_detection",
  "severity": "high",
  "title": "PII Detected in Request",
  "description": "Phone number detected",
  "request_id": "req-123",
  "trace_id": "trace-456",
  "details": {
    "type": "phone_number",
    "count": 2
  }
}
```

### Email Channel

Send alerts via SMTP email.

**Configuration:**

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
    "to": ["security-team@example.com"],
    "subject_template": "[{{.Severity}}] {{.Title}}"
  }
}
```

**Required fields:**
- `smtp_host`: SMTP server host
- `to`: Recipient email addresses

**Optional fields:**
- `smtp_port`: SMTP port (default: 587)
- `smtp_user`: SMTP username
- `smtp_password`: SMTP password (supports environment variables)
- `from`: Sender email address
- `subject_template`: Email subject template

**Environment variables:**

Use `${VAR_NAME}` syntax for sensitive values:

```json
{
  "smtp_password": "${SMTP_PASSWORD}"
}
```

### Slack Channel

Send alerts to Slack channels via Incoming Webhook.

**Configuration:**

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

**Required fields:**
- `webhook_url`: Slack Incoming Webhook URL

**Optional fields:**
- `channel`: Override default channel
- `username`: Bot username
- `icon_emoji`: Bot icon

**Message format:**

Alerts are formatted as Slack attachments with color-coded severity:
- **high/critical**: Red (danger)
- **medium**: Yellow (warning)
- **low**: Green (good)

## Alert Rules

Alert rules define when and how alerts are sent.

### Rule Configuration

```json
{
  "name": "pii-alert",
  "enabled": true,
  "source": "pii_detection",
  "severity": "high",
  "cooldown_minutes": 5,
  "channels": ["security-team", "security-email"]
}
```

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Rule name |
| `enabled` | bool | Enable/disable rule |
| `source` | string | Alert source (`pii_detection`, `content_safety`) |
| `severity` | string | Severity level (`low`, `medium`, `high`, `critical`) |
| `cooldown_minutes` | int | Cooldown period (suppress duplicates) |
| `channels` | []string | Channel names to notify |

### Severity Levels

- **low**: Informational, no immediate action required
- **medium**: Potential issue, may require investigation
- **high**: Significant issue, requires prompt attention
- **critical**: Severe issue, requires immediate action

### Cooldown Mechanism

Prevents alert spam by suppressing duplicate alerts within a cooldown period.

**Example:**

Rule with `cooldown_minutes: 5`:
- First violation at 21:30:00: Alert sent ✅
- Second violation at 21:32:00: Alert suppressed (within cooldown) ⏸️
- Third violation at 21:36:00: Alert sent ✅ (cooldown expired)

## Rate Limiting

Prevents alert storms during high-volume incidents.

**Configuration:**

```json
{
  "rate_limit": {
    "enabled": true,
    "max_alerts_per_minute": 10,
    "burst_size": 20
  }
}
```

**Behavior:**

- Token bucket algorithm
- Global limit across all sources
- Excess alerts are logged but not sent
- Recommended: 10-20 alerts/minute for most environments

## Retry Mechanism

Handles transient delivery failures.

**Configuration:**

```json
{
  "retry": {
    "enabled": true,
    "max_retries": 3,
    "retry_delay_seconds": 5
  }
}
```

**Retry logic:**

- **Retry on**: network errors, timeout, 5xx status codes
- **No retry**: invalid configuration, authentication errors, 4xx status codes
- **Backoff**: exponential (`delay = base_delay * 2^retry_count`)

## Alert Storage

Persists all alerts for audit and analysis.

**Configuration:**

```json
{
  "storage": {
    "enabled": true,
    "path": "data/alerts.jsonl",
    "max_file_bytes": 104857600
  }
}
```

**Format:**

JSONL (JSON Lines) format, one alert per line:

```json
{"id":"alert-001","timestamp":"2026-08-04T21:30:00Z","source":"pii_detection","severity":"high","title":"PII Detected","status":"sent"}
{"id":"alert-002","timestamp":"2026-08-04T21:35:00Z","source":"content_safety","severity":"medium","title":"Unsafe Content","status":"sent"}
```

**File rotation:**

- Automatic rotation when file exceeds `max_file_bytes`
- Rotated files: `alerts.jsonl.20260804-213000`
- Old files not automatically deleted

## Deployment Recommendations

### MVP Deployment

Minimal setup with webhook channel:

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
      }
    ]
  }
}
```

### Production Deployment

Redundant multi-channel setup:

```json
{
  "alerting": {
    "enabled": true,
    "channels": [
      {
        "type": "webhook",
        "name": "security-api",
        "config": {
          "url": "https://api.company.com/alerts"
        }
      },
      {
        "type": "email",
        "name": "security-email",
        "config": {
          "smtp_host": "smtp.company.com",
          "smtp_port": 587,
          "smtp_user": "alerts@company.com",
          "smtp_password": "${SMTP_PASSWORD}",
          "from": "AI Gateway <alerts@company.com>",
          "to": ["security-team@company.com", "oncall@company.com"]
        }
      },
      {
        "type": "slack",
        "name": "security-slack",
        "config": {
          "webhook_url": "${SLACK_WEBHOOK_URL}",
          "channel": "#security-alerts"
        }
      }
    ],
    "rules": [
      {
        "name": "pii-critical",
        "source": "pii_detection",
        "severity": "high",
        "cooldown_minutes": 5,
        "channels": ["security-api", "security-email", "security-slack"]
      },
      {
        "name": "content-safety-alert",
        "source": "content_safety",
        "severity": "medium",
        "cooldown_minutes": 10,
        "channels": ["security-slack"]
      }
    ]
  }
}
```

## Security Considerations

### PII Protection

Alerts **never** include raw PII data. Instead, masked samples are shown:

- Phone: `138****5678` (first 3 + last 4 digits)
- ID card: `310***********1234` (first 3 + last 4 digits)
- Bank card: `622***********5678` (first 3 + last 4 digits)

### Authentication

- Use Bearer tokens for webhook endpoints
- Use environment variables for sensitive credentials
- Enable TLS for all external communications

### Access Control

- Restrict alert configuration to administrators
- Audit alert configuration changes
- Monitor alert delivery metrics

## Monitoring

### Key Metrics

Monitor these metrics in production:

- `alert_total`: Total alerts generated
- `alert_sent_total`: Alerts successfully sent
- `alert_failed_total`: Alerts failed to send
- `alert_channel_latency_ms`: Channel send latency
- `alert_rate_limit_hits`: Rate limit triggered count

### Health Checks

Check alert system health:

```bash
# Check alert storage
ls -lh data/alerts.jsonl

# View recent alerts
tail -10 data/alerts.jsonl | jq .

# Test webhook endpoint
curl -X POST https://api.example.com/alerts \
  -H "Authorization: Bearer token123" \
  -H "Content-Type: application/json" \
  -d '{"test": true}'
```

## Troubleshooting

### Common Issues

**1. Alerts not being sent**

- Check if `alerting.enabled` is `true`
- Verify channel configuration
- Check rate limit settings
- Review logs for errors

**2. Webhook timeout**

- Increase `timeout_seconds`
- Check endpoint availability
- Verify network connectivity

**3. Email not received**

- Verify SMTP credentials
- Check spam folder
- Test SMTP connection manually

**4. Slack messages not appearing**

- Verify webhook URL
- Check channel permissions
- Test webhook with curl

### Debug Logging

Enable debug logging:

```json
{
  "log": {
    "level": "debug"
  }
}
```

Check logs for alert events:

```bash
grep "alert" logs/gateway.log
```

## Best Practices

1. **Start Simple**: Begin with one channel (webhook), add more as needed
2. **Set Appropriate Cooldown**: 5-10 minutes for most use cases
3. **Monitor Metrics**: Track alert volume and success rate
4. **Test Regularly**: Periodically test alert delivery
5. **Document Procedures**: Define response procedures for each alert type
6. **Secure Credentials**: Use environment variables for sensitive data
7. **Review Alerts**: Periodically review alert effectiveness

## Future Enhancements

Planned features (not in MVP):

- SMS notifications
- WeChat Work integration
- Microsoft Teams integration
- PagerDuty integration
- Alert analytics dashboard
- Auto-remediation workflows
- Escalation policies