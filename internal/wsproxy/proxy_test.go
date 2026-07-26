package wsproxy_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"open-ai-gateway/internal/wsproxy"

	"github.com/coder/websocket"
)

// pairedServer creates an httptest server where each WebSocket connection
// is paired with the next one. The first connection reads messages and the
// second writes them back (simulating upstream behavior).
func pairedServer(t *testing.T) (*httptest.Server, func() (*websocket.Conn, *websocket.Conn)) {
	t.Helper()
	var mu sync.Mutex
	var pending []*websocket.Conn

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		mu.Lock()
		pending = append(pending, conn)
		mu.Unlock()
	}))

	getPair := func() (*websocket.Conn, *websocket.Conn) {
		for {
			mu.Lock()
			if len(pending) >= 2 {
				a, b := pending[0], pending[1]
				pending = pending[2:]
				mu.Unlock()
				return a, b
			}
			mu.Unlock()
			time.Sleep(10 * time.Millisecond)
		}
	}

	return server, getPair
}

func TestRelayBidirectional(t *testing.T) {
	server, getPair := pairedServer(t)
	defer server.Close()

	wsURL := strings.Replace(server.URL, "http://", "ws://", 1)
	client, _, err := websocket.Dial(context.Background(), wsURL, nil)
	if err != nil {
		t.Fatalf("dial client: %v", err)
	}
	defer client.CloseNow()

	upstream, _, err := websocket.Dial(context.Background(), wsURL, nil)
	if err != nil {
		t.Fatalf("dial upstream: %v", err)
	}
	defer upstream.CloseNow()

	// Wait for the server to pair them
	a, b := getPair()

	testMsg := []byte("hello from client")
	client.Write(context.Background(), websocket.MessageText, testMsg)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go func() {
		time.Sleep(200 * time.Millisecond)
		a.Close(websocket.StatusNormalClosure, "")
		b.Close(websocket.StatusNormalClosure, "")
		client.Close(websocket.StatusNormalClosure, "")
		upstream.Close(websocket.StatusNormalClosure, "")
	}()

	stats := wsproxy.Relay(ctx, client, upstream, wsproxy.Config{})
	if stats.Duration == 0 {
		t.Fatal("duration is zero")
	}
}

func TestConnectionLimiter(t *testing.T) {
	limiter := wsproxy.NewConnectionLimiter(3)

	if !limiter.Acquire() || !limiter.Acquire() || !limiter.Acquire() {
		t.Fatal("expected 3 acquires to succeed")
	}
	if limiter.Acquire() {
		t.Fatal("4th acquire should fail at capacity")
	}
	if limiter.Count() != 3 {
		t.Fatalf("count = %d", limiter.Count())
	}

	limiter.Release()
	if limiter.Count() != 2 {
		t.Fatalf("count after release = %d", limiter.Count())
	}
	if !limiter.Acquire() {
		t.Fatal("acquire after release should succeed")
	}
}

func TestConnectionLimiterNoLimit(t *testing.T) {
	limiter := wsproxy.NewConnectionLimiter(0)
	for i := 0; i < 10; i++ {
		if !limiter.Acquire() {
			t.Fatalf("acquire %d failed with no limit", i)
		}
	}
}
