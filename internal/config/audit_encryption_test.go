package config_test

import (
	"testing"

	"open-ai-gateway/internal/config"
)

func TestLoadConfigAcceptsAuditEncryptionConfig(t *testing.T) {
	path := writeConfig(t, `{
		"addr": "127.0.0.1:8080",
		"api_key": "gateway-key",
		"audit": {
			"enabled": true,
			"path": "audit/custom-agent-trace.jsonl",
			"encryption": {
				"enabled": true,
				"algorithm": "aes-256-gcm",
				"key_env": "CUSTOM_AUDIT_KEY"
			}
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

	// 验证审计配置
	if !cfg.Audit.Enabled {
		t.Fatal("audit should be enabled")
	}

	// 验证加密配置
	if !cfg.Audit.Encryption.Enabled {
		t.Fatal("audit encryption should be enabled")
	}
	if cfg.Audit.Encryption.Algorithm != "aes-256-gcm" {
		t.Fatalf("encryption algorithm = %q, want aes-256-gcm", cfg.Audit.Encryption.Algorithm)
	}
	if cfg.Audit.Encryption.KeyEnv != "CUSTOM_AUDIT_KEY" {
		t.Fatalf("encryption key_env = %q, want CUSTOM_AUDIT_KEY", cfg.Audit.Encryption.KeyEnv)
	}
}

func TestLoadConfigDefaultAuditEncryptionConfig(t *testing.T) {
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

	// 验证默认配置
	if cfg.Audit.Encryption.Enabled {
		t.Fatal("audit encryption should be disabled by default")
	}
	if cfg.Audit.Encryption.Algorithm != "aes-256-gcm" {
		t.Fatalf("default encryption algorithm = %q, want aes-256-gcm", cfg.Audit.Encryption.Algorithm)
	}
	if cfg.Audit.Encryption.KeyEnv != "AUDIT_ENCRYPTION_KEY" {
		t.Fatalf("default encryption key_env = %q, want AUDIT_ENCRYPTION_KEY", cfg.Audit.Encryption.KeyEnv)
	}
}

func TestEnvironmentAuditEncryptionEnabled(t *testing.T) {
	t.Setenv("GATEWAY_AUDIT_ENCRYPTION_ENABLED", "true")
	t.Setenv("GATEWAY_AUDIT_ENCRYPTION_KEY", "12345678901234567890123456789012") // 32字节

	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("Load default: %v", err)
	}

	if !cfg.Audit.Encryption.Enabled {
		t.Fatal("audit encryption should be enabled")
	}
	if cfg.Audit.Encryption.KeyEnv != "GATEWAY_AUDIT_ENCRYPTION_KEY" {
		t.Fatalf("encryption key_env = %q, want GATEWAY_AUDIT_ENCRYPTION_KEY", cfg.Audit.Encryption.KeyEnv)
	}
}

func TestEnvironmentAuditEncryptionInvalidKey(t *testing.T) {
	t.Setenv("GATEWAY_AUDIT_ENCRYPTION_KEY", "too-short") // 不是32字节

	_, err := config.Load("")
	if err == nil {
		t.Fatal("expected validation error for invalid key length")
	}
}

func TestEnvironmentAuditEncryptionEnabledRejectsInvalidValue(t *testing.T) {
	t.Setenv("GATEWAY_AUDIT_ENCRYPTION_ENABLED", "definitely")

	_, err := config.Load("")
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestAuditEncryptionRequiresValidAlgorithm(t *testing.T) {
	path := writeConfig(t, `{
		"addr": "127.0.0.1:8080",
		"api_key": "gateway-key",
		"audit": {
			"enabled": true,
			"encryption": {
				"enabled": true,
				"algorithm": "invalid-algorithm",
				"key_env": "AUDIT_KEY"
			}
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
		t.Fatal("expected validation error for invalid algorithm")
	}
}

func TestAuditEncryptionRequiresKeyEnvWhenEnabled(t *testing.T) {
	path := writeConfig(t, `{
		"addr": "127.0.0.1:8080",
		"api_key": "gateway-key",
		"audit": {
			"enabled": true,
			"encryption": {
				"enabled": true,
				"algorithm": "aes-256-gcm",
				"key_env": ""
			}
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
		t.Fatal("expected validation error for empty key_env")
	}
}

func TestCheckReportIncludesAuditEncryption(t *testing.T) {
	path := writeConfig(t, `{
		"addr": "127.0.0.1:8080",
		"api_key": "gateway-key",
		"audit": {
			"enabled": true,
			"encryption": {
				"enabled": true,
				"algorithm": "aes-256-gcm",
				"key_env": "AUDIT_KEY"
			}
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

	report := cfg.CheckReport()

	// 验证CheckReport包含加密信息
	if !report.AuditEncryptionEnabled {
		t.Fatal("CheckReport should show audit encryption enabled")
	}
	if report.AuditEncryptionAlgorithm != "aes-256-gcm" {
		t.Fatalf("CheckReport algorithm = %q, want aes-256-gcm", report.AuditEncryptionAlgorithm)
	}
}