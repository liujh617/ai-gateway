package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"

	"open-ai-gateway/internal/requestctx"
)

const (
	maxRequestIDLength = 128
)

func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := normalizeRequestID(r.Header.Get(requestctx.RequestIDHeader))
		if id == "" {
			id = newRequestID()
		}
		w.Header().Set(requestctx.RequestIDHeader, id)
		ctx := requestctx.WithRequestID(r.Context(), id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func RequestIDFromContext(ctx context.Context) string {
	return requestctx.RequestID(ctx)
}

func newRequestID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "unknown"
	}
	return hex.EncodeToString(b[:])
}

func normalizeRequestID(raw string) string {
	id := strings.TrimSpace(raw)
	if id == "" || len(id) > maxRequestIDLength {
		return ""
	}
	for _, r := range id {
		if r <= ' ' || r > '~' {
			return ""
		}
	}
	return id
}
