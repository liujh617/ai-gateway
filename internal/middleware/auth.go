package middleware

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"open-ai-gateway/internal/compat"
	"open-ai-gateway/internal/routes"
)

type ErrorWriter interface {
	WriteError(w http.ResponseWriter, err *compat.Error)
}

func Auth(apiKeys []string, errors ErrorWriter) func(http.Handler) http.Handler {
	keys := append([]string(nil), apiKeys...)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if routes.IsPublicPath(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}
			if len(keys) == 0 {
				next.ServeHTTP(w, r)
				return
			}
			if !validBearerToken(r.Header.Get("Authorization"), keys) {
				SetLogError(r.Context(), "authentication_error", nil)
				errors.WriteError(w, compat.Authentication("invalid authorization token"))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func validBearerToken(header string, keys []string) bool {
	got := strings.TrimSpace(header)
	const prefix = "Bearer "
	if !strings.HasPrefix(got, prefix) {
		return false
	}
	token := strings.TrimSpace(strings.TrimPrefix(got, prefix))
	if token == "" {
		return false
	}
	for _, key := range keys {
		if subtle.ConstantTimeCompare([]byte(token), []byte(key)) == 1 {
			return true
		}
	}
	return false
}
