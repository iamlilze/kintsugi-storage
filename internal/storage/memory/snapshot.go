package memory

import (
	"encoding/json"
	"time"
)

type Snapshot struct {
	SavedAt time.Time      `json:"saved_at"`
	Items   []SnapshotItem `json:"items"`
}

type SnapshotItem struct {
	Key       string          `json:"key"`
	Payload   json.RawMessage `json:"payload"`
	ExpiresAt *time.Time      `json:"expires_at,omitempty"`
}
