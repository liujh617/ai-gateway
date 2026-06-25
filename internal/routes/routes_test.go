package routes

import (
	"net/http"
	"testing"
)

func TestNormalizePath(t *testing.T) {
	if got := NormalizePath(ChatCompletionsPath); got != ChatCompletionsPath {
		t.Fatalf("known path = %q", got)
	}
	if got := NormalizePath("/v1/unknown"); got != UnknownPathLabel {
		t.Fatalf("unknown path = %q", got)
	}
}

func TestPattern(t *testing.T) {
	if got := Pattern(http.MethodPost, ChatCompletionsPath); got != "POST /v1/chat/completions" {
		t.Fatalf("pattern = %q", got)
	}
}

func TestAllowedMethods(t *testing.T) {
	methods, ok := AllowedMethods(ChatCompletionsPath)
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

func TestMethodAllowed(t *testing.T) {
	allowed, known := MethodAllowed(ChatCompletionsPath, http.MethodPost)
	if !known {
		t.Fatal("known path was not found")
	}
	if !allowed {
		t.Fatal("POST should be allowed for chat completions")
	}

	allowed, known = MethodAllowed(ChatCompletionsPath, http.MethodGet)
	if !known {
		t.Fatal("known path was not found")
	}
	if allowed {
		t.Fatal("GET should not be allowed for chat completions")
	}

	allowed, known = MethodAllowed("/v1/unknown", http.MethodGet)
	if known {
		t.Fatal("unknown path should not be known")
	}
	if allowed {
		t.Fatal("unknown path should not allow a method")
	}
}

func TestAllowHeader(t *testing.T) {
	allow, ok := AllowHeader(HealthzPath)
	if !ok {
		t.Fatal("known path was not found")
	}
	if allow != "GET, HEAD" {
		t.Fatalf("allow = %q", allow)
	}

	if _, ok := AllowHeader("/v1/unknown"); ok {
		t.Fatal("unknown path had an Allow header")
	}
}

func TestAllowedMethodsReturnsCopy(t *testing.T) {
	methods, ok := AllowedMethods(HealthzPath)
	if !ok {
		t.Fatal("known path was not found")
	}
	methods[0] = http.MethodPost

	again, ok := AllowedMethods(HealthzPath)
	if !ok {
		t.Fatal("known path was not found on second lookup")
	}
	if again[0] != http.MethodGet {
		t.Fatalf("methods were mutated: %v", again)
	}
}
