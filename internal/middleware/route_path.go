package middleware

const UnknownRoutePathLabel = "/__unknown__"

var knownRoutePaths = map[string]struct{}{
	"/healthz":             {},
	"/readyz":              {},
	"/version":             {},
	"/metrics":             {},
	"/v1/models":           {},
	"/v1/chat/completions": {},
	"/v1/embeddings":       {},
}

func NormalizeRoutePath(path string) string {
	if _, ok := knownRoutePaths[path]; ok {
		return path
	}
	return UnknownRoutePathLabel
}
