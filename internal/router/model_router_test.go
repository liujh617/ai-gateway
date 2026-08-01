package router_test

import (
	"testing"

	"open-ai-gateway/internal/provider/fake"
	"open-ai-gateway/internal/router"
)

func TestResolveReturnsProviderAttemptsInOrder(t *testing.T) {
	primary := fake.New()
	backup := fake.New()
	modelRouter := router.NewModelRouter([]router.ModelRoute{{
		ExternalModel: "test-model",
		UpstreamModel: "primary-model",
		ProviderName:  "primary",
		Provider:      primary,
		Fallbacks: []router.ProviderRoute{{
			UpstreamModel: "backup-model",
			ProviderName:  "backup",
			Provider:      backup,
		}},
	}})

	route, err := modelRouter.Resolve("test-model")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	attempts := route.Attempts()
	if len(attempts) != 2 {
		t.Fatalf("attempts = %#v", attempts)
	}
	if attempts[0].ProviderName != "primary" || attempts[0].UpstreamModel != "primary-model" {
		t.Fatalf("primary attempt = %#v", attempts[0])
	}
	if attempts[1].ProviderName != "backup" || attempts[1].UpstreamModel != "backup-model" {
		t.Fatalf("fallback attempt = %#v", attempts[1])
	}
}

func TestModelRouteAttemptsPreserveReasoningDialectMetadata(t *testing.T) {
	route := router.ModelRoute{
		ExternalModel:   "codex-model",
		UpstreamModel:   "deepseek-chat",
		ProviderName:    "primary",
		Dialect:         "deepseek",
		ReasoningReplay: true,
		Provider:        fake.New(),
		Fallbacks: []router.ProviderRoute{{
			UpstreamModel:   "backup-model",
			ProviderName:    "backup",
			Dialect:         "openai-compatible",
			ReasoningReplay: false,
			Provider:        fake.New(),
		}},
	}

	resolved := router.NewModelRouter([]router.ModelRoute{route})
	got, err := resolved.Resolve("codex-model")
	if err != nil {
		t.Fatal(err)
	}
	attempts := got.Attempts()
	if len(attempts) != 2 || attempts[0].Dialect != "deepseek" || !attempts[0].ReasoningReplay || attempts[1].Dialect != "openai-compatible" || attempts[1].ReasoningReplay {
		t.Fatalf("attempts=%#v", attempts)
	}
	matched, ok := got.MatchAttempt("primary", "deepseek-chat", "deepseek")
	if !ok || !matched.ReasoningReplay {
		t.Fatalf("matched=%#v ok=%t", matched, ok)
	}
	if _, ok := got.MatchAttempt("primary", "deepseek-chat", "glm"); ok {
		t.Fatal("mismatched dialect matched")
	}
}

func TestResolveReturnsDefensiveCopy(t *testing.T) {
	modelRouter := router.NewModelRouter([]router.ModelRoute{{
		ExternalModel: "test-model",
		UpstreamModel: "primary-model",
		ProviderName:  "primary",
		Provider:      fake.New(),
		Capabilities:  map[string]bool{"chat": true},
		Fallbacks: []router.ProviderRoute{{
			UpstreamModel: "backup-model",
			ProviderName:  "backup",
			Provider:      fake.New(),
		}},
	}})

	route, err := modelRouter.Resolve("test-model")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	route.Capabilities["embeddings"] = true
	route.Fallbacks[0].ProviderName = "mutated"

	route, err = modelRouter.Resolve("test-model")
	if err != nil {
		t.Fatalf("Resolve again: %v", err)
	}
	if route.Capabilities["embeddings"] {
		t.Fatalf("capabilities were mutated: %#v", route.Capabilities)
	}
	if route.Fallbacks[0].ProviderName != "backup" {
		t.Fatalf("fallbacks were mutated: %#v", route.Fallbacks)
	}
}

func TestModelReturnsConfiguredMetadata(t *testing.T) {
	modelRouter := router.NewModelRouter([]router.ModelRoute{{ExternalModel: "test-model"}})

	model, ok := modelRouter.Model("test-model")
	if !ok {
		t.Fatal("configured model was not found")
	}
	if model.ID != "test-model" || model.Object != "model" || model.Created != 0 || model.OwnedBy != "open-ai-gateway" {
		t.Fatalf("model = %#v", model)
	}
	if _, ok := modelRouter.Model("missing"); ok {
		t.Fatal("missing model was found")
	}
}
