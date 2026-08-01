package reasoningenvelope

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

const (
	tokenPrefix = "gwre1"
	version     = 1
)

var ErrInvalid = errors.New("invalid reasoning item")

type Key struct {
	ID    string
	Bytes []byte
}

type Config struct {
	ActiveKey         Key
	PreviousKeys      []Key
	TTL               time.Duration
	MaxEnvelopeBytes  int
	MaxPlaintextBytes int
	Now               func() time.Time
	Rand              io.Reader
}

type RouteBinding struct {
	Dialect       string `json:"dialect"`
	Provider      string `json:"provider"`
	UpstreamModel string `json:"upstream_model"`
}

type Binding struct {
	Audience      string
	Client        string
	ExternalModel string
}

type Payload struct {
	Version          int          `json:"v"`
	EnvelopeID       string       `json:"id"`
	Route            RouteBinding `json:"route"`
	Client           string       `json:"client"`
	ExternalModel    string       `json:"external_model"`
	IssuedAt         int64        `json:"iat"`
	ExpiresAt        int64        `json:"exp"`
	ReasoningContent string       `json:"reasoning_content"`
	AssistantContent string       `json:"assistant_content"`
	CallIDs          []string     `json:"call_ids"`
}

type Codec struct {
	active            keyMaterial
	keys              map[string]keyMaterial
	ttl               time.Duration
	maxEnvelopeBytes  int
	maxPlaintextBytes int
	now               func() time.Time
	rand              io.Reader
}

type keyMaterial struct {
	id  string
	key []byte
}

func New(config Config) (*Codec, error) {
	if config.TTL <= 0 {
		return nil, errors.New("reasoning envelope TTL must be positive")
	}
	if config.MaxEnvelopeBytes <= 0 || config.MaxPlaintextBytes <= 0 {
		return nil, errors.New("reasoning envelope size limits must be positive")
	}
	all := append([]Key{config.ActiveKey}, config.PreviousKeys...)
	keys := make(map[string]keyMaterial, len(all))
	for _, item := range all {
		if strings.TrimSpace(item.ID) == "" || item.ID != strings.TrimSpace(item.ID) || strings.Contains(item.ID, ".") {
			return nil, errors.New("reasoning envelope key ID is invalid")
		}
		if len(item.Bytes) != 32 {
			return nil, errors.New("reasoning envelope key must be 32 bytes")
		}
		if _, exists := keys[item.ID]; exists {
			return nil, errors.New("duplicate reasoning envelope key ID")
		}
		keys[item.ID] = keyMaterial{id: item.ID, key: append([]byte(nil), item.Bytes...)}
	}
	now := config.Now
	if now == nil {
		now = time.Now
	}
	random := config.Rand
	if random == nil {
		random = rand.Reader
	}
	return &Codec{
		active:            keys[config.ActiveKey.ID],
		keys:              keys,
		ttl:               config.TTL,
		maxEnvelopeBytes:  config.MaxEnvelopeBytes,
		maxPlaintextBytes: config.MaxPlaintextBytes,
		now:               now,
		rand:              random,
	}, nil
}

func (c *Codec) Seal(payload Payload, binding Binding) (string, error) {
	if c == nil {
		return "", invalid("codec is nil")
	}
	if err := validateBinding(binding); err != nil {
		return "", err
	}
	if payload.Version != 0 && payload.Version != version {
		return "", invalid("unsupported payload version")
	}
	now := c.now()
	payload.Version = version
	payload.Client = binding.Client
	payload.ExternalModel = binding.ExternalModel
	payload.IssuedAt = now.Unix()
	payload.ExpiresAt = now.Add(c.ttl).Unix()
	if err := validatePayload(payload, binding, now); err != nil {
		return "", err
	}
	plaintext, err := json.Marshal(payload)
	if err != nil {
		return "", invalid("payload encoding failed")
	}
	if len(plaintext) > c.maxPlaintextBytes {
		return "", invalid("plaintext exceeds limit")
	}
	gcm, err := newGCM(c.active.key)
	if err != nil {
		return "", invalid("cipher initialization failed")
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(c.rand, nonce); err != nil {
		return "", fmt.Errorf("generate reasoning envelope nonce: %w", err)
	}
	sealed := gcm.Seal(nil, nonce, plaintext, associatedData(binding))
	body := append(nonce, sealed...)
	token := tokenPrefix + "." + c.active.id + "." + base64.RawURLEncoding.EncodeToString(body)
	if len(token) > c.maxEnvelopeBytes {
		return "", invalid("envelope exceeds limit")
	}
	return token, nil
}

func (c *Codec) Open(token string, binding Binding) (Payload, error) {
	if c == nil || len(token) == 0 || len(token) > c.maxEnvelopeBytes {
		return Payload{}, invalid("envelope size is invalid")
	}
	if err := validateBinding(binding); err != nil {
		return Payload{}, err
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 || parts[0] != tokenPrefix || parts[1] == "" || parts[2] == "" {
		return Payload{}, invalid("token format is invalid")
	}
	key, ok := c.keys[parts[1]]
	if !ok {
		return Payload{}, invalid("key is unknown")
	}
	body, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return Payload{}, invalid("token encoding is invalid")
	}
	gcm, err := newGCM(key.key)
	if err != nil || len(body) < gcm.NonceSize()+gcm.Overhead() {
		return Payload{}, invalid("ciphertext is invalid")
	}
	plaintext, err := gcm.Open(nil, body[:gcm.NonceSize()], body[gcm.NonceSize():], associatedData(binding))
	if err != nil {
		return Payload{}, invalid("authentication failed")
	}
	if len(plaintext) > c.maxPlaintextBytes {
		return Payload{}, invalid("plaintext exceeds limit")
	}
	var payload Payload
	decoder := json.NewDecoder(bytes.NewReader(plaintext))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		return Payload{}, invalid("payload is invalid")
	}
	var extra json.RawMessage
	if err := decoder.Decode(&extra); err != io.EOF {
		return Payload{}, invalid("payload has trailing data")
	}
	if err := validatePayload(payload, binding, c.now()); err != nil {
		return Payload{}, err
	}
	payload.CallIDs = append([]string(nil), payload.CallIDs...)
	return payload, nil
}

func newGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func validateBinding(binding Binding) error {
	for _, value := range []string{binding.Audience, binding.Client, binding.ExternalModel} {
		if strings.TrimSpace(value) == "" || value != strings.TrimSpace(value) {
			return invalid("binding is invalid")
		}
	}
	return nil
}

func validatePayload(payload Payload, binding Binding, now time.Time) error {
	if payload.Version != version || strings.TrimSpace(payload.EnvelopeID) == "" || payload.EnvelopeID != strings.TrimSpace(payload.EnvelopeID) {
		return invalid("payload identity is invalid")
	}
	if payload.Client != binding.Client || payload.ExternalModel != binding.ExternalModel {
		return invalid("payload binding does not match")
	}
	if payload.IssuedAt > now.Unix() || payload.ExpiresAt <= now.Unix() || payload.ExpiresAt <= payload.IssuedAt {
		return invalid("payload time is invalid")
	}
	if strings.TrimSpace(payload.Route.Dialect) == "" || strings.TrimSpace(payload.Route.Provider) == "" || strings.TrimSpace(payload.Route.UpstreamModel) == "" {
		return invalid("payload route is invalid")
	}
	if payload.ReasoningContent == "" {
		return invalid("reasoning content is empty")
	}
	seen := make(map[string]struct{}, len(payload.CallIDs))
	for _, callID := range payload.CallIDs {
		if strings.TrimSpace(callID) == "" || callID != strings.TrimSpace(callID) {
			return invalid("call ID is invalid")
		}
		if _, exists := seen[callID]; exists {
			return invalid("call ID is duplicated")
		}
		seen[callID] = struct{}{}
	}
	return nil
}

func associatedData(binding Binding) []byte {
	var out []byte
	out = appendAADField(out, tokenPrefix)
	out = appendAADField(out, binding.Audience)
	out = appendAADField(out, binding.Client)
	out = appendAADField(out, binding.ExternalModel)
	return out
}

func appendAADField(out []byte, value string) []byte {
	var length [4]byte
	binary.BigEndian.PutUint32(length[:], uint32(len(value)))
	out = append(out, length[:]...)
	return append(out, value...)
}

func invalid(detail string) error {
	return fmt.Errorf("%w: %s", ErrInvalid, detail)
}
