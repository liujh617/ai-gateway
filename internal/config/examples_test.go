package config_test

import (
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
