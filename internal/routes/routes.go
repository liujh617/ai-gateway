package routes

import "net/http"

const UnknownPathLabel = "/__unknown__"

type Route struct {
	Path    string
	Methods []string
}

var definitions = []Route{
	{Path: "/healthz", Methods: []string{http.MethodGet, http.MethodHead}},
	{Path: "/readyz", Methods: []string{http.MethodGet, http.MethodHead}},
	{Path: "/version", Methods: []string{http.MethodGet, http.MethodHead}},
	{Path: "/metrics", Methods: []string{http.MethodGet, http.MethodHead}},
	{Path: "/v1/models", Methods: []string{http.MethodGet, http.MethodHead}},
	{Path: "/v1/chat/completions", Methods: []string{http.MethodPost}},
	{Path: "/v1/embeddings", Methods: []string{http.MethodPost}},
}

var knownPaths = func() map[string]struct{} {
	paths := make(map[string]struct{}, len(definitions))
	for _, route := range definitions {
		paths[route.Path] = struct{}{}
	}
	return paths
}()

var methodsByPath = func() map[string][]string {
	methods := make(map[string][]string, len(definitions))
	for _, route := range definitions {
		methods[route.Path] = append([]string(nil), route.Methods...)
	}
	return methods
}()

func NormalizePath(path string) string {
	if _, ok := knownPaths[path]; ok {
		return path
	}
	return UnknownPathLabel
}

func AllowedMethods(path string) ([]string, bool) {
	methods, ok := methodsByPath[path]
	if !ok {
		return nil, false
	}
	return append([]string(nil), methods...), true
}
