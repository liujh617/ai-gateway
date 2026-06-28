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
