package config

import (
	"fmt"
	"strings"
)

// AlertingConfig configures the alerting system
type AlertingConfig struct {
	Enabled   bool            `json:"enabled"`
	Channels  []ChannelConfig `json:"channels"`
	Rules     []AlertRule     `json:"rules"`
	RateLimit RateLimitConfig `json:"rate_limit"`
	Retry     RetryConfig     `json:"retry"`
	Storage   StorageConfig   `json:"storage"`
}

// ChannelConfig configures an alert channel
type ChannelConfig struct {
	Type    string                 `json:"type"`
	Name    string                 `json:"name"`
	Enabled bool                   `json:"enabled"`
	Config  map[string]interface{} `json:"config"`
}

// AlertRule configures an alert rule
type AlertRule struct {
	Name            string                 `json:"name"`
	Enabled         bool                   `json:"enabled"`
	Source          string                 `json:"source"`
	Severity        string                 `json:"severity"`
	CooldownMinutes int                    `json:"cooldown_minutes"`
	Channels        []string               `json:"channels"`
	Filters         map[string]interface{} `json:"filters,omitempty"`
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

// Validate validates the alerting configuration
func (c *AlertingConfig) Validate() error {
	if !c.Enabled {
		return nil
	}

	// Validate channels
	if len(c.Channels) == 0 {
		return fmt.Errorf("at least one channel is required when alerting is enabled")
	}

	channelNames := make(map[string]bool)
	for i, ch := range c.Channels {
		if ch.Name == "" {
			return fmt.Errorf("channel %d: name is required", i)
		}
		if !ch.Enabled {
			continue
		}
		if ch.Type == "" {
			return fmt.Errorf("channel %s: type is required", ch.Name)
		}
		if !isValidChannelType(ch.Type) {
			return fmt.Errorf("channel %s: invalid type %s (must be webhook, email, or slack)", ch.Name, ch.Type)
		}
		if err := validateChannelConfig(ch); err != nil {
			return fmt.Errorf("channel %s: %w", ch.Name, err)
		}
		channelNames[ch.Name] = true
	}

	// Validate rules
	if len(c.Rules) == 0 {
		return fmt.Errorf("at least one rule is required when alerting is enabled")
	}

	for i, rule := range c.Rules {
		if rule.Name == "" {
			return fmt.Errorf("rule %d: name is required", i)
		}
		if !rule.Enabled {
			continue
		}
		if !isValidSource(rule.Source) {
			return fmt.Errorf("rule %s: invalid source %s (must be pii_detection or content_safety)", rule.Name, rule.Source)
		}
		if !isValidSeverity(rule.Severity) {
			return fmt.Errorf("rule %s: invalid severity %s (must be low, medium, high, or critical)", rule.Name, rule.Severity)
		}
		if len(rule.Channels) == 0 {
			return fmt.Errorf("rule %s: at least one channel is required", rule.Name)
		}
		for _, chName := range rule.Channels {
			if !channelNames[chName] {
				return fmt.Errorf("rule %s: channel %s not found", rule.Name, chName)
			}
		}
	}

	// Validate rate limit
	if c.RateLimit.Enabled {
		if c.RateLimit.MaxAlertsPerMinute <= 0 {
			return fmt.Errorf("rate_limit.max_alerts_per_minute must be positive")
		}
		if c.RateLimit.BurstSize <= 0 {
			return fmt.Errorf("rate_limit.burst_size must be positive")
		}
	}

	// Validate retry
	if c.Retry.Enabled {
		if c.Retry.MaxRetries < 0 {
			return fmt.Errorf("retry.max_retries must be non-negative")
		}
		if c.Retry.RetryDelaySeconds <= 0 {
			return fmt.Errorf("retry.retry_delay_seconds must be positive")
		}
	}

	// Validate storage
	if c.Storage.Enabled {
		if c.Storage.Path == "" {
			return fmt.Errorf("storage.path is required")
		}
		if c.Storage.MaxFileBytes <= 0 {
			return fmt.Errorf("storage.max_file_bytes must be positive")
		}
	}

	return nil
}

// isValidChannelType checks if channel type is valid
func isValidChannelType(t string) bool {
	switch t {
	case "webhook", "email", "slack":
		return true
	default:
		return false
	}
}

// isValidSource checks if source is valid
func isValidSource(s string) bool {
	switch s {
	case "pii_detection", "content_safety":
		return true
	default:
		return false
	}
}

// isValidSeverity checks if severity is valid
func isValidSeverity(s string) bool {
	switch strings.ToLower(s) {
	case "low", "medium", "high", "critical":
		return true
	default:
		return false
	}
}

// validateChannelConfig validates channel-specific configuration
func validateChannelConfig(ch ChannelConfig) error {
	switch ch.Type {
	case "webhook":
		return validateWebhookConfig(ch.Config)
	case "email":
		return validateEmailConfig(ch.Config)
	case "slack":
		return validateSlackConfig(ch.Config)
	}
	return nil
}

// validateWebhookConfig validates webhook channel configuration
func validateWebhookConfig(config map[string]interface{}) error {
	url, ok := config["url"].(string)
	if !ok || url == "" {
		return fmt.Errorf("url is required for webhook channel")
	}
	return nil
}

// validateEmailConfig validates email channel configuration
func validateEmailConfig(config map[string]interface{}) error {
	if _, ok := config["smtp_host"].(string); !ok {
		return fmt.Errorf("smtp_host is required for email channel")
	}
	to, ok := config["to"].([]interface{})
	if !ok || len(to) == 0 {
		return fmt.Errorf("to (recipients) is required for email channel")
	}
	return nil
}

// validateSlackConfig validates Slack channel configuration
func validateSlackConfig(config map[string]interface{}) error {
	url, ok := config["webhook_url"].(string)
	if !ok || url == "" {
		return fmt.Errorf("webhook_url is required for slack channel")
	}
	return nil
}

// SetDefaults sets default values for alerting config
func (c *AlertingConfig) SetDefaults() {
	if c.RateLimit.MaxAlertsPerMinute == 0 {
		c.RateLimit.MaxAlertsPerMinute = 10
	}
	if c.RateLimit.BurstSize == 0 {
		c.RateLimit.BurstSize = 20
	}
	if c.Retry.MaxRetries == 0 {
		c.Retry.MaxRetries = 3
	}
	if c.Retry.RetryDelaySeconds == 0 {
		c.Retry.RetryDelaySeconds = 5
	}
	if c.Storage.Path == "" {
		c.Storage.Path = "data/alerts.jsonl"
	}
	if c.Storage.MaxFileBytes == 0 {
		c.Storage.MaxFileBytes = 100 * 1024 * 1024 // 100MB
	}
}