package alert

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// SlackChannel sends alerts via Slack Incoming Webhook
type SlackChannel struct {
	name       string
	webhookURL string
	channel    string
	username   string
	iconEmoji  string
	client     *http.Client
}

// SlackConfig configures Slack channel
type SlackConfig struct {
	WebhookURL string `json:"webhook_url"`
	Channel    string `json:"channel"`
	Username   string `json:"username"`
	IconEmoji  string `json:"icon_emoji"`
}

// NewSlackChannel creates a new Slack channel
func NewSlackChannel(name string, config SlackConfig) (*SlackChannel, error) {
	if config.WebhookURL == "" {
		return nil, fmt.Errorf("webhook URL is required")
	}

	username := config.Username
	if username == "" {
		username = "AI Gateway Alert"
	}

	iconEmoji := config.IconEmoji
	if iconEmoji == "" {
		iconEmoji = ":warning:"
	}

	return &SlackChannel{
		name:       name,
		webhookURL: config.WebhookURL,
		channel:    config.Channel,
		username:   username,
		iconEmoji:  iconEmoji,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}, nil
}

// Name returns the channel name
func (c *SlackChannel) Name() string {
	return c.name
}

// Send sends an alert to Slack
func (c *SlackChannel) Send(ctx context.Context, alert *Alert) error {
	payload := c.buildSlackMessage(alert)

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.webhookURL, bytes.NewReader(jsonPayload))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("slack returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// buildSlackMessage builds Slack message payload
func (c *SlackChannel) buildSlackMessage(alert *Alert) map[string]interface{} {
	// Map severity to color
	color := "warning"
	switch alert.Severity {
	case SeverityHigh, SeverityCritical:
		color = "danger"
	case SeverityLow:
		color = "good"
	}

	// Build fields
	fields := []map[string]interface{}{
		{"title": "Source", "value": alert.Source, "short": true},
		{"title": "Severity", "value": strings.ToUpper(alert.Severity), "short": true},
	}

	if alert.RequestID != "" {
		fields = append(fields, map[string]interface{}{
			"title": "Request ID", "value": alert.RequestID, "short": true,
		})
	}
	if alert.TraceID != "" {
		fields = append(fields, map[string]interface{}{
			"title": "Trace ID", "value": alert.TraceID, "short": true,
		})
	}

	// Build attachment
	attachment := map[string]interface{}{
		"color":  color,
		"title":  fmt.Sprintf("[%s] %s", strings.ToUpper(alert.Severity), alert.Title),
		"text":   alert.Description,
		"fields": fields,
		"footer": "AI Gateway Alert",
		"ts":     alert.Timestamp.Unix(),
	}

	// Build message
	message := map[string]interface{}{
		"username":   c.username,
		"icon_emoji": c.iconEmoji,
		"attachments": []map[string]interface{}{attachment},
	}

	if c.channel != "" {
		message["channel"] = c.channel
	}

	return message
}

// Close closes the Slack channel
func (c *SlackChannel) Close() error {
	c.client.CloseIdleConnections()
	return nil
}