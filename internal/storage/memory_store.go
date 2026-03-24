package storage

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// На этапе компиляции проверяем, что MemoryStore действительно реализует Store.
var _ Store = (*MemoryStore)(nil)

// MemoryStore - потокобезопасное хранилище в оперативной памяти.
type MemoryStore struct {
	mu    sync.RWMutex
	items map[string]Item
}

// NewMemoryStore создает пустой in-memory store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		items: make(map[string]Item),
	}
}

// Put сохраняет объект по ключу или перезаписывает существующий.
func (s *MemoryStore) Put(key string, payload json.RawMessage, expiresAt *time.Time, now time.Time) error {
	if key == "" {
		return ErrInvalidKey
	}

	if !json.Valid(payload) {
		return ErrInvalidPayload
	}

	if expiresAt != nil && !expiresAt.After(now) {
		return ErrAlreadyExpired
	}

	item := NewItem(payload, expiresAt)

	s.mu.Lock()
	defer s.mu.Unlock()

	s.items[key] = item
	return nil
}

// Get возвращает объект по ключу, если он существует и еще не истек.
func (s *MemoryStore) Get(key string, now time.Time) (Item, error) {
	if key == "" {
		return Item{}, ErrInvalidKey
	}

	s.mu.RLock()
	item, ok := s.items[key]
	if !ok {
		s.mu.RUnlock()
		return Item{}, ErrNotFound
	}

	if !item.IsExpired(now) {
		s.mu.RUnlock()
		return item.Clone(), nil // Возвращаем копию, чтобы caller не получил ссылку на внутренние данные.
	}

	s.mu.RUnlock() // Если объект истек, read lock больше не нужен.

	s.mu.Lock() // Теперь берем write lock, чтобы безопасно удалить просроченный объект.
	defer s.mu.Unlock()

	item, ok = s.items[key]
	if !ok {
		return Item{}, ErrNotFound
	}

	if item.IsExpired(now) {
		delete(s.items, key)
		return Item{}, ErrNotFound
	}

	return item.Clone(), nil
}

// DeleteExpired удаляет все объекты, TTL которых уже закончился.
func (s *MemoryStore) DeleteExpired(now time.Time) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	deleted := 0 // Счетчик удаленных элементов, пригодится для метрик и логов.

	for key, item := range s.items {
		if item.IsExpired(now) {
			delete(s.items, key)
			deleted++
		}
	}

	return deleted
}

// Snapshot собирает сериализуемое состояние только из актуальных объектов.
func (s *MemoryStore) Snapshot(now time.Time) Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	snapshot := Snapshot{
		SavedAt: now,
		Items:   make([]SnapshotItem, 0, len(s.items)),
	}

	for key, item := range s.items {
		if item.IsExpired(now) {
			continue
		}

		snapshot.Items = append(snapshot.Items, SnapshotItem{
			Key:       key,
			Payload:   cloneBytes(item.Payload),
			ExpiresAt: cloneTimePtr(item.ExpiresAt),
		})
	}

	return snapshot
}

// Restore полностью заменяет текущее состояние данными из snapshot.
func (s *MemoryStore) Restore(snapshot Snapshot, now time.Time) error {
	restored := make(map[string]Item, len(snapshot.Items)) // Готовим новую map заранее, чтобы потом атомарно заменить old state.

	for _, snapItem := range snapshot.Items {
		if snapItem.Key == "" { // Пустой ключ в snapshot считаем повреждением данных.
			return fmt.Errorf("restore snapshot: %w", ErrInvalidKey)
		}

		if !json.Valid(snapItem.Payload) { // Невалидный JSON в snapshot тоже считаем ошибкой.
			return fmt.Errorf("restore snapshot: %w", ErrInvalidPayload)
		}

		item := NewItem(snapItem.Payload, snapItem.ExpiresAt)

		if item.IsExpired(now) {
			continue
		}

		restored[snapItem.Key] = item
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.items = restored
	return nil
}

// Len возвращает количество только актуальных объектов.
func (s *MemoryStore) Len(now time.Time) int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	count := 0

	for _, item := range s.items {
		if item.IsExpired(now) {
			continue
		}

		count++
	}

	return count
}
