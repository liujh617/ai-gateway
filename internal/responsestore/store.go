package responsestore

import (
	"container/list"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"open-ai-gateway/internal/compat"
)

var (
	ErrDisabled        = errors.New("response store is disabled")
	ErrContextTooLarge = errors.New("response context is too large")
	ErrIDCollision     = errors.New("response id already exists")
)

type MissReason string

const (
	MissNotFound MissReason = "not_found"
	MissExpired  MissReason = "expired"
	MissClient   MissReason = "client"
	MissModel    MissReason = "model"
)

type EvictionReason string

const (
	EvictionExpired  EvictionReason = "expired"
	EvictionCapacity EvictionReason = "capacity"
)

type Clock interface {
	Now() time.Time
}

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

type Config struct {
	TTL             time.Duration
	MaxEntries      int
	MaxContextBytes int64
	MaxTotalBytes   int64
}

func (c Config) enabled() bool {
	return c.TTL > 0 && c.MaxEntries > 0 && c.MaxContextBytes > 0 && c.MaxTotalBytes > 0
}

type Record struct {
	ID         string
	Client     string
	Model      string
	Transcript []compat.ChatMessage
}

type Stats struct {
	Entries   int
	Bytes     int64
	Evictions map[EvictionReason]uint64
	Misses    map[MissReason]uint64
}

type entry struct {
	record    Record
	bytes     int64
	expiresAt time.Time
	lru       *list.Element
}

type Store struct {
	mu        sync.Mutex
	config    Config
	clock     Clock
	entries   map[string]*entry
	lru       *list.List
	bytes     int64
	evictions map[EvictionReason]uint64
	misses    map[MissReason]uint64
}

func New(config Config, clock Clock) *Store {
	if clock == nil {
		clock = systemClock{}
	}
	return &Store{
		config:    config,
		clock:     clock,
		entries:   make(map[string]*entry),
		lru:       list.New(),
		evictions: make(map[EvictionReason]uint64),
		misses:    make(map[MissReason]uint64),
	}
}

func (s *Store) Enabled() bool {
	return s != nil && s.config.enabled()
}

func (s *Store) Put(record Record) error {
	if s == nil || !s.config.enabled() {
		return ErrDisabled
	}
	cloned := cloneRecord(record)
	size, err := encodedTranscriptSize(cloned.Transcript)
	if err != nil {
		return err
	}
	if size > s.config.MaxContextBytes {
		return ErrContextTooLarge
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.clock.Now()
	s.removeExpiredLocked(now)
	if _, exists := s.entries[cloned.ID]; exists {
		return ErrIDCollision
	}
	if size > s.config.MaxTotalBytes {
		return ErrContextTooLarge
	}
	for len(s.entries) >= s.config.MaxEntries || s.bytes+size > s.config.MaxTotalBytes {
		oldest := s.lru.Back()
		if oldest == nil {
			break
		}
		s.removeLocked(oldest.Value.(*entry), EvictionCapacity)
	}
	e := &entry{record: cloned, bytes: size, expiresAt: now.Add(s.config.TTL)}
	e.lru = s.lru.PushFront(e)
	s.entries[cloned.ID] = e
	s.bytes += size
	return nil
}

func (s *Store) Get(id, client, model string) (Record, MissReason, bool) {
	if s == nil || !s.config.enabled() {
		return Record{}, MissNotFound, false
	}
	s.mu.Lock()
	e, ok := s.entries[id]
	if !ok {
		s.misses[MissNotFound]++
		s.mu.Unlock()
		return Record{}, MissNotFound, false
	}
	if !s.clock.Now().Before(e.expiresAt) {
		s.removeLocked(e, EvictionExpired)
		s.misses[MissExpired]++
		s.mu.Unlock()
		return Record{}, MissExpired, false
	}
	if e.record.Client != client {
		s.misses[MissClient]++
		s.mu.Unlock()
		return Record{}, MissClient, false
	}
	if e.record.Model != model {
		s.misses[MissModel]++
		s.mu.Unlock()
		return Record{}, MissModel, false
	}
	s.lru.MoveToFront(e.lru)
	record := e.record
	s.mu.Unlock()
	return cloneRecord(record), "", true
}

func (s *Store) Snapshot() Stats {
	if s == nil {
		return Stats{Evictions: map[EvictionReason]uint64{}, Misses: map[MissReason]uint64{}}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.config.enabled() {
		s.removeExpiredLocked(s.clock.Now())
	}
	return Stats{
		Entries:   len(s.entries),
		Bytes:     s.bytes,
		Evictions: cloneEvictions(s.evictions),
		Misses:    cloneMisses(s.misses),
	}
}

func (s *Store) removeExpiredLocked(now time.Time) {
	for element := s.lru.Back(); element != nil; {
		previous := element.Prev()
		e := element.Value.(*entry)
		if !now.Before(e.expiresAt) {
			s.removeLocked(e, EvictionExpired)
		}
		element = previous
	}
}

func (s *Store) removeLocked(e *entry, reason EvictionReason) {
	delete(s.entries, e.record.ID)
	s.lru.Remove(e.lru)
	s.bytes -= e.bytes
	s.evictions[reason]++
}

func encodedTranscriptSize(transcript []compat.ChatMessage) (int64, error) {
	data, err := json.Marshal(transcript)
	return int64(len(data)), err
}

func cloneRecord(record Record) Record {
	cloned := Record{ID: record.ID, Client: record.Client, Model: record.Model}
	if record.Transcript == nil {
		return cloned
	}
	cloned.Transcript = make([]compat.ChatMessage, len(record.Transcript))
	for i, message := range record.Transcript {
		cloned.Transcript[i] = compat.ChatMessage{Role: message.Role, Content: cloneRaw(message.Content)}
		if message.Extra != nil {
			cloned.Transcript[i].Extra = make(map[string]json.RawMessage, len(message.Extra))
			for key, value := range message.Extra {
				cloned.Transcript[i].Extra[key] = cloneRaw(value)
			}
		}
	}
	return cloned
}

func cloneRaw(value json.RawMessage) json.RawMessage {
	if value == nil {
		return nil
	}
	return append(json.RawMessage(nil), value...)
}

func cloneEvictions(source map[EvictionReason]uint64) map[EvictionReason]uint64 {
	result := make(map[EvictionReason]uint64, len(source))
	for reason, count := range source {
		result[reason] = count
	}
	return result
}

func cloneMisses(source map[MissReason]uint64) map[MissReason]uint64 {
	result := make(map[MissReason]uint64, len(source))
	for reason, count := range source {
		result[reason] = count
	}
	return result
}
