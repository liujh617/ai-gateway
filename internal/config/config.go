package config

import (
	"encoding/json"
	"fmt"
	"io"
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
	APIKeys                  []string                  `json:"api_keys"`
	APIClients               []GatewayClientConfig     `json:"api_clients"`
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
	ProviderHealth           ProviderHealthConfig      `json:"provider_health"`
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

type GatewayClientConfig struct {
	Name      string                `json:"name"`
	APIKey    string                `json:"api_key"`
	Models    []string              `json:"models"`
	RateLimit ClientRateLimitConfig `json:"rate_limit"`
}

type ClientRateLimitConfig struct {
	RequestsPerMinute *int `json:"requests_per_minute,omitempty"`
}

type ModelConfig struct {
	Provider      string                `json:"provider"`
	UpstreamModel string                `json:"upstream_model"`
	Capabilities  []string              `json:"capabilities"`
	Pricing       PricingConfig         `json:"pricing"`
	Fallbacks     []ModelFallbackConfig `json:"fallbacks"`
}

type ModelFallbackConfig struct {
	Provider      string        `json:"provider"`
	UpstreamModel string        `json:"upstream_model"`
	Pricing       PricingConfig `json:"pricing"`
}

type PricingConfig struct {
	PromptUSDPer1MTokens     float64 `json:"prompt_usd_per_1m_tokens"`
	CompletionUSDPer1MTokens float64 `json:"completion_usd_per_1m_tokens"`
}

type RateLimitConfig struct {
	RequestsPerMinute int `json:"requests_per_minute"`
}

type ProviderHealthConfig struct {
	FailureThreshold int `json:"failure_threshold"`
	CooldownSeconds  int `json:"cooldown_seconds"`
}

type LogConfig struct {
	Format string `json:"format"`
	Level  string `json:"level"`
}

type CheckReport struct {
	Addr                           string                 `json:"addr"`
	GatewayAPIKeyCount             int                    `json:"gateway_api_key_count"`
	GatewayClients                 []GatewayClientSummary `json:"gateway_clients"`
	RequestTimeoutSeconds          int                    `json:"request_timeout_seconds"`
	StreamTimeoutSeconds           int                    `json:"stream_timeout_seconds"`
	ReadHeaderTimeoutSeconds       int                    `json:"read_header_timeout_seconds"`
	ReadTimeoutSeconds             int                    `json:"read_timeout_seconds"`
	WriteTimeoutSeconds            int                    `json:"write_timeout_seconds"`
	IdleTimeoutSeconds             int                    `json:"idle_timeout_seconds"`
	ShutdownTimeoutSeconds         int                    `json:"shutdown_timeout_seconds"`
	MaxRequestBodyBytes            int64                  `json:"max_request_body_bytes"`
	LogFormat                      string                 `json:"log_format"`
	LogLevel                       string                 `json:"log_level"`
	RateLimitRequestsPerMinute     int                    `json:"rate_limit_requests_per_minute"`
	ProviderHealthFailureThreshold int                    `json:"provider_health_failure_threshold"`
	ProviderHealthCooldownSeconds  int                    `json:"provider_health_cooldown_seconds"`
	ProviderCount                  int                    `json:"provider_count"`
	ModelCount                     int                    `json:"model_count"`
	Providers                      []ProviderSummary      `json:"providers"`
	Models                         []ModelSummary         `json:"models"`
	Warnings                       []string               `json:"warnings"`
}

type GatewayClientSummary struct {
	Name                       string   `json:"name"`
	Models                     []string `json:"models"`
	RateLimitRequestsPerMinute *int     `json:"rate_limit_requests_per_minute"`
}

type ProviderSummary struct {
	Name           string `json:"name"`
	Type           string `json:"type"`
	BaseURL        string `json:"base_url"`
	TimeoutSeconds int    `json:"timeout_seconds"`
	APIKeySet      bool   `json:"api_key_set"`
	APIKeyEnv      string `json:"api_key_env"`
	APIKeyEnvSet   bool   `json:"api_key_env_set"`
}

type ModelSummary struct {
	Name          string                 `json:"name"`
	Provider      string                 `json:"provider"`
	UpstreamModel string                 `json:"upstream_model"`
	Capabilities  []string               `json:"capabilities"`
	Pricing       PricingConfig          `json:"pricing"`
	Fallbacks     []ModelFallbackSummary `json:"fallbacks"`
}

type ModelFallbackSummary struct {
	Provider      string        `json:"provider"`
	UpstreamModel string        `json:"upstream_model"`
	Pricing       PricingConfig `json:"pricing"`
}

func Load(path string) (*Config, error) {
	if path == "" {
		cfg := Default()
		if err := cfg.Validate(); err != nil {
			return nil, err
		}
		return cfg, nil
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
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("decode config: config must contain a single JSON value")
		}
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
		Addr:                           c.Addr,
		GatewayAPIKeyCount:             len(c.GatewayAPIKeys()),
		RequestTimeoutSeconds:          c.RequestTimeoutSeconds,
		StreamTimeoutSeconds:           c.StreamTimeoutSeconds,
		ReadHeaderTimeoutSeconds:       c.ReadHeaderTimeoutSeconds,
		ReadTimeoutSeconds:             c.ReadTimeoutSeconds,
		WriteTimeoutSeconds:            c.WriteTimeoutSeconds,
		IdleTimeoutSeconds:             c.IdleTimeoutSeconds,
		ShutdownTimeoutSeconds:         c.ShutdownTimeoutSeconds,
		MaxRequestBodyBytes:            c.MaxRequestBodyBytes,
		LogFormat:                      c.Log.Format,
		LogLevel:                       c.Log.Level,
		RateLimitRequestsPerMinute:     c.RateLimit.RequestsPerMinute,
		ProviderHealthFailureThreshold: c.ProviderHealth.FailureThreshold,
		ProviderHealthCooldownSeconds:  c.ProviderHealth.CooldownSeconds,
		ProviderCount:                  len(c.Providers),
		ModelCount:                     len(c.Models),
	}

	for _, client := range c.GatewayAPIClients() {
		report.GatewayClients = append(report.GatewayClients, GatewayClientSummary{
			Name:                       client.Name,
			Models:                     append([]string(nil), client.Models...),
			RateLimitRequestsPerMinute: copyIntPointer(client.RateLimit.RequestsPerMinute),
		})
	}

	for _, name := range c.ProviderNames() {
		provider := c.Providers[name]
		summary := ProviderSummary{
			Name:           name,
			Type:           provider.Type,
			BaseURL:        provider.BaseURL,
			TimeoutSeconds: provider.TimeoutSeconds,
			APIKeySet:      provider.APIKey != "",
			APIKeyEnv:      provider.APIKeyEnv,
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
			Pricing:       model.Pricing,
			Fallbacks:     fallbackSummaries(name, model.Fallbacks),
		})
	}
	return report
}

func fallbackSummaries(externalModel string, fallbacks []ModelFallbackConfig) []ModelFallbackSummary {
	if len(fallbacks) == 0 {
		return nil
	}
	out := make([]ModelFallbackSummary, 0, len(fallbacks))
	for _, fallback := range fallbacks {
		upstreamModel := fallback.UpstreamModel
		if upstreamModel == "" {
			upstreamModel = externalModel
		}
		out = append(out, ModelFallbackSummary{
			Provider:      fallback.Provider,
			UpstreamModel: upstreamModel,
			Pricing:       fallback.Pricing,
		})
	}
	return out
}

func copyIntPointer(value *int) *int {
	if value == nil {
		return nil
	}
	copied := *value
	return &copied
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
	if err := validateGatewayAPIKey(c.APIKey); err != nil {
		return err
	}
	if err := validateGatewayAPIKeys(c.APIKeys); err != nil {
		return err
	}
	if err := validateGatewayAPIClients(c.APIClients); err != nil {
		return err
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
	if c.ProviderHealth.FailureThreshold < 0 {
		return fmt.Errorf("provider_health.failure_threshold must be non-negative")
	}
	if c.ProviderHealth.CooldownSeconds < 0 {
		return fmt.Errorf("provider_health.cooldown_seconds must be non-negative")
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
		if name != strings.TrimSpace(name) {
			return fmt.Errorf("provider name must not contain leading or trailing whitespace")
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
		if externalModel != strings.TrimSpace(externalModel) {
			return fmt.Errorf("model name must not contain leading or trailing whitespace")
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
		if err := validatePricing(model.Pricing); err != nil {
			return fmt.Errorf("model %q pricing %w", externalModel, err)
		}
		for index, fallback := range model.Fallbacks {
			if strings.TrimSpace(fallback.Provider) == "" {
				return fmt.Errorf("model %q fallback %d provider is required", externalModel, index)
			}
			if _, ok := c.Providers[fallback.Provider]; !ok {
				return fmt.Errorf("model %q fallback %d references unknown provider %q", externalModel, index, fallback.Provider)
			}
			if err := validatePricing(fallback.Pricing); err != nil {
				return fmt.Errorf("model %q fallback %d pricing %w", externalModel, index, err)
			}
		}
	}
	if err := c.validateGatewayClientModels(); err != nil {
		return err
	}
	return nil
}

func validatePricing(pricing PricingConfig) error {
	if pricing.PromptUSDPer1MTokens < 0 {
		return fmt.Errorf("prompt_usd_per_1m_tokens must be non-negative")
	}
	if pricing.CompletionUSDPer1MTokens < 0 {
		return fmt.Errorf("completion_usd_per_1m_tokens must be non-negative")
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
		c.APIKeys = nil
		c.APIClients = nil
	}
	if env := os.Getenv("GATEWAY_API_KEYS"); env != "" {
		c.APIKey = ""
		c.APIKeys = splitAPIKeys(env)
		c.APIClients = nil
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
	if c.ProviderHealth.FailureThreshold == 0 {
		c.ProviderHealth.FailureThreshold = 2
	}
	if c.ProviderHealth.CooldownSeconds == 0 {
		c.ProviderHealth.CooldownSeconds = 30
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

func (c *Config) GatewayAPIKeys() []string {
	clients := c.GatewayAPIClients()
	if len(clients) > 0 {
		keys := make([]string, 0, len(clients))
		for _, client := range clients {
			keys = append(keys, client.APIKey)
		}
		return keys
	}
	return nil
}

func (c *Config) GatewayAPIClients() []GatewayClientConfig {
	if len(c.APIClients) > 0 {
		return append([]GatewayClientConfig(nil), c.APIClients...)
	}
	if len(c.APIKeys) > 0 {
		clients := make([]GatewayClientConfig, 0, len(c.APIKeys))
		for index, key := range c.APIKeys {
			clients = append(clients, GatewayClientConfig{
				Name:   fmt.Sprintf("key_%d", index+1),
				APIKey: key,
			})
		}
		return clients
	}
	if c.APIKey == "" {
		return nil
	}
	return []GatewayClientConfig{{
		Name:   "default",
		APIKey: c.APIKey,
	}}
}

func validateGatewayAPIKey(key string) error {
	if key == "" {
		return nil
	}
	if strings.TrimSpace(key) == "" {
		return fmt.Errorf("api_key must be non-empty")
	}
	if key != strings.TrimSpace(key) {
		return fmt.Errorf("api_key must not contain leading or trailing whitespace")
	}
	return nil
}

func validateGatewayAPIKeys(keys []string) error {
	seen := make(map[string]struct{}, len(keys))
	for index, key := range keys {
		if strings.TrimSpace(key) == "" {
			return fmt.Errorf("api_keys[%d] must be non-empty", index)
		}
		if key != strings.TrimSpace(key) {
			return fmt.Errorf("api_keys[%d] must not contain leading or trailing whitespace", index)
		}
		if _, ok := seen[key]; ok {
			return fmt.Errorf("api_keys[%d] duplicates another gateway API key", index)
		}
		seen[key] = struct{}{}
	}
	return nil
}

func validateGatewayAPIClients(clients []GatewayClientConfig) error {
	seenNames := make(map[string]struct{}, len(clients))
	seenKeys := make(map[string]struct{}, len(clients))
	for index, client := range clients {
		if strings.TrimSpace(client.Name) == "" {
			return fmt.Errorf("api_clients[%d].name must be non-empty", index)
		}
		if client.Name != strings.TrimSpace(client.Name) {
			return fmt.Errorf("api_clients[%d].name must not contain leading or trailing whitespace", index)
		}
		if strings.TrimSpace(client.APIKey) == "" {
			return fmt.Errorf("api_clients[%d].api_key must be non-empty", index)
		}
		if client.APIKey != strings.TrimSpace(client.APIKey) {
			return fmt.Errorf("api_clients[%d].api_key must not contain leading or trailing whitespace", index)
		}
		if client.RateLimit.RequestsPerMinute != nil && *client.RateLimit.RequestsPerMinute < 0 {
			return fmt.Errorf("api_clients[%d].rate_limit.requests_per_minute must be non-negative", index)
		}
		for modelIndex, model := range client.Models {
			if strings.TrimSpace(model) == "" {
				return fmt.Errorf("api_clients[%d].models[%d] must be non-empty", index, modelIndex)
			}
			if model != strings.TrimSpace(model) {
				return fmt.Errorf("api_clients[%d].models[%d] must not contain leading or trailing whitespace", index, modelIndex)
			}
		}
		if _, ok := seenNames[client.Name]; ok {
			return fmt.Errorf("api_clients[%d].name duplicates another gateway client", index)
		}
		if _, ok := seenKeys[client.APIKey]; ok {
			return fmt.Errorf("api_clients[%d].api_key duplicates another gateway API key", index)
		}
		seenNames[client.Name] = struct{}{}
		seenKeys[client.APIKey] = struct{}{}
	}
	return nil
}

func (c *Config) validateGatewayClientModels() error {
	for clientIndex, client := range c.APIClients {
		seen := make(map[string]struct{}, len(client.Models))
		for modelIndex, model := range client.Models {
			if _, ok := c.Models[model]; !ok {
				return fmt.Errorf("api_clients[%d].models[%d] references unknown model %q", clientIndex, modelIndex, model)
			}
			if _, ok := seen[model]; ok {
				return fmt.Errorf("api_clients[%d].models[%d] duplicates another model", clientIndex, modelIndex)
			}
			seen[model] = struct{}{}
		}
	}
	return nil
}

func splitAPIKeys(value string) []string {
	parts := strings.Split(value, ",")
	keys := make([]string, 0, len(parts))
	for _, part := range parts {
		keys = append(keys, strings.TrimSpace(part))
	}
	return keys
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

func (c *Config) ProviderHealthCooldown() time.Duration {
	if c.ProviderHealth.CooldownSeconds <= 0 {
		return 30 * time.Second
	}
	return time.Duration(c.ProviderHealth.CooldownSeconds) * time.Second
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
