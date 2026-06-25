package routes

import "net/http"

const (
	UnknownPathLabel = "/__unknown__"

	HealthzPath         = "/healthz"
	ReadyzPath          = "/readyz"
	VersionPath         = "/version"
	MetricsPath         = "/metrics"
	ModelsPath          = "/v1/models"
	ChatCompletionsPath = "/v1/chat/completions"
	EmbeddingsPath      = "/v1/embeddings"
)

type Route struct {
	Path    string
	Methods []string
}

var definitions = []Route{
	{Path: HealthzPath, Methods: []string{http.MethodGet, http.MethodHead}},
	{Path: ReadyzPath, Methods: []string{http.MethodGet, http.MethodHead}},
	{Path: VersionPath, Methods: []string{http.MethodGet, http.MethodHead}},
	{Path: MetricsPath, Methods: []string{http.MethodGet, http.MethodHead}},
	{Path: ModelsPath, Methods: []string{http.MethodGet, http.MethodHead}},
	{Path: ChatCompletionsPath, Methods: []string{http.MethodPost}},
	{Path: EmbeddingsPath, Methods: []string{http.MethodPost}},
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

func Pattern(method, path string) string {
	return method + " " + path
}

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
