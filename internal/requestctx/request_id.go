package requestctx

import (
	"context"
	"time"
)

const RequestIDHeader = "X-Request-Id"

type requestIDKey struct{}

func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, id)
}

func RequestID(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey{}).(string)
	return id
}

type startedAtKey struct{}

// WithStartedAt stores the request start time in the context so downstream
// components (e.g. the audit recorder) can compute elapsed duration.
func WithStartedAt(ctx context.Context, t time.Time) context.Context {
	return context.WithValue(ctx, startedAtKey{}, t)
}

// StartedAt returns the request start time previously stored via WithStartedAt.
// The zero value is returned when no start time was set.
func StartedAt(ctx context.Context) time.Time {
	t, _ := ctx.Value(startedAtKey{}).(time.Time)
	return t
}
