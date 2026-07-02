package router

import (
	"sort"

	"open-ai-gateway/internal/compat"
	"open-ai-gateway/internal/provider"
)

type ModelRoute struct {
	ExternalModel string
	UpstreamModel string
	ProviderName  string
	Capabilities  map[string]bool
	Provider      provider.Provider
	Pricing       TokenPricing
	Fallbacks     []ProviderRoute
}

type ProviderRoute struct {
	UpstreamModel string
	ProviderName  string
	Provider      provider.Provider
	Pricing       TokenPricing
}

type TokenPricing struct {
	PromptUSDPer1MTokens     float64
	CompletionUSDPer1MTokens float64
}

type ModelRouter struct {
	routes map[string]ModelRoute
}

func NewModelRouter(routes []ModelRoute) *ModelRouter {
	byModel := make(map[string]ModelRoute, len(routes))
	for _, route := range routes {
		byModel[route.ExternalModel] = route.copy()
	}
	return &ModelRouter{routes: byModel}
}

func (r *ModelRouter) Resolve(model string) (ModelRoute, *compat.Error) {
	route, ok := r.routes[model]
	if !ok || route.Provider == nil {
		return ModelRoute{}, compat.ModelNotFound(model)
	}
	return route.copy(), nil
}

func (r *ModelRouter) ResolveFor(model, capability string) (ModelRoute, *compat.Error) {
	route, err := r.Resolve(model)
	if err != nil {
		return ModelRoute{}, err
	}
	if len(route.Capabilities) == 0 || route.Capabilities[capability] {
		return route, nil
	}
	return ModelRoute{}, compat.NewError(404, "invalid_request_error", "model does not support "+capability+": "+model, nil)
}

func (r *ModelRouter) Models() []compat.Model {
	if r == nil {
		return nil
	}
	models := make([]compat.Model, 0, len(r.routes))
	for model := range r.routes {
		models = append(models, compat.Model{
			ID:      model,
			Object:  "model",
			Created: 0,
			OwnedBy: "open-ai-gateway",
		})
	}
	sort.Slice(models, func(i, j int) bool {
		return models[i].ID < models[j].ID
	})
	return models
}

func (r *ModelRouter) ProviderNames() []string {
	if r == nil {
		return nil
	}
	seen := make(map[string]struct{})
	for _, route := range r.routes {
		if route.ProviderName != "" {
			seen[route.ProviderName] = struct{}{}
		}
		for _, fallback := range route.Fallbacks {
			if fallback.ProviderName != "" {
				seen[fallback.ProviderName] = struct{}{}
			}
		}
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (r *ModelRouter) ModelCount() int {
	if r == nil {
		return 0
	}
	return len(r.routes)
}

func (r ModelRoute) Attempts() []ProviderRoute {
	attempts := make([]ProviderRoute, 0, 1+len(r.Fallbacks))
	attempts = append(attempts, ProviderRoute{
		UpstreamModel: r.UpstreamModel,
		ProviderName:  r.ProviderName,
		Provider:      r.Provider,
		Pricing:       r.Pricing,
	})
	attempts = append(attempts, r.Fallbacks...)
	return attempts
}

func (r ModelRoute) copy() ModelRoute {
	return ModelRoute{
		ExternalModel: r.ExternalModel,
		UpstreamModel: r.UpstreamModel,
		ProviderName:  r.ProviderName,
		Capabilities:  copyCapabilities(r.Capabilities),
		Provider:      r.Provider,
		Pricing:       r.Pricing,
		Fallbacks:     append([]ProviderRoute(nil), r.Fallbacks...),
	}
}

func copyCapabilities(in map[string]bool) map[string]bool {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]bool, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
