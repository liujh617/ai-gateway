package routes

import (
	"net/http"
	"testing"
)

func TestNormalizePath(t *testing.T) {
	if got := NormalizePath("/v1/chat/completions"); got != "/v1/chat/completions" {
		t.Fatalf("known path = %q", got)
	}
	if got := NormalizePath("/v1/unknown"); got != UnknownPathLabel {
		t.Fatalf("unknown path = %q", got)
	}
}

func TestAllowedMethods(t *testing.T) {
	methods, ok := AllowedMethods("/v1/chat/completions")
	if !ok {
		t.Fatal("known path was not found")
	}
	if len(methods) != 1 || methods[0] != http.MethodPost {
		t.Fatalf("methods = %v", methods)
	}

	if _, ok := AllowedMethods("/v1/unknown"); ok {
		t.Fatal("unknown path had allowed methods")
	}
}

func TestAllowedMethodsReturnsCopy(t *testing.T) {
	methods, ok := AllowedMethods("/healthz")
	if !ok {
		t.Fatal("known path was not found")
	}
	methods[0] = http.MethodPost

	again, ok := AllowedMethods("/healthz")
	if !ok {
		t.Fatal("known path was not found on second lookup")
	}
	if again[0] != http.MethodGet {
		t.Fatalf("methods were mutated: %v", again)
	}
}
