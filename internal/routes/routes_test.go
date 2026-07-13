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

func TestResponsesRoute(t *testing.T) {
	if got := NormalizePath(ResponsesPath); got != ResponsesPath {
		t.Fatalf("NormalizePath = %q", got)
	}
	if allowed, known := MethodAllowed(ResponsesPath, http.MethodPost); !known || !allowed {
		t.Fatalf("POST known=%v allowed=%v", known, allowed)
	}
	if allowed, known := MethodAllowed(ResponsesPath, http.MethodGet); !known || allowed {
		t.Fatalf("GET known=%v allowed=%v", known, allowed)
	}
}

func TestRetrieveModelRoute(t *testing.T) {
	path := "/v1/models/test-model"
	if got := NormalizePath(path); got != ModelsRetrievePath {
		t.Fatalf("NormalizePath = %q", got)
	}
	if allowed, known := MethodAllowed(path, http.MethodGet); !known || !allowed {
		t.Fatalf("GET known=%v allowed=%v", known, allowed)
	}
	if allowed, known := MethodAllowed(path, http.MethodHead); !known || !allowed {
		t.Fatalf("HEAD known=%v allowed=%v", known, allowed)
	}
	if allowed, known := MethodAllowed(path, http.MethodPost); !known || allowed {
		t.Fatalf("POST known=%v allowed=%v", known, allowed)
	}
	if allow, ok := AllowHeader(path); !ok || allow != "GET, HEAD" {
		t.Fatalf("AllowHeader = %q, %v", allow, ok)
	}
	if IsPublicPath(path) {
		t.Fatal("retrieve model route must require authentication")
	}
}

func TestRetrieveModelRouteRejectsInvalidShapes(t *testing.T) {
	for _, path := range []string{"/v1/models/", "/v1/models/a/b"} {
		if got := NormalizePath(path); got != UnknownPathLabel {
			t.Fatalf("NormalizePath(%q) = %q", path, got)
		}
		if _, known := AllowedMethods(path); known {
			t.Fatalf("AllowedMethods(%q) marked path known", path)
		}
	}
}

func TestPattern(t *testing.T) {
	if got := Pattern(http.MethodPost, ChatCompletionsPath); got != "POST /v1/chat/completions" {
		t.Fatalf("pattern = %q", got)
	}
}

func TestAllReturnsCopy(t *testing.T) {
	all := All()
	if len(all) == 0 {
		t.Fatal("routes are empty")
	}
	all[0].Path = "/mutated"
	all[0].Methods[0] = http.MethodPost
	all[0].Public = false

	again := All()
	if again[0].Path == "/mutated" {
		t.Fatalf("route path was mutated: %v", again[0])
	}
	if again[0].Methods[0] != http.MethodGet {
		t.Fatalf("route methods were mutated: %v", again[0].Methods)
	}
	if !again[0].Public {
		t.Fatalf("route public flag was mutated: %v", again[0])
	}
}

func TestRegistrationPattern(t *testing.T) {
	health := Route{Path: HealthzPath, Methods: []string{http.MethodGet, http.MethodHead}}
	if got := health.RegistrationPattern(); got != "GET /healthz" {
		t.Fatalf("health pattern = %q", got)
	}

	chat := Route{Path: ChatCompletionsPath, Methods: []string{http.MethodPost}}
	if got := chat.RegistrationPattern(); got != "POST /v1/chat/completions" {
		t.Fatalf("chat pattern = %q", got)
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

func TestIsPublicPath(t *testing.T) {
	for _, path := range []string{HealthzPath, ReadyzPath, MetricsPath, VersionPath} {
		if !IsPublicPath(path) {
			t.Fatalf("%s should be public", path)
		}
	}
	for _, path := range []string{ModelsPath, ChatCompletionsPath, EmbeddingsPath, "/v1/unknown"} {
		if IsPublicPath(path) {
			t.Fatalf("%s should not be public", path)
		}
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
