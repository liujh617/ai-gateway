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
	Provider      provider.Provider
}

type ModelRouter struct {
	routes map[string]ModelRoute
}

func NewModelRouter(routes []ModelRoute) *ModelRouter {
	byModel := make(map[string]ModelRoute, len(routes))
	for _, route := range routes {
		byModel[route.ExternalModel] = route
	}
	return &ModelRouter{routes: byModel}
}

func (r *ModelRouter) Resolve(model string) (ModelRoute, *compat.Error) {
	route, ok := r.routes[model]
	if !ok || route.Provider == nil {
		return ModelRoute{}, compat.ModelNotFound(model)
	}
	return route, nil
}

func (r *ModelRouter) Models() []compat.Model {
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
