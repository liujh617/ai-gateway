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
		"api_key": "gateway-key",
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
				"capabilities": ["chat"]
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
	if len(report.Providers) != 1 || !report.Providers[0].APIKeyEnvSet {
		t.Fatalf("provider summary = %#v", report.Providers)
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
