package responsestore

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"open-ai-gateway/internal/compat"
)

type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(d)
	c.mu.Unlock()
}

func message(role, content string) compat.ChatMessage {
	return compat.ChatMessage{Role: role, Content: json.RawMessage(fmt.Sprintf("%q", content))}
}

func newTestStore(clock Clock, mutate func(*Config)) *Store {
	cfg := Config{TTL: time.Hour, MaxEntries: 10, MaxContextBytes: 1024, MaxTotalBytes: 4096}
	if mutate != nil {
		mutate(&cfg)
	}
	return New(cfg, clock)
}

func TestPutGetReturnsDeepCopy(t *testing.T) {
	clock := &fakeClock{now: time.Unix(100, 0)}
	store := newTestStore(clock, nil)
	record := Record{ID: "resp_1", Client: "client-a", Model: "gpt", Transcript: []compat.ChatMessage{{Role: "assistant", Content: json.RawMessage(`"hello"`), Extra: map[string]json.RawMessage{"tool_calls": json.RawMessage(`[{"id":"call_1"}]`)}}}}
	if err := store.Put(record); err != nil {
		t.Fatal(err)
	}
	record.Transcript[0].Content[1] = 'X'
	record.Transcript[0].Extra["tool_calls"][2] = 'X'

	got, reason, ok := store.Get("resp_1", "client-a", "gpt")
	if !ok || reason != "" || string(got.Transcript[0].Content) != `"hello"` || string(got.Transcript[0].Extra["tool_calls"]) != `[{"id":"call_1"}]` {
		t.Fatalf("unexpected Get result: ok=%v reason=%q record=%+v", ok, reason, got)
	}
	got.Transcript[0].Content[1] = 'Y'
	again, _, _ := store.Get("resp_1", "client-a", "gpt")
	if string(again.Transcript[0].Content) != `"hello"` {
		t.Fatalf("Get exposed stored memory: %s", again.Transcript[0].Content)
	}
}

func TestGetMissReasonsAndExpiry(t *testing.T) {
	clock := &fakeClock{now: time.Unix(100, 0)}
	store := newTestStore(clock, func(c *Config) { c.TTL = time.Minute })
	if err := store.Put(Record{ID: "resp_1", Client: "client-a", Model: "gpt", Transcript: []compat.ChatMessage{message("user", "hi")}}); err != nil {
		t.Fatal(err)
	}
	for name, tc := range map[string]struct {
		client, model string
		want          MissReason
	}{
		"not found": {"client-a", "gpt", MissNotFound},
		"client":    {"client-b", "gpt", MissClient},
		"model":     {"client-a", "other", MissModel},
	} {
		id := "resp_1"
		if name == "not found" {
			id = "missing"
		}
		if _, got, ok := store.Get(id, tc.client, tc.model); ok || got != tc.want {
			t.Errorf("%s: got ok=%v reason=%q, want %q", name, ok, got, tc.want)
		}
	}
	clock.Advance(time.Minute)
	if _, reason, ok := store.Get("resp_1", "client-a", "gpt"); ok || reason != MissExpired {
		t.Fatalf("expired Get: ok=%v reason=%q", ok, reason)
	}
	if _, reason, ok := store.Get("resp_1", "client-a", "gpt"); ok || reason != MissNotFound {
		t.Fatalf("second expired Get: ok=%v reason=%q", ok, reason)
	}
}

func TestGetRefreshesLRUButNotAbsoluteExpiry(t *testing.T) {
	clock := &fakeClock{now: time.Unix(100, 0)}
	store := newTestStore(clock, func(c *Config) { c.TTL = time.Minute; c.MaxEntries = 2 })
	put := func(id string) {
		t.Helper()
		if err := store.Put(Record{ID: id, Client: "c", Model: "m", Transcript: []compat.ChatMessage{message("user", id)}}); err != nil {
			t.Fatal(err)
		}
	}
	put("one")
	clock.Advance(10 * time.Second)
	put("two")
	if _, _, ok := store.Get("one", "c", "m"); !ok {
		t.Fatal("expected one")
	}
	put("three")
	if _, reason, ok := store.Get("two", "c", "m"); ok || reason != MissNotFound {
		t.Fatalf("two should be LRU-evicted, ok=%v reason=%q", ok, reason)
	}
	clock.Advance(50 * time.Second)
	if _, reason, ok := store.Get("one", "c", "m"); ok || reason != MissExpired {
		t.Fatalf("Get must not extend absolute TTL: ok=%v reason=%q", ok, reason)
	}
}

func TestLimitsAndCollision(t *testing.T) {
	clock := &fakeClock{now: time.Unix(100, 0)}
	record := Record{ID: "one", Client: "c", Model: "m", Transcript: []compat.ChatMessage{message("user", "one")}}
	size, err := encodedTranscriptSize(record.Transcript)
	if err != nil {
		t.Fatal(err)
	}

	tooSmall := newTestStore(clock, func(c *Config) { c.MaxContextBytes = size - 1 })
	if err := tooSmall.Put(record); !errors.Is(err, ErrContextTooLarge) {
		t.Fatalf("got %v", err)
	}

	store := newTestStore(clock, func(c *Config) { c.MaxTotalBytes = size + 1 })
	if err := store.Put(record); err != nil {
		t.Fatal(err)
	}
	if err := store.Put(Record{ID: "two", Client: "c", Model: "m", Transcript: []compat.ChatMessage{message("user", "two")}}); err != nil {
		t.Fatal(err)
	}
	if _, reason, ok := store.Get("one", "c", "m"); ok || reason != MissNotFound {
		t.Fatalf("one should be byte-evicted: ok=%v reason=%q", ok, reason)
	}

	if err := store.Put(Record{ID: "two", Client: "c", Model: "m", Transcript: []compat.ChatMessage{message("user", "replacement")}}); !errors.Is(err, ErrIDCollision) {
		t.Fatalf("got %v", err)
	}
	got, _, _ := store.Get("two", "c", "m")
	if string(got.Transcript[0].Content) != `"two"` {
		t.Fatalf("collision overwrote record: %s", got.Transcript[0].Content)
	}
}

func TestDisabledStore(t *testing.T) {
	store := New(Config{}, nil)
	if err := store.Put(Record{ID: "x"}); !errors.Is(err, ErrDisabled) {
		t.Fatalf("got %v", err)
	}
	if _, reason, ok := store.Get("x", "c", "m"); ok || reason != MissNotFound {
		t.Fatalf("ok=%v reason=%q", ok, reason)
	}
}

func TestSnapshotAndConcurrentAccess(t *testing.T) {
	clock := &fakeClock{now: time.Unix(100, 0)}
	store := newTestStore(clock, func(c *Config) { c.MaxEntries = 100 })
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id := fmt.Sprintf("resp_%d", i)
			if err := store.Put(Record{ID: id, Client: "c", Model: "m", Transcript: []compat.ChatMessage{message("user", id)}}); err != nil {
				t.Errorf("Put: %v", err)
				return
			}
			if _, _, ok := store.Get(id, "c", "m"); !ok {
				t.Errorf("missing %s", id)
			}
		}(i)
	}
	wg.Wait()
	stats := store.Snapshot()
	if stats.Entries != 50 || stats.Bytes <= 0 {
		t.Fatalf("unexpected stats: %+v", stats)
	}
}
