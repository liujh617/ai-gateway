package middleware

import (
	"log/slog"
	"net/http"

	"open-ai-gateway/internal/compat"
)

func Recovery(logger *slog.Logger, errors ErrorWriter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if value := recover(); value != nil {
					logger.Error("panic recovered", "panic", value)
					errors.WriteError(w, compat.ServerError(http.StatusInternalServerError, "internal server error"))
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
