package alert

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// WebhookChannel sends alerts via HTTP webhook
type WebhookChannel struct {
	name          string
	url           string
	method        string
	headers       map[string]string
	timeoutSeconds int
	client        *http.Client
}

// WebhookConfig configures webhook channel
type WebhookConfig struct {
	URL            string            `json:"url"`
	Method         string            `json:"method"`
	Headers        map[string]string `json:"headers"`
	TimeoutSeconds int               `json:"timeout_seconds"`
}

// NewWebhookChannel creates a new webhook channel
func NewWebhookChannel(name string, config WebhookConfig) (*WebhookChannel, error) {
	if config.URL == "" {
		return nil, fmt.Errorf("webhook URL is required")
	}

	method := config.Method
	if method == "" {
		method = "POST"
	}

	timeout := config.TimeoutSeconds
	if timeout == 0 {
		timeout = 10
	}

	return &WebhookChannel{
		name:          name,
		url:           config.URL,
		method:        method,
		headers:       config.Headers,
		timeoutSeconds: timeout,
		client: &http.Client{
			Timeout: time.Duration(timeout) * time.Second,
		},
	}, nil
}

// Name returns the channel name
func (c *WebhookChannel) Name() string {
	return c.name
}

// Send sends an alert to the webhook endpoint
func (c *WebhookChannel) Send(ctx context.Context, alert *Alert) error {
	// Serialize alert to JSON
	payload, err := json.Marshal(alert)
	if err != nil {
		return fmt.Errorf("marshal alert: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, c.method, c.url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	for key, value := range c.headers {
		req.Header.Set(key, value)
	}

	// Send request
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("webhook returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// Close closes the webhook channel
func (c *WebhookChannel) Close() error {
	c.client.CloseIdleConnections()
	return nil
}