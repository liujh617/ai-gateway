package config

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"sort"
	"strings"
	"time"

	"open-ai-gateway/internal/upstreamurl"
)

type Config struct {
	Addr                     string                    `json:"addr"`
	APIKey                   string                    `json:"api_key"`
	RequestTimeoutSeconds    int                       `json:"request_timeout_seconds"`
	StreamTimeoutSeconds     int                       `json:"stream_timeout_seconds"`
	ReadHeaderTimeoutSeconds int                       `json:"read_header_timeout_seconds"`
	ReadTimeoutSeconds       int                       `json:"read_timeout_seconds"`
	WriteTimeoutSeconds      int                       `json:"write_timeout_seconds"`
	IdleTimeoutSeconds       int                       `json:"idle_timeout_seconds"`
	ShutdownTimeoutSeconds   int                       `json:"shutdown_timeout_seconds"`
	MaxRequestBodyBytes      int64                     `json:"max_request_body_bytes"`
	Log                      LogConfig                 `json:"log"`
	RateLimit                RateLimitConfig           `json:"rate_limit"`
	Providers                map[string]ProviderConfig `json:"providers"`
	Models                   map[string]ModelConfig    `json:"models"`
}

type ProviderConfig struct {
	Type           string `json:"type"`
	BaseURL        string `json:"base_url"`
	APIKey         string `json:"api_key"`
	APIKeyEnv      string `json:"api_key_env"`
	TimeoutSeconds int    `json:"timeout_seconds"`
}

type ModelConfig struct {
	Provider      string   `json:"provider"`
	UpstreamModel string   `json:"upstream_model"`
	Capabilities  []string `json:"capabilities"`
}

type RateLimitConfig struct {
	RequestsPerMinute int `json:"requests_per_minute"`
}

type LogConfig struct {
	Format string `json:"format"`
	Level  string `json:"level"`
}

type CheckReport struct {
	ProviderCount int
	ModelCount    int
	Providers     []ProviderSummary
	Models        []ModelSummary
	Warnings      []string
}

type ProviderSummary struct {
	Name         string
	Type         string
	BaseURL      string
	APIKeySet    bool
	APIKeyEnv    string
	APIKeyEnvSet bool
}

type ModelSummary struct {
	Name          string
	Provider      string
	UpstreamModel string
	Capabilities  []string
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

func Check(path string) (*Config, CheckReport, error) {
	cfg, err := Load(path)
	if err != nil {
		return nil, CheckReport{}, err
	}
	report := cfg.CheckReport()
	return cfg, report, nil
}

func (c *Config) CheckReport() CheckReport {
	report := CheckReport{
		ProviderCount: len(c.Providers),
		ModelCount:    len(c.Models),
	}

	for _, name := range c.ProviderNames() {
		provider := c.Providers[name]
		summary := ProviderSummary{
			Name:      name,
			Type:      provider.Type,
			BaseURL:   provider.BaseURL,
			APIKeySet: provider.APIKey != "",
			APIKeyEnv: provider.APIKeyEnv,
		}
		if provider.APIKeyEnv != "" {
			_, summary.APIKeyEnvSet = os.LookupEnv(provider.APIKeyEnv)
			if !summary.APIKeyEnvSet {
				report.Warnings = append(report.Warnings, fmt.Sprintf("provider %q api_key_env %q is not set", name, provider.APIKeyEnv))
			}
		}
		report.Providers = append(report.Providers, summary)
	}

	for _, name := range c.ModelNames() {
		model := c.Models[name]
		upstreamModel := model.UpstreamModel
		if upstreamModel == "" {
			upstreamModel = name
		}
		report.Models = append(report.Models, ModelSummary{
			Name:          name,
			Provider:      model.Provider,
			UpstreamModel: upstreamModel,
			Capabilities:  append([]string(nil), model.Capabilities...),
		})
	}
	return report
}

func Default() *Config {
	cfg := &Config{
		Addr:                     "127.0.0.1:8080",
		APIKey:                   "test-gateway-key",
		RequestTimeoutSeconds:    60,
		StreamTimeoutSeconds:     600,
		ReadHeaderTimeoutSeconds: 10,
		ReadTimeoutSeconds:       0,
		WriteTimeoutSeconds:      0,
		IdleTimeoutSeconds:       120,
		ShutdownTimeoutSeconds:   10,
		MaxRequestBodyBytes:      1 << 20,
		Log: LogConfig{
			Format: "text",
			Level:  "info",
		},
		Providers: map[string]ProviderConfig{
			"fake": {
				Type: "fake",
			},
		},
		Models: map[string]ModelConfig{
			"test-model": {
				Provider:      "fake",
				UpstreamModel: "test-model",
				Capabilities:  []string{"chat", "embeddings"},
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
	if c.ReadHeaderTimeoutSeconds < 0 {
		return fmt.Errorf("read_header_timeout_seconds must be non-negative")
	}
	if c.ReadTimeoutSeconds < 0 {
		return fmt.Errorf("read_timeout_seconds must be non-negative")
	}
	if c.WriteTimeoutSeconds < 0 {
		return fmt.Errorf("write_timeout_seconds must be non-negative")
	}
	if c.IdleTimeoutSeconds < 0 {
		return fmt.Errorf("idle_timeout_seconds must be non-negative")
	}
	if c.ShutdownTimeoutSeconds < 0 {
		return fmt.Errorf("shutdown_timeout_seconds must be non-negative")
	}
	if c.MaxRequestBodyBytes < 0 {
		return fmt.Errorf("max_request_body_bytes must be non-negative")
	}
	if c.RateLimit.RequestsPerMinute < 0 {
		return fmt.Errorf("rate_limit.requests_per_minute must be non-negative")
	}
	switch c.Log.Format {
	case "text", "json":
	default:
		return fmt.Errorf("log.format must be text or json")
	}
	switch c.Log.Level {
	case "debug", "info", "warn", "error":
	default:
		return fmt.Errorf("log.level must be debug, info, warn, or error")
	}

	for name, provider := range c.Providers {
		if strings.TrimSpace(name) == "" {
			return fmt.Errorf("provider name is required")
		}
		switch provider.Type {
		case "fake":
		case "openai-compatible":
			if _, err := upstreamurl.NormalizeHTTPBaseURL(provider.BaseURL); err != nil {
				return fmt.Errorf("provider %q %w", name, err)
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
		for _, capability := range model.Capabilities {
			switch capability {
			case "chat", "embeddings":
			default:
				return fmt.Errorf("model %q has unsupported capability %q", externalModel, capability)
			}
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
	if c.ReadHeaderTimeoutSeconds == 0 {
		c.ReadHeaderTimeoutSeconds = 10
	}
	if c.IdleTimeoutSeconds == 0 {
		c.IdleTimeoutSeconds = 120
	}
	if c.ShutdownTimeoutSeconds == 0 {
		c.ShutdownTimeoutSeconds = 10
	}
	if c.MaxRequestBodyBytes == 0 {
		c.MaxRequestBodyBytes = 1 << 20
	}
	if c.Log.Format == "" {
		c.Log.Format = "text"
	}
	if c.Log.Level == "" {
		c.Log.Level = "info"
	}
	for name, provider := range c.Providers {
		if provider.TimeoutSeconds == 0 {
			provider.TimeoutSeconds = 60
		}
		c.Providers[name] = provider
	}
	for name, model := range c.Models {
		if len(model.Capabilities) == 0 {
			model.Capabilities = []string{"chat", "embeddings"}
		}
		c.Models[name] = model
	}
}

func (c *Config) ReadHeaderTimeout() time.Duration {
	return durationFromSeconds(c.ReadHeaderTimeoutSeconds)
}

func (c *Config) ReadTimeout() time.Duration {
	return durationFromSeconds(c.ReadTimeoutSeconds)
}

func (c *Config) WriteTimeout() time.Duration {
	return durationFromSeconds(c.WriteTimeoutSeconds)
}

func (c *Config) IdleTimeout() time.Duration {
	return durationFromSeconds(c.IdleTimeoutSeconds)
}

func (c *Config) ShutdownTimeout() time.Duration {
	if c.ShutdownTimeoutSeconds <= 0 {
		return 10 * time.Second
	}
	return time.Duration(c.ShutdownTimeoutSeconds) * time.Second
}

func durationFromSeconds(seconds int) time.Duration {
	if seconds <= 0 {
		return 0
	}
	return time.Duration(seconds) * time.Second
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
