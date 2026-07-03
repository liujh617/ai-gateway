package middleware

import (
	"context"
	"crypto/subtle"
	"net/http"
	"strings"

	"open-ai-gateway/internal/compat"
	"open-ai-gateway/internal/routes"
)

type ErrorWriter interface {
	WriteError(w http.ResponseWriter, err *compat.Error)
}

type AuthCredential struct {
	Client string
	APIKey string
}

type clientKey struct{}

func Auth(credentials []AuthCredential, errors ErrorWriter) func(http.Handler) http.Handler {
	credentials = copyCredentials(credentials)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if routes.IsPublicPath(r.URL.Path) {
				client := "public"
				SetLogClient(r.Context(), client)
				SetMetricsClient(r.Context(), client)
				next.ServeHTTP(w, r)
				return
			}
			if len(credentials) == 0 {
				client := "unconfigured"
				SetLogClient(r.Context(), client)
				SetMetricsClient(r.Context(), client)
				r = r.WithContext(WithClient(r.Context(), client))
				next.ServeHTTP(w, r)
				return
			}
			client, ok := validBearerToken(r.Header.Get("Authorization"), credentials)
			if !ok {
				SetLogError(r.Context(), "authentication_error", nil)
				client := "unauthenticated"
				SetLogClient(r.Context(), client)
				SetMetricsClient(r.Context(), client)
				errors.WriteError(w, compat.Authentication("invalid authorization token"))
				return
			}
			SetLogClient(r.Context(), client)
			SetMetricsClient(r.Context(), client)
			r = r.WithContext(WithClient(r.Context(), client))
			next.ServeHTTP(w, r)
		})
	}
}

func WithClient(ctx context.Context, client string) context.Context {
	return context.WithValue(ctx, clientKey{}, client)
}

func ClientFromContext(ctx context.Context) string {
	client, _ := ctx.Value(clientKey{}).(string)
	return client
}

func validBearerToken(header string, credentials []AuthCredential) (string, bool) {
	got := strings.TrimSpace(header)
	const prefix = "Bearer "
	if !strings.HasPrefix(got, prefix) {
		return "", false
	}
	token := strings.TrimSpace(strings.TrimPrefix(got, prefix))
	if token == "" {
		return "", false
	}
	for _, credential := range credentials {
		if subtle.ConstantTimeCompare([]byte(token), []byte(credential.APIKey)) == 1 {
			return credential.Client, true
		}
	}
	return "", false
}

func copyCredentials(in []AuthCredential) []AuthCredential {
	out := make([]AuthCredential, 0, len(in))
	for _, credential := range in {
		if credential.APIKey == "" {
			continue
		}
		if credential.Client == "" {
			credential.Client = "default"
		}
		out = append(out, credential)
	}
	return out
}
