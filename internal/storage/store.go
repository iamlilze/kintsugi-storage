package storage

import (
	"encoding/json"
	"time"
)

// Store описывает поведение хранилища
type Store interface {
	// Put сохраняет или перезаписывает объект по ключу.
	Put(key string, payload json.RawMessage, expiresAt *time.Time, now time.Time) error

	// Get возвращает объект по ключу или ошибку, если объект отсутствует или уже истек.
	Get(key string, now time.Time) (Item, error)

	// DeleteExpired удаляет все истекшие объекты и возвращает их количество.
	DeleteExpired(now time.Time) int

	// Snapshot возвращает сериализуемый срез актуального состояния стора.
	Snapshot(now time.Time) Snapshot

	// Restore восстанавливает состояние стора из snapshot.
	Restore(snapshot Snapshot, now time.Time) error

	// Len возвращает количество "живых" объектов на текущий момент.
	Len(now time.Time) int
}
