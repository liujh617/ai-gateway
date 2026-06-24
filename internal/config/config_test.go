package config_test

import (
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

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}
