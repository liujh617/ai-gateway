# Web Dashboard Design

## Goal

Add a simple web dashboard for AI Gateway management and monitoring. Provide visual interface for audit log viewing, PII/content safety detection results, alert history, and basic configuration management.

## Non-Goals

- This is not a complex enterprise admin platform (keep it simple).
- This does not include advanced analytics or BI features.
- This does not support multi-tenant management in MVP.
- This does not include user authentication/authorization in MVP (rely on network-level security).

## Target Users

1. **Security Team**: Review PII leaks, content safety violations, alert history
2. **Ops Team**: Monitor audit logs, check gateway status, view statistics
3. **Admin Team**: Manage configuration, adjust detection rules, maintain keyword files

## Core Features

### Page 1: Dashboard Overview

**Purpose**: Quick overview of gateway status and key metrics

**Components**:
- Gateway status (running/stopped)
- Total requests (today/week/month)
- PII detection summary (total violations, by type)
- Content safety summary (total violations, by category)
- Alert summary (total alerts, by channel)
- Recent alerts (last 10)

**Data Source**: Aggregated from JSONL logs

### Page 2: Audit Logs

**Purpose**: Search and view audit logs

**Components**:
- Search filters:
  - Time range (last 1h/6h/24h/7d/30d, custom)
  - Provider (OpenAI/Anthropic/...)
  - Model (gpt-4/claude-3/...)
  - Status code (200/4xx/5xx)
  - Request ID (exact match)
- Results table:
  - Timestamp
  - Provider/Model
  - Method (chat/completions/embeddings)
  - Status
  - Latency
  - PII detected (yes/no)
  - Content safety (safe/unsafe)
  - Request ID (click to view details)
- Detail modal:
  - Full request/response (PII redacted)
  - Metadata (tokens, latency, model)
  - PII detection results (if any)
  - Content safety results (if any)

**Data Source**: `audit.jsonl` (decrypted on-the-fly)

### Page 3: PII Detection

**Purpose**: View PII detection results and statistics

**Components**:
- Summary cards:
  - Total PII violations
  - By type (phone/ID card/bank card)
  - By action (alert/reject)
- Trend chart (daily/weekly)
- Violation list:
  - Timestamp
  - Request ID
  - PII type
  - Action taken
  - Sample (redacted: `138****5678`)
  - View details

**Data Source**: PII audit records

### Page 4: Content Safety

**Purpose**: View content safety detection results and statistics

**Components**:
- Summary cards:
  - Total violations
  - By category (politics/pornography/violence/advertising)
  - By action (alert/reject)
- Trend chart (daily/weekly)
- Violation list:
  - Timestamp
  - Request ID
  - Category
  - Keywords matched
  - Action taken
  - View details

**Data Source**: Content safety audit records

### Page 5: Alerts

**Purpose**: View alert history and delivery status

**Components**:
- Summary cards:
  - Total alerts
  - By channel (webhook/email/slack)
  - By status (sent/failed)
- Trend chart (daily/weekly)
- Alert list:
  - Timestamp
  - Source (PII/content safety)
  - Severity (low/medium/high/critical)
  - Channel
  - Status (sent/failed)
  - Error message (if failed)
  - View details

**Data Source**: Alert storage (`alerts.jsonl`)

### Page 6: Configuration (Optional)

**Purpose**: View and edit gateway configuration

**Components**:
- View current config (read-only in MVP)
- Download config file
- Restart gateway button (requires confirmation)

**Data Source**: `config.json`

## Technical Architecture

### Backend API

**Technology**: Go HTTP server (reuse existing gateway infrastructure)

**Endpoints**:

```
GET  /api/v1/status          # Gateway status and health
GET  /api/v1/stats           # Summary statistics
GET  /api/v1/audit/logs      # List audit logs with filters
GET  /api/v1/audit/logs/:id  # Get audit log detail
GET  /api/v1/pii/violations  # List PII violations
GET  /api/v1/pii/stats       # PII statistics
GET  /api/v1/content-safety/violations  # List content safety violations
GET  /api/v1/content-safety/stats       # Content safety statistics
GET  /api/v1/alerts          # List alerts
GET  /api/v1/alerts/stats    # Alert statistics
GET  /api/v1/config          # Get current configuration
POST /api/v1/config/reload   # Reload configuration (optional)
```

**Data Access**:
- Read JSONL files directly
- Decrypt audit logs using encryption key (if enabled)
- Aggregate statistics in-memory (no database required)

### Frontend

**Technology**: React + Ant Design (or simpler alternatives)

**Options**:
1. **Option A (Recommended)**: React + Ant Design
   - Rich components (Table, Modal, DatePicker)
   - Good documentation
   - Larger bundle size (~500KB gzipped)

2. **Option B**: Vue 3 + Element Plus
   - Lighter weight
   - Similar component library
   - Smaller bundle size (~300KB gzipped)

3. **Option C**: Vanilla JS + Tailwind CSS
   - Minimal dependencies
   - Smallest bundle size (~50KB)
   - More manual work

**Build**:
- Single-page application
- Static files served by Go backend
- API calls to `/api/v1/*` endpoints

### Deployment

**Mode 1: Integrated** (Recommended for MVP)
- Dashboard served by gateway itself
- Single binary, single port
- Access: `http://gateway-host:port/dashboard/`

**Mode 2: Separate** (Optional for production)
- Dashboard as separate service
- Independent scaling
- Requires CORS configuration

## Security Considerations

### MVP (No Authentication)

- Dashboard accessible only from trusted network
- Recommend: VPN or private VPC
- Do NOT expose to public internet without auth

### Future Enhancements

- Basic authentication (username/password)
- OAuth2 integration (GitHub, Google)
- RBAC (role-based access control)
- Audit trail for dashboard actions

## Performance

### Target Metrics

- Page load: < 2s (first load), < 500ms (subsequent)
- API response: < 500ms (P99)
- Support: 10 concurrent users (MVP)
- Log search: < 3s for 100K logs

### Optimization Strategies

- Pagination for log lists (50 items per page)
- Time range filters (avoid full scan)
- Cache statistics (refresh every 30s)
- Lazy load detail modals

## Development Plan

### Phase 1: Backend API (3 days)

- Implement API endpoints
- Add JSONL reader and parser
- Add decryption logic for audit logs
- Add statistics aggregation
- Add unit tests

### Phase 2: Frontend Core (3 days)

- Setup React project
- Implement layout and routing
- Implement Dashboard Overview page
- Implement Audit Logs page
- Add search filters and pagination

### Phase 3: Frontend Features (2 days)

- Implement PII Detection page
- Implement Content Safety page
- Implement Alerts page
- Add charts and visualizations

### Phase 4: Integration (1 day)

- Integrate frontend with backend
- Test all pages end-to-end
- Fix bugs and edge cases

### Phase 5: Documentation (1 day)

- Add usage documentation
- Add deployment guide
- Add troubleshooting

**Total Estimate**: 10 days (2 weeks)

## Alternatives Considered

### Alternative 1: CLI Tools Only

- **Pros**: No UI development, suitable for technical users
- **Cons**: Not user-friendly for security/ops teams, hard to visualize trends

### Alternative 3: External Dashboard (Grafana)

- **Pros**: Rich visualization, alerting integration
- **Cons**: Requires additional infrastructure, steeper learning curve

## Success Criteria

- Dashboard shows accurate audit log data
- Search filters work correctly
- PII/content safety results displayed clearly
- Alert history visible with delivery status
- Page load < 2s for normal datasets
- Works in Chrome/Firefox/Safari