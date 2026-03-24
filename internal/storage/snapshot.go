package storage

import (
	"encoding/json"
	"time"
)

// Snapshot - это сериализуемое состояние всего хранилища.
type Snapshot struct {
	SavedAt time.Time      `json:"saved_at"`
	Items   []SnapshotItem `json:"items"`
}

// SnapshotItem - сериализуемая версия одного объекта.
type SnapshotItem struct {
	Key       string          `json:"key"`
	Payload   json.RawMessage `json:"payload"`
	ExpiresAt *time.Time      `json:"expires_at,omitempty"`
}

// Clone возвращает полную копию snapshot.
func (s Snapshot) Clone() Snapshot {
	cloned := Snapshot{
		SavedAt: s.SavedAt,
		Items:   make([]SnapshotItem, 0, len(s.Items)),
	}

	for _, item := range s.Items {
		cloned.Items = append(cloned.Items, SnapshotItem{
			Key:       item.Key,
			Payload:   cloneBytes(item.Payload),
			ExpiresAt: cloneTimePtr(item.ExpiresAt),
		})
	}

	return cloned
}
