package config_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"open-ai-gateway/internal/config"
)

func TestLoadDefaultConfig(t *testing.T) {
	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("Load default: %v", err)
	}
	if cfg.Addr != "127.0.0.1:8080" {
		t.Fatalf("addr = %q", cfg.Addr)
	}
	if _, ok := cfg.Providers["fake"]; !ok {
		t.Fatal("missing fake provider")
	}
	if _, ok := cfg.Models["test-model"]; !ok {
		t.Fatal("missing test-model")
	}
	if cfg.ReadHeaderTimeoutSeconds != 10 {
		t.Fatalf("read header timeout = %d", cfg.ReadHeaderTimeoutSeconds)
	}
	if cfg.IdleTimeoutSeconds != 120 {
		t.Fatalf("idle timeout = %d", cfg.IdleTimeoutSeconds)
	}
	if cfg.ShutdownTimeoutSeconds != 10 {
		t.Fatalf("shutdown timeout = %d", cfg.ShutdownTimeoutSeconds)
	}
	if cfg.MaxRequestBodyBytes != 1<<20 {
		t.Fatalf("max request body bytes = %d", cfg.MaxRequestBodyBytes)
	}
	if cfg.Log.Format != "text" || cfg.Log.Level != "info" {
		t.Fatalf("log config = %#v", cfg.Log)
	}
	if cfg.Audit.Enabled {
		t.Fatal("audit should be disabled by default")
	}
	if cfg.Audit.Path != "audit/agent-trace.jsonl" {
		t.Fatalf("audit path = %q", cfg.Audit.Path)
	}
	if cfg.Audit.MaxFileBytes != 0 {
		t.Fatalf("audit max file bytes = %d", cfg.Audit.MaxFileBytes)
	}
}

func TestLoadConfigAcceptsAuditConfig(t *testing.T) {
	path := writeConfig(t, `{
		"addr": "127.0.0.1:8080",
		"api_key": "gateway-key",
		"audit": {
			"enabled": true,
			"path": "audit/custom-agent-trace.jsonl",
			"max_file_bytes": 1048576
		},
		"providers": {
			"fake": {
				"type": "fake"
			}
		},
		"models": {
			"test-model": {
				"provider": "fake"
			}
		}
	}`)

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.Audit.Enabled {
		t.Fatal("audit should be enabled")
	}
	if cfg.Audit.Path != "audit/custom-agent-trace.jsonl" {
		t.Fatalf("audit path = %q", cfg.Audit.Path)
	}
	if cfg.Audit.MaxFileBytes != 1048576 {
		t.Fatalf("audit max file bytes = %d", cfg.Audit.MaxFileBytes)
	}
}

func TestLoadConfigRejectsTrailingJSON(t *testing.T) {
	path := writeConfig(t, `{
		"addr": "127.0.0.1:8080",
		"api_key": "gateway-key",
		"providers": {
			"fake": {
				"type": "fake"
			}
		},
		"models": {
			"test-model": {
				"provider": "fake"
			}
		}
	}{}`)

	_, err := config.Load(path)
	if err == nil {
		t.Fatal("expected trailing JSON error")
	}
	if !strings.Contains(err.Error(), "single JSON value") {
		t.Fatalf("error = %v", err)
	}
}

func TestLoadConfigValidatesUnknownProvider(t *testing.T) {
	path := writeConfig(t, `{
		"addr": "127.0.0.1:8080",
		"api_key": "gateway-key",
		"providers": {
			"openai": {
				"type": "openai-compatible",
				"base_url": "https://api.openai.com/v1",
				"api_key": "upstream-key"
			}
		},
		"models": {
			"gpt-4o-mini": {
				"provider": "missing",
				"upstream_model": "gpt-4o-mini"
			}
		}
	}`)

	_, err := config.Load(path)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "unknown provider") {
		t.Fatalf("error = %v", err)
	}
}

func TestLoadConfigRequiresOpenAICompatibleKeyReference(t *testing.T) {
	path := writeConfig(t, `{
		"addr": "127.0.0.1:8080",
		"api_key": "gateway-key",
		"providers": {
			"openai": {
				"type": "openai-compatible",
				"base_url": "https://api.openai.com/v1"
			}
		},
		"models": {
			"gpt-4o-mini": {
				"provider": "openai",
				"upstream_model": "gpt-4o-mini"
			}
		}
	}`)

	_, err := config.Load(path)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "requires api_key or api_key_env") {
		t.Fatalf("error = %v", err)
	}
}

func TestLoadConfigRejectsNonHTTPProviderBaseURL(t *testing.T) {
	path := writeConfig(t, `{
		"addr": "127.0.0.1:8080",
		"api_key": "gateway-key",
		"providers": {
			"openai": {
				"type": "openai-compatible",
				"base_url": "ftp://api.example.com/v1",
				"api_key": "upstream-key"
			}
		},
		"models": {
			"gpt-4o-mini": {
				"provider": "openai",
				"upstream_model": "gpt-4o-mini"
			}
		}
	}`)

	_, err := config.Load(path)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "http or https") {
		t.Fatalf("error = %v", err)
	}
}

func TestLoadConfigRejectsProviderBaseURLWithQueryOrFragment(t *testing.T) {
	for _, baseURL := range []string{
		"https://api.example.com/v1?tenant=one",
		"https://api.example.com/v1#models",
	} {
		t.Run(baseURL, func(t *testing.T) {
			path := writeConfig(t, `{
				"addr": "127.0.0.1:8080",
				"api_key": "gateway-key",
				"providers": {
					"openai": {
						"type": "openai-compatible",
						"base_url": `+quoteJSON(t, baseURL)+`,
						"api_key": "upstream-key"
					}
				},
				"models": {
					"gpt-4o-mini": {
						"provider": "openai",
						"upstream_model": "gpt-4o-mini"
					}
				}
			}`)

			_, err := config.Load(path)
			if err == nil {
				t.Fatal("expected validation error")
			}
			if !strings.Contains(err.Error(), "query or fragment") {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestLoadConfigValidatesCapabilities(t *testing.T) {
	path := writeConfig(t, `{
		"addr": "127.0.0.1:8080",
		"api_key": "gateway-key",
		"providers": {
			"fake": {
				"type": "fake"
			}
		},
		"models": {
			"bad-model": {
				"provider": "fake",
				"capabilities": ["images"]
			}
		}
	}`)

	_, err := config.Load(path)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "unsupported capability") {
		t.Fatalf("error = %v", err)
	}
}

func TestLoadConfigDefaultsCapabilities(t *testing.T) {
	path := writeConfig(t, `{
		"addr": "127.0.0.1:8080",
		"api_key": "gateway-key",
		"providers": {
			"fake": {
				"type": "fake"
			}
		},
		"models": {
			"test-model": {
				"provider": "fake"
			}
		}
	}`)

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	got := cfg.Models["test-model"].Capabilities
	if len(got) != 2 || got[0] != "chat" || got[1] != "embeddings" {
		t.Fatalf("capabilities = %#v", got)
	}
}

func TestLoadConfigAcceptsGatewayAPIKeys(t *testing.T) {
	path := writeConfig(t, `{
		"addr": "127.0.0.1:8080",
		"api_keys": ["client-key-1", "client-key-2"],
		"providers": {
			"fake": {
				"type": "fake"
			}
		},
		"models": {
			"test-model": {
				"provider": "fake"
			}
		}
	}`)

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	keys := cfg.GatewayAPIKeys()
	if len(keys) != 2 || keys[0] != "client-key-1" || keys[1] != "client-key-2" {
		t.Fatalf("gateway keys = %#v", keys)
	}
}

func TestLoadConfigAcceptsGatewayAPIClients(t *testing.T) {
	path := writeConfig(t, `{
		"addr": "127.0.0.1:8080",
		"api_clients": [
			{"name": "alpha", "api_key": "client-key-1", "models": ["test-model"], "rate_limit": {"requests_per_minute": 30}},
			{"name": "beta", "api_key": "client-key-2"}
		],
		"providers": {
			"fake": {
				"type": "fake"
			}
		},
		"models": {
			"test-model": {
				"provider": "fake"
			}
		}
	}`)

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	clients := cfg.GatewayAPIClients()
	if len(clients) != 2 || clients[0].Name != "alpha" || clients[1].APIKey != "client-key-2" {
		t.Fatalf("gateway clients = %#v", clients)
	}
	if clients[0].RateLimit.RequestsPerMinute == nil || *clients[0].RateLimit.RequestsPerMinute != 30 {
		t.Fatalf("alpha rate limit = %#v", clients[0].RateLimit.RequestsPerMinute)
	}
	if len(clients[0].Models) != 1 || clients[0].Models[0] != "test-model" {
		t.Fatalf("alpha models = %#v", clients[0].Models)
	}
	if clients[1].RateLimit.RequestsPerMinute != nil {
		t.Fatalf("beta rate limit = %#v", clients[1].RateLimit.RequestsPerMinute)
	}
	keys := cfg.GatewayAPIKeys()
	if len(keys) != 2 || keys[0] != "client-key-1" || keys[1] != "client-key-2" {
		t.Fatalf("gateway keys = %#v", keys)
	}
}

func TestLoadConfigRejectsInvalidGatewayAPIKey(t *testing.T) {
	for name, rawKey := range map[string]string{
		"empty": `"   "`,
		"space": `" gateway-key "`,
	} {
		t.Run(name, func(t *testing.T) {
			path := writeConfig(t, `{
				"addr": "127.0.0.1:8080",
				"api_key": `+rawKey+`,
				"providers": {
					"fake": {
						"type": "fake"
					}
				},
				"models": {
					"test-model": {
						"provider": "fake"
					}
				}
			}`)

			_, err := config.Load(path)
			if err == nil {
				t.Fatal("expected validation error")
			}
			if !strings.Contains(err.Error(), "api_key") {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestLoadConfigRejectsInvalidGatewayAPIKeys(t *testing.T) {
	for name, rawKeys := range map[string]string{
		"empty":     `["client-key", ""]`,
		"duplicate": `["client-key", "client-key"]`,
		"space":     `[" client-key "]`,
	} {
		t.Run(name, func(t *testing.T) {
			path := writeConfig(t, `{
				"addr": "127.0.0.1:8080",
				"api_keys": `+rawKeys+`,
				"providers": {
					"fake": {
						"type": "fake"
					}
				},
				"models": {
					"test-model": {
						"provider": "fake"
					}
				}
			}`)

			_, err := config.Load(path)
			if err == nil {
				t.Fatal("expected validation error")
			}
			if !strings.Contains(err.Error(), "api_keys") {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestLoadConfigRejectsInvalidGatewayAPIClients(t *testing.T) {
	for name, rawClients := range map[string]string{
		"empty-name":      `[{"name":"","api_key":"client-key"}]`,
		"empty-key":       `[{"name":"alpha","api_key":""}]`,
		"duplicate-name":  `[{"name":"alpha","api_key":"one"},{"name":"alpha","api_key":"two"}]`,
		"duplicate-key":   `[{"name":"alpha","api_key":"same"},{"name":"beta","api_key":"same"}]`,
		"space-name":      `[{"name":" alpha ","api_key":"client-key"}]`,
		"space-key":       `[{"name":"alpha","api_key":" client-key "}]`,
		"negative-limit":  `[{"name":"alpha","api_key":"client-key","rate_limit":{"requests_per_minute":-1}}]`,
		"empty-model":     `[{"name":"alpha","api_key":"client-key","models":[""]}]`,
		"space-model":     `[{"name":"alpha","api_key":"client-key","models":[" test-model "]}]`,
		"unknown-model":   `[{"name":"alpha","api_key":"client-key","models":["missing-model"]}]`,
		"duplicate-model": `[{"name":"alpha","api_key":"client-key","models":["test-model","test-model"]}]`,
	} {
		t.Run(name, func(t *testing.T) {
			path := writeConfig(t, `{
				"addr": "127.0.0.1:8080",
				"api_clients": `+rawClients+`,
				"providers": {
					"fake": {
						"type": "fake"
					}
				},
				"models": {
					"test-model": {
						"provider": "fake"
					}
				}
			}`)

			_, err := config.Load(path)
			if err == nil {
				t.Fatal("expected validation error")
			}
			if !strings.Contains(err.Error(), "api_clients") {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestLoadConfigRejectsProviderAndModelNamesWithWhitespace(t *testing.T) {
	for name, body := range map[string]string{
		"provider-name": `"providers": {" fake ": {"type": "fake"}}, "models": {"test-model": {"provider": " fake "}}`,
		"model-name":    `"providers": {"fake": {"type": "fake"}}, "models": {" test-model ": {"provider": "fake"}}`,
	} {
		t.Run(name, func(t *testing.T) {
			path := writeConfig(t, `{
				"addr": "127.0.0.1:8080",
				"api_key": "gateway-key",
				`+body+`
			}`)

			_, err := config.Load(path)
			if err == nil {
				t.Fatal("expected validation error")
			}
			if !strings.Contains(err.Error(), "whitespace") {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestLoadConfigRejectsProviderReferencesWithWhitespace(t *testing.T) {
	for name, body := range map[string]string{
		"model-provider":    `"models": {"test-model": {"provider": " fake "}}`,
		"fallback-provider": `"models": {"test-model": {"provider": "fake", "fallbacks": [{"provider": " fake "}]}}`,
	} {
		t.Run(name, func(t *testing.T) {
			path := writeConfig(t, `{
				"addr": "127.0.0.1:8080",
				"api_key": "gateway-key",
				"providers": {
					"fake": {
						"type": "fake"
					}
				},
				`+body+`
			}`)

			_, err := config.Load(path)
			if err == nil {
				t.Fatal("expected validation error")
			}
			if !strings.Contains(err.Error(), "whitespace") {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestLoadConfigAcceptsModelFallbacks(t *testing.T) {
	path := writeConfig(t, `{
		"addr": "127.0.0.1:8080",
		"api_key": "gateway-key",
		"providers": {
			"primary": {
				"type": "fake"
			},
			"backup": {
				"type": "fake"
			}
		},
		"models": {
			"test-model": {
				"provider": "primary",
				"upstream_model": "primary-model",
				"pricing": {
					"prompt_usd_per_1m_tokens": 0.1,
					"completion_usd_per_1m_tokens": 0.2
				},
				"fallbacks": [
					{
						"provider": "backup",
						"upstream_model": "backup-model",
						"pricing": {
							"prompt_usd_per_1m_tokens": 0.3,
							"completion_usd_per_1m_tokens": 0.4
						}
					}
				]
			}
		}
	}`)

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	fallbacks := cfg.Models["test-model"].Fallbacks
	if len(fallbacks) != 1 || fallbacks[0].Provider != "backup" || fallbacks[0].UpstreamModel != "backup-model" {
		t.Fatalf("fallbacks = %#v", fallbacks)
	}
	if cfg.Models["test-model"].Pricing.PromptUSDPer1MTokens != 0.1 {
		t.Fatalf("pricing = %#v", cfg.Models["test-model"].Pricing)
	}
	if fallbacks[0].Pricing.CompletionUSDPer1MTokens != 0.4 {
		t.Fatalf("fallback pricing = %#v", fallbacks[0].Pricing)
	}
}

func TestLoadConfigValidatesModelFallbackProvider(t *testing.T) {
	path := writeConfig(t, `{
		"addr": "127.0.0.1:8080",
		"api_key": "gateway-key",
		"providers": {
			"primary": {
				"type": "fake"
			}
		},
		"models": {
			"test-model": {
				"provider": "primary",
				"fallbacks": [
					{
						"provider": "missing"
					}
				]
			}
		}
	}`)

	_, err := config.Load(path)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "fallback") || !strings.Contains(err.Error(), "unknown provider") {
		t.Fatalf("error = %v", err)
	}
}

func TestLoadConfigValidatesPricing(t *testing.T) {
	for name, body := range map[string]string{
		"model":    `"pricing":{"prompt_usd_per_1m_tokens":-0.1}`,
		"fallback": `"fallbacks":[{"provider":"backup","pricing":{"completion_usd_per_1m_tokens":-0.1}}]`,
	} {
		t.Run(name, func(t *testing.T) {
			path := writeConfig(t, `{
				"addr": "127.0.0.1:8080",
				"api_key": "gateway-key",
				"providers": {
					"primary": {
						"type": "fake"
					},
					"backup": {
						"type": "fake"
					}
				},
				"models": {
					"test-model": {
						"provider": "primary",
						`+body+`
					}
				}
			}`)

			_, err := config.Load(path)
			if err == nil {
				t.Fatal("expected validation error")
			}
			if !strings.Contains(err.Error(), "pricing") {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestEnvironmentOverrides(t *testing.T) {
	t.Setenv("GATEWAY_ADDR", "127.0.0.1:9090")
	t.Setenv("GATEWAY_API_KEY", "override-key")

	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("Load default: %v", err)
	}
	if cfg.Addr != "127.0.0.1:9090" {
		t.Fatalf("addr = %q", cfg.Addr)
	}
	if cfg.APIKey != "override-key" {
		t.Fatalf("api key = %q", cfg.APIKey)
	}
}

func TestEnvironmentAPIKeysOverride(t *testing.T) {
	t.Setenv("GATEWAY_API_KEYS", "env-key-1, env-key-2")

	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("Load default: %v", err)
	}
	keys := cfg.GatewayAPIKeys()
	if len(keys) != 2 || keys[0] != "env-key-1" || keys[1] != "env-key-2" {
		t.Fatalf("gateway keys = %#v", keys)
	}
}

func TestEnvironmentAPIKeysRejectsEmptyEntry(t *testing.T) {
	t.Setenv("GATEWAY_API_KEYS", "env-key-1,,env-key-2")

	_, err := config.Load("")
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "api_keys") {
		t.Fatalf("error = %v", err)
	}
}

func TestEnvironmentAuditOverrides(t *testing.T) {
	t.Setenv("GATEWAY_AUDIT_ENABLED", "true")
	t.Setenv("GATEWAY_AUDIT_PATH", "audit/env-agent-trace.jsonl")
	t.Setenv("GATEWAY_AUDIT_MAX_FILE_BYTES", "2048")

	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("Load default: %v", err)
	}
	if !cfg.Audit.Enabled {
		t.Fatal("audit should be enabled")
	}
	if cfg.Audit.Path != "audit/env-agent-trace.jsonl" {
		t.Fatalf("audit path = %q", cfg.Audit.Path)
	}
	if cfg.Audit.MaxFileBytes != 2048 {
		t.Fatalf("audit max file bytes = %d", cfg.Audit.MaxFileBytes)
	}
}

func TestEnvironmentAuditEnabledRejectsInvalidValue(t *testing.T) {
	t.Setenv("GATEWAY_AUDIT_ENABLED", "definitely")

	_, err := config.Load("")
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "GATEWAY_AUDIT_ENABLED") {
		t.Fatalf("error = %v", err)
	}
}

func TestEnvironmentAuditMaxFileBytesRejectsInvalidValue(t *testing.T) {
	t.Setenv("GATEWAY_AUDIT_MAX_FILE_BYTES", "nope")

	_, err := config.Load("")
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "GATEWAY_AUDIT_MAX_FILE_BYTES") {
		t.Fatalf("error = %v", err)
	}
}

func TestLoadConfigValidatesServerTimeouts(t *testing.T) {
	path := writeConfig(t, `{
		"addr": "127.0.0.1:8080",
		"api_key": "gateway-key",
		"read_header_timeout_seconds": -1,
		"providers": {
			"fake": {
				"type": "fake"
			}
		},
		"models": {
			"test-model": {
				"provider": "fake"
			}
		}
	}`)

	_, err := config.Load(path)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "read_header_timeout_seconds") {
		t.Fatalf("error = %v", err)
	}
}

func TestLoadConfigValidatesMaxRequestBodyBytes(t *testing.T) {
	path := writeConfig(t, `{
		"addr": "127.0.0.1:8080",
		"api_key": "gateway-key",
		"max_request_body_bytes": -1,
		"providers": {
			"fake": {
				"type": "fake"
			}
		},
		"models": {
			"test-model": {
				"provider": "fake"
			}
		}
	}`)

	_, err := config.Load(path)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "max_request_body_bytes") {
		t.Fatalf("error = %v", err)
	}
}

func TestLoadConfigValidatesLogConfig(t *testing.T) {
	path := writeConfig(t, `{
		"addr": "127.0.0.1:8080",
		"api_key": "gateway-key",
		"log": {
			"format": "xml",
			"level": "info"
		},
		"providers": {
			"fake": {
				"type": "fake"
			}
		},
		"models": {
			"test-model": {
				"provider": "fake"
			}
		}
	}`)

	_, err := config.Load(path)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "log.format") {
		t.Fatalf("error = %v", err)
	}
}

func TestLoadConfigAcceptsJSONDebugLogConfig(t *testing.T) {
	path := writeConfig(t, `{
		"addr": "127.0.0.1:8080",
		"api_key": "gateway-key",
		"log": {
			"format": "json",
			"level": "debug"
		},
		"providers": {
			"fake": {
				"type": "fake"
			}
		},
		"models": {
			"test-model": {
				"provider": "fake"
			}
		}
	}`)

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Log.Format != "json" || cfg.Log.Level != "debug" {
		t.Fatalf("log config = %#v", cfg.Log)
	}
}

func TestCheckReportDoesNotExposeAPIKey(t *testing.T) {
	t.Setenv("UPSTREAM_KEY", "secret-value")
	path := writeConfig(t, `{
		"addr": "127.0.0.1:8080",
		"api_clients": [
			{"name":"alpha","api_key":"gateway-key-one","models":["gpt-4o-mini"],"rate_limit":{"requests_per_minute":60}},
			{"name":"beta","api_key":"gateway-key-two"}
		],
		"rate_limit": {
			"requests_per_minute": 120
		},
		"providers": {
			"openai": {
				"type": "openai-compatible",
				"base_url": "https://api.openai.com/v1",
				"api_key_env": "UPSTREAM_KEY"
			}
		},
		"models": {
			"gpt-4o-mini": {
				"provider": "openai",
				"capabilities": ["chat"],
				"fallbacks": [
					{"provider": "openai"}
				]
			}
		}
	}`)

	_, report, err := config.Check(path)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	payload, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal report: %v", err)
	}
	text := string(payload)
	if strings.Contains(text, "secret-value") || strings.Contains(text, "gateway-key") {
		t.Fatalf("report leaked secret: %s", text)
	}
	for _, want := range []string{
		`"addr":"127.0.0.1:8080"`,
		`"gateway_api_key_count":2`,
		`"gateway_clients"`,
		`"request_timeout_seconds":60`,
		`"stream_timeout_seconds":600`,
		`"read_header_timeout_seconds":10`,
		`"read_timeout_seconds":0`,
		`"write_timeout_seconds":0`,
		`"idle_timeout_seconds":120`,
		`"shutdown_timeout_seconds":10`,
		`"max_request_body_bytes":1048576`,
		`"log_format":"text"`,
		`"log_level":"info"`,
		`"audit_enabled":false`,
		`"audit_path":"audit/agent-trace.jsonl"`,
		`"audit_max_file_bytes":0`,
		`"rate_limit_requests_per_minute":120`,
		`"rate_limit_requests_per_minute":60`,
		`"provider_health_failure_threshold":2`,
		`"provider_health_cooldown_seconds":30`,
		`"provider_count":1`,
		`"timeout_seconds":60`,
		`"api_key_env_set":true`,
		`"upstream_model":"gpt-4o-mini"`,
		`"fallbacks":[{"provider":"openai","upstream_model":"gpt-4o-mini"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("report missing %s: %s", want, text)
		}
	}
	for _, unwanted := range []string{
		"Addr",
		"GatewayAPIKeyCount",
		"GatewayClients",
		"RequestTimeoutSeconds",
		"StreamTimeoutSeconds",
		"ReadHeaderTimeoutSeconds",
		"ReadTimeoutSeconds",
		"WriteTimeoutSeconds",
		"IdleTimeoutSeconds",
		"ShutdownTimeoutSeconds",
		"MaxRequestBodyBytes",
		"LogFormat",
		"LogLevel",
		"AuditEnabled",
		"AuditPath",
		"AuditMaxFileBytes",
		"RateLimitRequestsPerMinute",
		"ProviderHealthFailureThreshold",
		"ProviderHealthCooldownSeconds",
		"ProviderCount",
		"TimeoutSeconds",
		"APIKeyEnvSet",
		"UpstreamModel",
	} {
		if strings.Contains(text, unwanted) {
			t.Fatalf("report used unstable field %q: %s", unwanted, text)
		}
	}
	if report.GatewayAPIKeyCount != 2 {
		t.Fatalf("gateway api key count = %d", report.GatewayAPIKeyCount)
	}
	if report.Addr != "127.0.0.1:8080" {
		t.Fatalf("addr summary = %q", report.Addr)
	}
	if len(report.GatewayClients) != 2 {
		t.Fatalf("gateway clients = %#v", report.GatewayClients)
	}
	if report.RequestTimeoutSeconds != 60 || report.StreamTimeoutSeconds != 600 || report.ReadHeaderTimeoutSeconds != 10 {
		t.Fatalf("timeout summary = request %d stream %d read header %d", report.RequestTimeoutSeconds, report.StreamTimeoutSeconds, report.ReadHeaderTimeoutSeconds)
	}
	if report.ReadTimeoutSeconds != 0 || report.WriteTimeoutSeconds != 0 {
		t.Fatalf("io timeout summary = read %d write %d", report.ReadTimeoutSeconds, report.WriteTimeoutSeconds)
	}
	if report.IdleTimeoutSeconds != 120 || report.ShutdownTimeoutSeconds != 10 || report.MaxRequestBodyBytes != 1<<20 {
		t.Fatalf("server summary = idle %d shutdown %d max body %d", report.IdleTimeoutSeconds, report.ShutdownTimeoutSeconds, report.MaxRequestBodyBytes)
	}
	if report.LogFormat != "text" || report.LogLevel != "info" {
		t.Fatalf("log summary = format %q level %q", report.LogFormat, report.LogLevel)
	}
	if report.AuditEnabled || report.AuditPath != "audit/agent-trace.jsonl" || report.AuditMaxFileBytes != 0 {
		t.Fatalf("audit summary = enabled %t path %q max file bytes %d", report.AuditEnabled, report.AuditPath, report.AuditMaxFileBytes)
	}
	if report.RateLimitRequestsPerMinute != 120 {
		t.Fatalf("rate limit summary = %d", report.RateLimitRequestsPerMinute)
	}
	if report.ProviderHealthFailureThreshold != 2 || report.ProviderHealthCooldownSeconds != 30 {
		t.Fatalf("provider health summary = threshold %d cooldown %d", report.ProviderHealthFailureThreshold, report.ProviderHealthCooldownSeconds)
	}
	if report.GatewayClients[0].Name != "alpha" || len(report.GatewayClients[0].Models) != 1 || report.GatewayClients[0].Models[0] != "gpt-4o-mini" {
		t.Fatalf("alpha summary = %#v", report.GatewayClients[0])
	}
	if report.GatewayClients[0].RateLimitRequestsPerMinute == nil || *report.GatewayClients[0].RateLimitRequestsPerMinute != 60 {
		t.Fatalf("alpha rate limit summary = %#v", report.GatewayClients[0].RateLimitRequestsPerMinute)
	}
	if report.GatewayClients[1].Name != "beta" || len(report.GatewayClients[1].Models) != 0 || report.GatewayClients[1].RateLimitRequestsPerMinute != nil {
		t.Fatalf("beta summary = %#v", report.GatewayClients[1])
	}
	if len(report.Providers) != 1 || !report.Providers[0].APIKeyEnvSet || report.Providers[0].TimeoutSeconds != 60 {
		t.Fatalf("provider summary = %#v", report.Providers)
	}
	if len(report.Models) != 1 || len(report.Models[0].Fallbacks) != 1 || report.Models[0].Fallbacks[0].UpstreamModel != "gpt-4o-mini" {
		t.Fatalf("model fallback summary = %#v", report.Models)
	}
}

func TestCheckReportWarnsMissingAPIKeyEnv(t *testing.T) {
	path := writeConfig(t, `{
		"addr": "127.0.0.1:8080",
		"api_key": "gateway-key",
		"providers": {
			"openai": {
				"type": "openai-compatible",
				"base_url": "https://api.openai.com/v1",
				"api_key_env": "MISSING_UPSTREAM_KEY"
			}
		},
		"models": {
			"gpt-4o-mini": {
				"provider": "openai",
				"capabilities": ["chat"]
			}
		}
	}`)

	_, report, err := config.Check(path)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(report.Warnings) != 1 || !strings.Contains(report.Warnings[0], "MISSING_UPSTREAM_KEY") {
		t.Fatalf("warnings = %#v", report.Warnings)
	}
}

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func quoteJSON(t *testing.T, value string) string {
	t.Helper()
	payload, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal string: %v", err)
	}
	return string(payload)
}
