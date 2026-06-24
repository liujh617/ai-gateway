package middleware

import (
	"net/http"
	"strings"

	"open-ai-gateway/internal/compat"
)

type ErrorWriter interface {
	WriteError(w http.ResponseWriter, err *compat.Error)
}

func Auth(apiKey string, errors ErrorWriter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/healthz" || r.URL.Path == "/metrics" {
				next.ServeHTTP(w, r)
				return
			}
			if apiKey == "" {
				next.ServeHTTP(w, r)
				return
			}
			got := strings.TrimSpace(r.Header.Get("Authorization"))
			want := "Bearer " + apiKey
			if got != want {
				SetLogError(r.Context(), "authentication_error", nil)
				errors.WriteError(w, compat.Authentication("invalid authorization token"))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
