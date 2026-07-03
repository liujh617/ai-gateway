package config_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"open-ai-gateway/internal/config"
)

func TestExampleConfigsLoad(t *testing.T) {
	for _, path := range []string{
		filepath.Join("..", "..", "config.example.json"),
		filepath.Join("..", "..", "config.local.example.json"),
		filepath.Join("..", "..", "config.deepseek.example.json"),
	} {
		t.Run(path, func(t *testing.T) {
			if _, err := config.Load(path); err != nil {
				t.Fatalf("Load(%s): %v", path, err)
			}
		})
	}
}

func TestConfigSchemaIsValidJSON(t *testing.T) {
	schema := loadConfigSchema(t)
	if schema["$schema"] == "" {
		t.Fatal("schema is missing $schema")
	}
}

func TestConfigSchemaKeepsCoreContract(t *testing.T) {
	schema := loadConfigSchema(t)
	if schema["additionalProperties"] != false {
		t.Fatalf("additionalProperties = %#v, want false", schema["additionalProperties"])
	}
	required, ok := schema["required"].([]any)
	if !ok {
		t.Fatalf("required = %#v", schema["required"])
	}
	assertContainsString(t, required, "providers")
	assertContainsString(t, required, "models")

	defs, ok := schema["$defs"].(map[string]any)
	if !ok {
		t.Fatalf("$defs = %#v", schema["$defs"])
	}
	for _, name := range []string{"api_client", "rate_limit", "provider_health", "provider", "model", "model_fallback", "pricing"} {
		if _, ok := defs[name]; !ok {
			t.Fatalf("missing schema def %q", name)
		}
	}
}

func loadConfigSchema(t *testing.T) map[string]any {
	t.Helper()
	path := filepath.Join("..", "..", "schema", "config.schema.json")
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", path, err)
	}
	if !json.Valid(payload) {
		t.Fatalf("%s is not valid JSON", path)
	}
	var schema map[string]any
	if err := json.Unmarshal(payload, &schema); err != nil {
		t.Fatalf("Unmarshal(%s): %v", path, err)
	}
	return schema
}

func assertContainsString(t *testing.T, values []any, want string) {
	t.Helper()
	for _, value := range values {
		if value == want {
			return
		}
	}
	t.Fatalf("%q not found in %#v", want, values)
}
