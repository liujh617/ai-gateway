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
	} {
		t.Run(path, func(t *testing.T) {
			if _, err := config.Load(path); err != nil {
				t.Fatalf("Load(%s): %v", path, err)
			}
		})
	}
}

func TestConfigSchemaIsValidJSON(t *testing.T) {
	path := filepath.Join("..", "..", "schema", "config.schema.json")
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", path, err)
	}
	if !json.Valid(payload) {
		t.Fatalf("%s is not valid JSON", path)
	}
}
