package routes

import (
	"net/http"
	"strings"
)

const (
	UnknownPathLabel = "/__unknown__"

	HealthzPath                  = "/healthz"
	ReadyzPath                   = "/readyz"
	VersionPath                  = "/version"
	MetricsPath                  = "/metrics"
	ModelsPath                   = "/v1/models"
	ModelsRetrievePath           = "/v1/models/{model}"
	ChatCompletionsPath          = "/v1/chat/completions"
	ResponsesPath                = "/v1/responses"
	ResponsesRetrievePath        = "/v1/responses/{response_id}"
	CompletionsPath              = "/v1/completions"
	ImageGenerationsPath         = "/v1/images/generations"
	ModerationsPath              = "/v1/moderations"
	EmbeddingsPath               = "/v1/embeddings"
	AudioTranscriptionsPath      = "/v1/audio/transcriptions"
	AudioTranslationsPath        = "/v1/audio/translations"
	AudioSpeechPath              = "/v1/audio/speech"
	BatchesPath                  = "/v1/batches"
	BatchesRetrievePath          = "/v1/batches/{batch_id}"
	BatchesCancelPath            = "/v1/batches/{batch_id}/cancel"
	FilesPath                    = "/v1/files"
	FilesRetrievePath            = "/v1/files/{file_id}"
	FilesContentPath             = "/v1/files/{file_id}/content"
	FineTuningJobsPath           = "/v1/fine_tuning/jobs"
	FineTuningJobRetrievePath    = "/v1/fine_tuning/jobs/{job_id}"
	FineTuningJobEventsPath      = "/v1/fine_tuning/jobs/{job_id}/events"
	FineTuningJobCancelPath      = "/v1/fine_tuning/jobs/{job_id}/cancel"
	FineTuningJobCheckpointsPath = "/v1/fine_tuning/jobs/{job_id}/checkpoints"
	RealtimePath                 = "/v1/realtime"
	RealtimeTokensPath           = "/v1/realtime/tokens"
	AnthropicMessagesPath        = "/v1/messages"
)

type Route struct {
	Path    string
	Methods []string
	Public  bool
}

var definitions = []Route{
	{Path: HealthzPath, Methods: []string{http.MethodGet, http.MethodHead}, Public: true},
	{Path: ReadyzPath, Methods: []string{http.MethodGet, http.MethodHead}, Public: true},
	{Path: VersionPath, Methods: []string{http.MethodGet, http.MethodHead}, Public: true},
	{Path: MetricsPath, Methods: []string{http.MethodGet, http.MethodHead}, Public: true},
	{Path: ModelsPath, Methods: []string{http.MethodGet, http.MethodHead}},
	{Path: ModelsRetrievePath, Methods: []string{http.MethodGet, http.MethodHead}},
	{Path: ChatCompletionsPath, Methods: []string{http.MethodPost}},
	{Path: ResponsesPath, Methods: []string{http.MethodPost}},
	{Path: ResponsesRetrievePath, Methods: []string{http.MethodGet, http.MethodHead, http.MethodDelete}},
	{Path: CompletionsPath, Methods: []string{http.MethodPost}},
	{Path: ImageGenerationsPath, Methods: []string{http.MethodPost}},
	{Path: ModerationsPath, Methods: []string{http.MethodPost}},
	{Path: EmbeddingsPath, Methods: []string{http.MethodPost}},
	{Path: AudioTranscriptionsPath, Methods: []string{http.MethodPost}},
	{Path: AudioTranslationsPath, Methods: []string{http.MethodPost}},
	{Path: AudioSpeechPath, Methods: []string{http.MethodPost}},
	{Path: BatchesPath, Methods: []string{http.MethodPost, http.MethodGet}},
	{Path: BatchesRetrievePath, Methods: []string{http.MethodGet}},
	{Path: BatchesCancelPath, Methods: []string{http.MethodPost}},
	{Path: FilesPath, Methods: []string{http.MethodPost, http.MethodGet}},
	{Path: FilesRetrievePath, Methods: []string{http.MethodGet, http.MethodDelete}},
	{Path: FilesContentPath, Methods: []string{http.MethodGet}},
	{Path: AnthropicMessagesPath, Methods: []string{http.MethodPost}, Public: true},
	{Path: RealtimeTokensPath, Methods: []string{http.MethodPost}},
	{Path: RealtimePath, Methods: []string{http.MethodGet}, Public: true},
	{Path: FineTuningJobsPath, Methods: []string{http.MethodPost, http.MethodGet}},
	{Path: FineTuningJobRetrievePath, Methods: []string{http.MethodGet}},
	{Path: FineTuningJobEventsPath, Methods: []string{http.MethodGet}},
	{Path: FineTuningJobCancelPath, Methods: []string{http.MethodPost}},
	{Path: FineTuningJobCheckpointsPath, Methods: []string{http.MethodGet}},
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

var publicPaths = func() map[string]struct{} {
	paths := make(map[string]struct{})
	for _, route := range definitions {
		if route.Public {
			paths[route.Path] = struct{}{}
		}
	}
	return paths
}()

func Pattern(method, path string) string {
	return method + " " + path
}

func All() []Route {
	routes := make([]Route, 0, len(definitions))
	for _, route := range definitions {
		routes = append(routes, route.copy())
	}
	return routes
}

func (r Route) RegistrationPattern() string {
	method := r.registrationMethod()
	for _, item := range r.Methods {
		if item != http.MethodHead && item != method {
			return r.Path
		}
	}
	return Pattern(method, r.Path)
}

func NormalizePath(path string) string {
	if canonical, ok := canonicalPath(path); ok {
		return canonical
	}
	return UnknownPathLabel
}

func AllowedMethods(path string) ([]string, bool) {
	canonical, ok := canonicalPath(path)
	if !ok {
		return nil, false
	}
	methods, ok := methodsByPath[canonical]
	if !ok {
		return nil, false
	}
	return append([]string(nil), methods...), true
}

func MethodAllowed(path, method string) (bool, bool) {
	canonical, ok := canonicalPath(path)
	if !ok {
		return false, false
	}
	methods, ok := methodsByPath[canonical]
	if !ok {
		return false, false
	}
	for _, item := range methods {
		if method == item {
			return true, true
		}
	}
	return false, true
}

func AllowHeader(path string) (string, bool) {
	canonical, ok := canonicalPath(path)
	if !ok {
		return "", false
	}
	methods, ok := methodsByPath[canonical]
	if !ok {
		return "", false
	}
	return strings.Join(methods, ", "), true
}

func IsPublicPath(path string) bool {
	canonical, ok := canonicalPath(path)
	if !ok {
		return false
	}
	_, ok = publicPaths[canonical]
	return ok
}

func canonicalPath(path string) (string, bool) {
	if _, ok := knownPaths[path]; ok {
		return path, true
	}
	if strings.HasPrefix(path, BatchesPath+"/") && strings.HasSuffix(path, "/cancel") {
		suffix := strings.TrimPrefix(path, BatchesPath+"/")
		suffix = strings.TrimSuffix(suffix, "/cancel")
		if suffix != "" && !strings.Contains(suffix, "/") {
			return BatchesCancelPath, true
		}
	}
	for _, dynamic := range []struct {
		prefix    string
		canonical string
	}{
		{prefix: FineTuningJobsPath + "/", canonical: FineTuningJobRetrievePath},
		{prefix: FilesPath + "/", canonical: FilesRetrievePath},
		{prefix: BatchesPath + "/", canonical: BatchesRetrievePath},
		{prefix: ModelsPath + "/", canonical: ModelsRetrievePath},
		{prefix: ResponsesPath + "/", canonical: ResponsesRetrievePath},
	} {
		if singlePathSegment(path, dynamic.prefix) {
			return dynamic.canonical, true
		}
	}
	return "", false
}

func singlePathSegment(path, prefix string) bool {
	if !strings.HasPrefix(path, prefix) {
		return false
	}
	value := strings.TrimPrefix(path, prefix)
	return value != "" && !strings.Contains(value, "/")
}

func (r Route) copy() Route {
	return Route{
		Path:    r.Path,
		Methods: append([]string(nil), r.Methods...),
		Public:  r.Public,
	}
}

func (r Route) registrationMethod() string {
	for _, method := range r.Methods {
		if method != http.MethodHead {
			return method
		}
	}
	return r.Methods[0]
}
