package config

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

type Config struct {
	Addr      string                    `json:"addr"`
	APIKey    string                    `json:"api_key"`
	Providers map[string]ProviderConfig `json:"providers"`
	Models    map[string]ModelConfig    `json:"models"`
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
	return &cfg, nil
}

func Default() *Config {
	cfg := &Config{
		Addr:   "127.0.0.1:8080",
		APIKey: "test-gateway-key",
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
	for name, provider := range c.Providers {
		if provider.TimeoutSeconds == 0 {
			provider.TimeoutSeconds = 60
		}
		c.Providers[name] = provider
	}
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
