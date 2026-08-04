package dashboard

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// LogReader reads and parses JSONL audit logs
type LogReader struct {
	auditPath     string
	encryptionKey []byte
	decrypt       bool
}

// AuditLog represents a single audit log entry
type AuditLog struct {
	Timestamp     time.Time              `json:"timestamp"`
	RequestID     string                 `json:"request_id"`
	TraceID       string                 `json:"trace_id"`
	Method        string                 `json:"method"`
	Path          string                 `json:"path"`
	Provider      string                 `json:"provider"`
	Model         string                 `json:"model"`
	StatusCode    int                    `json:"status_code"`
	LatencyMs     int64                  `json:"latency_ms"`
	Tokens        *TokenUsage            `json:"tokens,omitempty"`
	PIIDetected   bool                   `json:"pii_detected"`
	ContentSafety bool                   `json:"content_safety_violation"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// TokenUsage represents token usage information
type TokenUsage struct {
	Prompt     int `json:"prompt"`
	Completion int `json:"completion"`
	Total      int `json:"total"`
}

// AuditLogFilter represents filters for audit logs
type AuditLogFilter struct {
	StartTime *time.Time
	EndTime   *time.Time
	Provider  string
	Model     string
	Status    *int
	RequestID string
	Page      int
	Size      int
}

// ParseAuditLogFilter parses query parameters into AuditLogFilter
func ParseAuditLogFilter(r *interface{}) *AuditLogFilter {
	// Will be implemented when integrating with HTTP handlers
	return &AuditLogFilter{
		Page: 1,
		Size: 50,
	}
}

// NewLogReader creates a new LogReader instance
func NewLogReader(auditPath string, encryptionKey []byte, decrypt bool) (*LogReader, error) {
	// Check if audit path exists
	if _, err := os.Stat(auditPath); err != nil {
		// Create directory if not exists
		if err := os.MkdirAll(filepath.Dir(auditPath), 0755); err != nil {
			return nil, fmt.Errorf("create audit directory: %w", err)
		}
	}

	return &LogReader{
		auditPath:     auditPath,
		encryptionKey: encryptionKey,
		decrypt:       decrypt,
	}, nil
}

// ListLogs returns a paginated list of audit logs
func (lr *LogReader) ListLogs(ctx context.Context, filter *AuditLogFilter) ([]*AuditLog, int, error) {
	file, err := os.Open(lr.auditPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []*AuditLog{}, 0, nil
		}
		return nil, 0, fmt.Errorf("open audit file: %w", err)
	}
	defer file.Close()

	var logs []*AuditLog
	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		var log AuditLog
		if err := json.Unmarshal([]byte(line), &log); err != nil {
			// Skip malformed lines
			continue
		}

		// Apply filters
		if !lr.matchesFilter(&log, filter) {
			continue
		}

		// Decrypt body if needed
		if lr.decrypt {
			// TODO: Decrypt body_encrypted field
		}

		logs = append(logs, &log)

		// Check context cancellation
		if lineNum%1000 == 0 {
			select {
			case <-ctx.Done():
				return nil, 0, ctx.Err()
			default:
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, 0, fmt.Errorf("scan audit file: %w", err)
	}

	// Apply pagination
	total := len(logs)
	start := (filter.Page - 1) * filter.Size
	end := start + filter.Size

	if start >= total {
		return []*AuditLog{}, total, nil
	}
	if end > total {
		end = total
	}

	return logs[start:end], total, nil
}

// GetLogDetail returns a single audit log by request ID
func (lr *LogReader) GetLogDetail(ctx context.Context, requestID string) (*AuditLog, error) {
	file, err := os.Open(lr.auditPath)
	if err != nil {
		return nil, fmt.Errorf("open audit file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()

		var log AuditLog
		if err := json.Unmarshal([]byte(line), &log); err != nil {
			continue
		}

		if log.RequestID == requestID {
			// Decrypt body if needed
			if lr.decrypt {
				// TODO: Decrypt body_encrypted field
			}
			return &log, nil
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan audit file: %w", err)
	}

	return nil, fmt.Errorf("audit log not found: %s", requestID)
}

// matchesFilter checks if a log entry matches the filter criteria
func (lr *LogReader) matchesFilter(log *AuditLog, filter *AuditLogFilter) bool {
	if filter == nil {
		return true
	}

	if filter.StartTime != nil && log.Timestamp.Before(*filter.StartTime) {
		return false
	}

	if filter.EndTime != nil && log.Timestamp.After(*filter.EndTime) {
		return false
	}

	if filter.Provider != "" && log.Provider != filter.Provider {
		return false
	}

	if filter.Model != "" && log.Model != filter.Model {
		return false
	}

	if filter.Status != nil && log.StatusCode != *filter.Status {
		return false
	}

	if filter.RequestID != "" && log.RequestID != filter.RequestID {
		return false
	}

	return true
}

// StreamLogs streams audit logs in real-time (for future use)
func (lr *LogReader) StreamLogs(ctx context.Context) (<-chan *AuditLog, error) {
	// TODO: Implement tail -f style streaming
	return nil, fmt.Errorf("not implemented")
}

// Close closes the log reader
func (lr *LogReader) Close() error {
	// No persistent resources to close
	return nil
}