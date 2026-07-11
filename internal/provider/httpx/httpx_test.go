package httpx_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"open-ai-gateway/internal/compat"
	"open-ai-gateway/internal/provider/httpx"
)

func TestStreamReadsSSEAndDone(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, "data: {\"id\":\"chunk\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"m\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"hi\"},\"finish_reason\":null}]}\n\n")
		io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	resp, err := http.Get(server.URL)
	if err != nil {
		t.Fatalf("get stream: %v", err)
	}
	defer resp.Body.Close()
	if err := httpx.RequireEventStreamResponse(resp); err != nil {
		t.Fatalf("RequireEventStreamResponse: %v", err)
	}
	stream := httpx.NewChatCompletionStream(resp.Body)
	defer stream.Close()

	chunk, err := stream.Next(context.Background())
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if chunk.Choices[0].Delta.Content != "hi" {
		t.Fatalf("content = %q", chunk.Choices[0].Delta.Content)
	}
	if _, err := stream.Next(context.Background()); err != io.EOF {
		t.Fatalf("done err = %v, want EOF", err)
	}
}

func TestUpstreamErrorMapping(t *testing.T) {
	resp := &http.Response{
		StatusCode: http.StatusInternalServerError,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"upstream exploded","type":"server_error","code":"upstream_error"}}`)),
	}
	resp.Header.Set("Content-Type", "application/json")

	err := httpx.UpstreamError(resp)
	compatErr, ok := err.(*compat.Error)
	if !ok {
		t.Fatalf("error type = %T", err)
	}
	if compatErr.Status != http.StatusBadGateway {
		t.Fatalf("status = %d", compatErr.Status)
	}
	if compatErr.Message != "upstream exploded" || compatErr.Type != "server_error" {
		t.Fatalf("mapped error = %+v", compatErr)
	}
	if compatErr.Code == nil || *compatErr.Code != "upstream_error" {
		t.Fatalf("code = %v", compatErr.Code)
	}
}

func TestTransportTimeoutIsDeadlineExceeded(t *testing.T) {
	err := httpx.TransportError(&timeoutError{})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want DeadlineExceeded", err)
	}
}

type timeoutError struct{}

func (*timeoutError) Error() string   { return "timeout" }
func (*timeoutError) Timeout() bool   { return true }
func (*timeoutError) Temporary() bool { return true }

type timeoutReadCloser struct{}

func (*timeoutReadCloser) Read([]byte) (int, error) { return 0, &timeoutError{} }
func (*timeoutReadCloser) Close() error             { return nil }

func TestStreamReadTimeoutIsDeadlineExceeded(t *testing.T) {
	stream := httpx.NewChatCompletionStream(&timeoutReadCloser{})
	defer stream.Close()

	_, err := stream.Next(context.Background())
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want DeadlineExceeded", err)
	}
}
