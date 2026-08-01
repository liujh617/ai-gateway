package reasoningenvelope

import (
	"bytes"
	"crypto/rand"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestCodecSealOpenRoundTripUsesRandomNonce(t *testing.T) {
	now := time.Unix(1000, 0)
	codec := newTestCodec(t, &now, Key{ID: "active", Bytes: bytes.Repeat([]byte{1}, 32)}, nil)
	binding := Binding{Audience: "prod", Client: "alpha", ExternalModel: "codex-model"}
	want := Payload{
		EnvelopeID:       "env_1",
		Route:            RouteBinding{Dialect: "deepseek", Provider: "deepseek", UpstreamModel: "deepseek-chat"},
		ReasoningContent: "private reasoning",
		AssistantContent: "",
		CallIDs:          []string{"call_1", "call_2"},
	}
	first, err := codec.Seal(want, binding)
	if err != nil {
		t.Fatal(err)
	}
	second, err := codec.Seal(want, binding)
	if err != nil {
		t.Fatal(err)
	}
	if first == second || !strings.HasPrefix(first, "gwre1.active.") || !strings.HasPrefix(second, "gwre1.active.") {
		t.Fatalf("tokens first=%q second=%q", first, second)
	}
	got, err := codec.Open(first, binding)
	if err != nil {
		t.Fatal(err)
	}
	want.Version = 1
	want.Client = "alpha"
	want.ExternalModel = "codex-model"
	want.IssuedAt = now.Unix()
	want.ExpiresAt = now.Add(time.Hour).Unix()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("payload=%#v want=%#v", got, want)
	}
}

func TestCodecOpenRejectsInvalidTokensAndBindings(t *testing.T) {
	now := time.Unix(2000, 0)
	codec := newTestCodec(t, &now, Key{ID: "active", Bytes: bytes.Repeat([]byte{2}, 32)}, nil)
	binding := Binding{Audience: "prod", Client: "alpha", ExternalModel: "codex-model"}
	token, err := codec.Seal(testPayload(), binding)
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(token, ".")
	tamperedPayload := []byte(parts[2])
	tamperedPayload[len(tamperedPayload)/2] = differentBase64Byte(tamperedPayload[len(tamperedPayload)/2])
	tests := []struct {
		name    string
		token   string
		binding Binding
	}{
		{name: "prefix", token: "bad." + parts[1] + "." + parts[2], binding: binding},
		{name: "unknown key", token: parts[0] + ".missing." + parts[2], binding: binding},
		{name: "base64", token: parts[0] + "." + parts[1] + ".***", binding: binding},
		{name: "tampered", token: parts[0] + "." + parts[1] + "." + string(tamperedPayload), binding: binding},
		{name: "audience", token: token, binding: Binding{Audience: "other", Client: "alpha", ExternalModel: "codex-model"}},
		{name: "client", token: token, binding: Binding{Audience: "prod", Client: "beta", ExternalModel: "codex-model"}},
		{name: "model", token: token, binding: Binding{Audience: "prod", Client: "alpha", ExternalModel: "other-model"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := codec.Open(tt.token, tt.binding)
			if !errors.Is(err, ErrInvalid) {
				t.Fatalf("error=%v", err)
			}
		})
	}
}

func TestCodecOpenEnforcesTimeAndKeyRotation(t *testing.T) {
	issuedAt := time.Unix(3000, 0)
	oldKey := Key{ID: "old", Bytes: bytes.Repeat([]byte{3}, 32)}
	newKey := Key{ID: "new", Bytes: bytes.Repeat([]byte{4}, 32)}
	binding := Binding{Audience: "prod", Client: "alpha", ExternalModel: "codex-model"}
	oldCodec := newTestCodec(t, &issuedAt, oldKey, nil)
	token, err := oldCodec.Seal(testPayload(), binding)
	if err != nil {
		t.Fatal(err)
	}
	openedAt := issuedAt.Add(30 * time.Minute)
	rotated := newTestCodec(t, &openedAt, newKey, []Key{oldKey})
	if _, err := rotated.Open(token, binding); err != nil {
		t.Fatalf("previous key failed: %v", err)
	}
	newToken, err := rotated.Seal(testPayload(), binding)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(newToken, "gwre1.new.") {
		t.Fatalf("new token=%q", newToken)
	}
	openedAt = issuedAt.Add(time.Hour + time.Second)
	if _, err := rotated.Open(token, binding); !errors.Is(err, ErrInvalid) {
		t.Fatalf("expired error=%v", err)
	}

	future := time.Unix(9000, 0)
	futureCodec := newTestCodec(t, &future, oldKey, nil)
	futureToken, err := futureCodec.Seal(testPayload(), binding)
	if err != nil {
		t.Fatal(err)
	}
	openedAt = future.Add(-time.Minute)
	rotated = newTestCodec(t, &openedAt, newKey, []Key{oldKey})
	if _, err := rotated.Open(futureToken, binding); !errors.Is(err, ErrInvalid) {
		t.Fatalf("future issued-at error=%v", err)
	}
}

func TestCodecRejectsDuplicateCallsAndSizeLimits(t *testing.T) {
	now := time.Unix(4000, 0)
	key := Key{ID: "active", Bytes: bytes.Repeat([]byte{5}, 32)}
	codec := newTestCodec(t, &now, key, nil)
	binding := Binding{Audience: "prod", Client: "alpha", ExternalModel: "codex-model"}
	payload := testPayload()
	payload.CallIDs = []string{"call_1", "call_1"}
	if _, err := codec.Seal(payload, binding); !errors.Is(err, ErrInvalid) {
		t.Fatalf("duplicate call error=%v", err)
	}

	small, err := New(Config{
		ActiveKey: key, TTL: time.Hour,
		MaxEnvelopeBytes: 64, MaxPlaintextBytes: 32,
		Now: func() time.Time { return now }, Rand: rand.Reader,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := small.Seal(testPayload(), binding); !errors.Is(err, ErrInvalid) {
		t.Fatalf("plaintext limit error=%v", err)
	}
	if _, err := small.Open(strings.Repeat("x", 65), binding); !errors.Is(err, ErrInvalid) {
		t.Fatalf("envelope limit error=%v", err)
	}
}

func TestNewRejectsInvalidKeyConfiguration(t *testing.T) {
	valid := Key{ID: "active", Bytes: bytes.Repeat([]byte{6}, 32)}
	tests := []Config{
		{ActiveKey: Key{ID: "", Bytes: valid.Bytes}},
		{ActiveKey: Key{ID: "active", Bytes: []byte("short")}},
		{ActiveKey: valid, PreviousKeys: []Key{{ID: "active", Bytes: bytes.Repeat([]byte{7}, 32)}}},
	}
	for _, cfg := range tests {
		cfg.TTL = time.Hour
		cfg.MaxEnvelopeBytes = 1024
		cfg.MaxPlaintextBytes = 512
		if _, err := New(cfg); err == nil {
			t.Fatalf("config accepted: %#v", cfg)
		}
	}
}

func newTestCodec(t *testing.T, now *time.Time, active Key, previous []Key) *Codec {
	t.Helper()
	codec, err := New(Config{
		ActiveKey: active, PreviousKeys: previous, TTL: time.Hour,
		MaxEnvelopeBytes: 1 << 20, MaxPlaintextBytes: 1 << 19,
		Now: func() time.Time { return *now }, Rand: rand.Reader,
	})
	if err != nil {
		t.Fatal(err)
	}
	return codec
}

func testPayload() Payload {
	return Payload{
		EnvelopeID:       "env_1",
		Route:            RouteBinding{Dialect: "deepseek", Provider: "deepseek", UpstreamModel: "deepseek-chat"},
		ReasoningContent: "private reasoning",
		CallIDs:          []string{"call_1"},
	}
}

func differentBase64Byte(value byte) byte {
	if value == 'A' {
		return 'B'
	}
	return 'A'
}
