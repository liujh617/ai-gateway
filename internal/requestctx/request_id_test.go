package requestctx

import (
	"context"
	"testing"
	"time"
)

func TestRequestIDRoundTrip(t *testing.T) {
	ctx := context.Background()
	if got := RequestID(ctx); got != "" {
		t.Fatalf("empty context request id = %q", got)
	}
	ctx = WithRequestID(ctx, "req-123")
	if got := RequestID(ctx); got != "req-123" {
		t.Fatalf("request id = %q, want req-123", got)
	}
}

func TestStartedAtRoundTrip(t *testing.T) {
	ctx := context.Background()
	if got := StartedAt(ctx); !got.IsZero() {
		t.Fatalf("empty context started at = %v", got)
	}
	now := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	ctx = WithStartedAt(ctx, now)
	if got := StartedAt(ctx); !got.Equal(now) {
		t.Fatalf("started at = %v, want %v", got, now)
	}
}
