package config

import (
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"
)

type Config struct {
	Addr                  string                    `json:"addr"`
	APIKey                string                    `json:"api_key"`
	RequestTimeoutSeconds int                       `json:"request_timeout_seconds"`
	StreamTimeoutSeconds  int                       `json:"stream_timeout_seconds"`
	RateLimit             RateLimitConfig           `json:"rate_limit"`
	Providers             map[string]ProviderConfig `json:"providers"`
	Models                map[string]ModelConfig    `json:"models"`
}

type ProviderConfig struct {
	Type           string `json:"type"`
	BaseURL        string `json:"base_url"`
	APIKey         string `json:"api_key"`
	APIKeyEnv      string `json:"api_key_env"`
	TimeoutSeconds int    `json:"timeout_seconds"`
}

type ModelConfig struct {
	Provider      string `json:"provider"`
	UpstreamModel string `json:"upstream_model"`
}

type RateLimitConfig struct {
	RequestsPerMinute int `json:"requests_per_minute"`
}

func Load(path string) (*Config, error) {
	if path == "" {
		return Default(), nil
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open config: %w", err)
	}
	defer file.Close()

	var cfg Config
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("decode config: %w", err)
	}
	cfg.applyDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func Default() *Config {
	cfg := &Config{
		Addr:                  "127.0.0.1:8080",
		APIKey:                "test-gateway-key",
		RequestTimeoutSeconds: 60,
		StreamTimeoutSeconds:  600,
		Providers: map[string]ProviderConfig{
			"fake": {
				Type: "fake",
			},
		},
		Models: map[string]ModelConfig{
			"test-model": {
				Provider:      "fake",
				UpstreamModel: "test-model",
			},
		},
	}
	cfg.applyDefaults()
	return cfg
}

func (c *Config) Validate() error {
	if c == nil {
		return fmt.Errorf("config is nil")
	}
	if strings.TrimSpace(c.Addr) == "" {
		return fmt.Errorf("addr is required")
	}
	if _, _, err := net.SplitHostPort(c.Addr); err != nil {
		return fmt.Errorf("addr must be host:port: %w", err)
	}
	if len(c.Providers) == 0 {
		return fmt.Errorf("at least one provider is required")
	}
	if len(c.Models) == 0 {
		return fmt.Errorf("at least one model is required")
	}
	if c.RequestTimeoutSeconds < 0 {
		return fmt.Errorf("request_timeout_seconds must be non-negative")
	}
	if c.StreamTimeoutSeconds < 0 {
		return fmt.Errorf("stream_timeout_seconds must be non-negative")
	}
	if c.RateLimit.RequestsPerMinute < 0 {
		return fmt.Errorf("rate_limit.requests_per_minute must be non-negative")
	}

	for name, provider := range c.Providers {
		if strings.TrimSpace(name) == "" {
			return fmt.Errorf("provider name is required")
		}
		switch provider.Type {
		case "fake":
		case "openai-compatible":
			if strings.TrimSpace(provider.BaseURL) == "" {
				return fmt.Errorf("provider %q base_url is required", name)
			}
			if _, err := url.ParseRequestURI(provider.BaseURL); err != nil {
				return fmt.Errorf("provider %q base_url is invalid: %w", name, err)
			}
			if provider.APIKey == "" && provider.APIKeyEnv == "" {
				return fmt.Errorf("provider %q requires api_key or api_key_env", name)
			}
		default:
			return fmt.Errorf("provider %q has unsupported type %q", name, provider.Type)
		}
		if provider.TimeoutSeconds < 0 {
			return fmt.Errorf("provider %q timeout_seconds must be non-negative", name)
		}
	}

	for externalModel, model := range c.Models {
		if strings.TrimSpace(externalModel) == "" {
			return fmt.Errorf("model name is required")
		}
		if strings.TrimSpace(model.Provider) == "" {
			return fmt.Errorf("model %q provider is required", externalModel)
		}
		if _, ok := c.Providers[model.Provider]; !ok {
			return fmt.Errorf("model %q references unknown provider %q", externalModel, model.Provider)
		}
	}
	return nil
}

func (c *Config) ProviderNames() []string {
	names := make([]string, 0, len(c.Providers))
	for name := range c.Providers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (c *Config) ModelNames() []string {
	names := make([]string, 0, len(c.Models))
	for name := range c.Models {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (c *Config) applyDefaults() {
	if c.Addr == "" {
		c.Addr = "127.0.0.1:8080"
	}
	if c.APIKey == "" {
		c.APIKey = "test-gateway-key"
	}
	if env := os.Getenv("GATEWAY_ADDR"); env != "" {
		c.Addr = env
	}
	if env := os.Getenv("GATEWAY_API_KEY"); env != "" {
		c.APIKey = env
	}
	if c.RequestTimeoutSeconds == 0 {
		c.RequestTimeoutSeconds = 60
	}
	if c.StreamTimeoutSeconds == 0 {
		c.StreamTimeoutSeconds = 600
	}
	for name, provider := range c.Providers {
		if provider.TimeoutSeconds == 0 {
			provider.TimeoutSeconds = 60
		}
		c.Providers[name] = provider
	}
}

func (c *Config) RequestTimeout() time.Duration {
	if c.RequestTimeoutSeconds <= 0 {
		return 60 * time.Second
	}
	return time.Duration(c.RequestTimeoutSeconds) * time.Second
}

func (c *Config) StreamTimeout() time.Duration {
	if c.StreamTimeoutSeconds <= 0 {
		return 10 * time.Minute
	}
	return time.Duration(c.StreamTimeoutSeconds) * time.Second
}

func (p ProviderConfig) ResolvedAPIKey() string {
	if p.APIKey != "" {
		return p.APIKey
	}
	if p.APIKeyEnv != "" {
		return os.Getenv(p.APIKeyEnv)
	}
	return ""
}

func (p ProviderConfig) Timeout() time.Duration {
	if p.TimeoutSeconds <= 0 {
		return 60 * time.Second
	}
	return time.Duration(p.TimeoutSeconds) * time.Second
}
