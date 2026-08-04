package alert

import (
	"context"
	"time"
)

// Alert represents a security alert
type Alert struct {
	ID          string                 `json:"id"`
	Timestamp   time.Time              `json:"timestamp"`
	Source      string                 `json:"source"`       // "pii_detection", "content_safety"
	Severity     string                 `json:"severity"`     // "low", "medium", "high", "critical"
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	RequestID   string                 `json:"request_id,omitempty"`
	TraceID     string                 `json:"trace_id,omitempty"`
	Client      string                 `json:"client,omitempty"`
	Provider    string                 `json:"provider,omitempty"`
	Model       string                 `json:"model,omitempty"`
	Details     map[string]interface{} `json:"details,omitempty"`
	Status      string                 `json:"status"` // "pending", "sent", "failed"
	SentAt      *time.Time             `json:"sent_at,omitempty"`
	Error       string                 `json:"error,omitempty"`
	Channels    []string               `json:"channels"`
	RetryCount  int                    `json:"retry_count"`
}

// Channel defines the interface for alert delivery channels
type Channel interface {
	// Name returns the channel name
	Name() string

	// Send sends an alert to the channel
	Send(ctx context.Context, alert *Alert) error

	// Close closes the channel and releases resources
	Close() error
}

// AlertRule defines when and how alerts are sent
type AlertRule struct {
	Name            string                 `json:"name"`
	Enabled         bool                   `json:"enabled"`
	Source          string                 `json:"source"`           // "pii_detection", "content_safety"
	Severity        string                 `json:"severity"`         // "low", "medium", "high", "critical"
	CooldownMinutes int                    `json:"cooldown_minutes"` // Prevent alert spam
	Channels        []string               `json:"channels"`         // Channel names to notify
	Filters         map[string]interface{} `json:"filters,omitempty"` // Optional filters
}

// AlertManager manages alert creation, routing, and delivery
type AlertManager interface {
	// CreateAlert creates and queues a new alert
	CreateAlert(ctx context.Context, alert *Alert) error

	// Close shuts down the alert manager
	Close() error
}

// AlertManagerOptions configures the alert manager
type AlertManagerOptions struct {
	Channels  map[string]Channel
	Rules     []AlertRule
	RateLimit RateLimitConfig
	Retry     RetryConfig
	Storage   StorageConfig
}

// RateLimitConfig configures rate limiting
type RateLimitConfig struct {
	Enabled            bool `json:"enabled"`
	MaxAlertsPerMinute int  `json:"max_alerts_per_minute"`
	BurstSize         int  `json:"burst_size"`
}

// RetryConfig configures retry behavior
type RetryConfig struct {
	Enabled           bool `json:"enabled"`
	MaxRetries        int  `json:"max_retries"`
	RetryDelaySeconds int  `json:"retry_delay_seconds"`
}

// StorageConfig configures alert persistence
type StorageConfig struct {
	Enabled       bool   `json:"enabled"`
	Path          string `json:"path"`
	MaxFileBytes  int64  `json:"max_file_bytes"`
}

// Severity levels
const (
	SeverityLow      = "low"
	SeverityMedium   = "medium"
	SeverityHigh     = "high"
	SeverityCritical = "critical"
)

// Alert sources
const (
	SourcePIIDetection  = "pii_detection"
	SourceContentSafety = "content_safety"
)

// Alert status
const (
	StatusPending = "pending"
	StatusSent    = "sent"
	StatusFailed  = "failed"
)

// DefaultConfig returns default alert manager configuration
func DefaultConfig() AlertManagerOptions {
	return AlertManagerOptions{
		RateLimit: RateLimitConfig{
			Enabled:            true,
			MaxAlertsPerMinute: 10,
			BurstSize:         20,
		},
		Retry: RetryConfig{
			Enabled:           true,
			MaxRetries:        3,
			RetryDelaySeconds: 5,
		},
		Storage: StorageConfig{
			Enabled:       true,
			Path:          "data/alerts.jsonl",
			MaxFileBytes:  100 * 1024 * 1024, // 100MB
		},
	}
}