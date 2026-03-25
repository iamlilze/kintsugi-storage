package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"kintsugi-storage/internal/storage"
)

var _ storage.Store = (*Store)(nil)

type item struct {
	payload   json.RawMessage
	expiresAt *time.Time
}

func (i item) isExpired(now time.Time) bool {
	if i.expiresAt == nil {
		return false
	}
	return !now.Before(*i.expiresAt)
}

type StoreOption func(*Store)

// WithClock позволяет подменить источник времени (полезно для тестов).
func WithClock(fn func() time.Time) StoreOption {
	return func(s *Store) { s.clock = fn }
}

// Store — потокобезопасное хранилище в оперативной памяти.
type Store struct {
	mu    sync.RWMutex
	items map[string]item
	clock func() time.Time
}

func New(opts ...StoreOption) *Store {
	s := &Store{
		items: make(map[string]item),
		clock: time.Now,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (s *Store) Put(_ context.Context, key string, value json.RawMessage, ttl *time.Duration) error {
	if key == "" {
		return storage.ErrInvalidKey
	}
	if !json.Valid(value) {
		return storage.ErrInvalidPayload
	}

	now := s.clock()
	var expiresAt *time.Time
	if ttl != nil {
		if *ttl <= 0 {
			return storage.ErrAlreadyExpired
		}
		t := now.Add(*ttl)
		expiresAt = &t
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.items[key] = item{
		payload:   cloneBytes(value),
		expiresAt: expiresAt,
	}
	return nil
}

func (s *Store) Get(_ context.Context, key string) (json.RawMessage, error) {
	if key == "" {
		return nil, storage.ErrInvalidKey
	}

	now := s.clock()

	s.mu.RLock()
	it, ok := s.items[key]
	if !ok {
		s.mu.RUnlock()
		return nil, storage.ErrNotFound
	}
	if !it.isExpired(now) {
		payload := cloneBytes(it.payload)
		s.mu.RUnlock()
		return payload, nil
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()

	it, ok = s.items[key]
	if !ok {
		return nil, storage.ErrNotFound
	}
	if it.isExpired(now) {
		delete(s.items, key)
		return nil, storage.ErrNotFound
	}
	return cloneBytes(it.payload), nil
}

func (s *Store) Len(_ context.Context) (int, error) {
	now := s.clock()

	s.mu.RLock()
	defer s.mu.RUnlock()

	count := 0
	for _, it := range s.items {
		if !it.isExpired(now) {
			count++
		}
	}
	return count, nil
}

func (s *Store) Ping(_ context.Context) error { return nil }

func (s *Store) Close() error { return nil }

func (s *Store) DeleteExpired() int {
	now := s.clock()

	s.mu.Lock()
	defer s.mu.Unlock()

	deleted := 0
	for key, it := range s.items {
		if it.isExpired(now) {
			delete(s.items, key)
			deleted++
		}
	}
	return deleted
}

// Snapshot собирает сериализуемое состояние только из актуальных объектов.
func (s *Store) Snapshot() Snapshot {
	now := s.clock()

	s.mu.RLock()
	defer s.mu.RUnlock()

	snap := Snapshot{
		SavedAt: now,
		Items:   make([]SnapshotItem, 0, len(s.items)),
	}

	for key, it := range s.items {
		if it.isExpired(now) {
			continue
		}
		snap.Items = append(snap.Items, SnapshotItem{
			Key:       key,
			Payload:   cloneBytes(it.payload),
			ExpiresAt: cloneTimePtr(it.expiresAt),
		})
	}
	return snap
}

// Restore полностью заменяет состояние данными из snapshot.
func (s *Store) Restore(snap Snapshot) error {
	now := s.clock()
	restored := make(map[string]item, len(snap.Items))

	for _, si := range snap.Items {
		if si.Key == "" {
			return fmt.Errorf("restore snapshot: %w", storage.ErrInvalidKey)
		}
		if !json.Valid(si.Payload) {
			return fmt.Errorf("restore snapshot: %w", storage.ErrInvalidPayload)
		}
		it := item{
			payload:   cloneBytes(si.Payload),
			expiresAt: cloneTimePtr(si.ExpiresAt),
		}
		if it.isExpired(now) {
			continue
		}
		restored[si.Key] = it
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.items = restored
	return nil
}

func cloneBytes(src []byte) []byte {
	if src == nil {
		return nil
	}
	dst := make([]byte, len(src))
	copy(dst, src)
	return dst
}

func cloneTimePtr(src *time.Time) *time.Time {
	if src == nil {
		return nil
	}
	v := *src
	return &v
}
