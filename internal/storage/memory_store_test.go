package storage

import (
	"encoding/json"
	"errors"
	"testing"
	"time"
)

// TestMemoryStorePutAndGet tests that the Put method writes an object to the store and the Get method reads it back.
func TestMemoryStorePutAndGet(t *testing.T) {
	store := NewMemoryStore()
	now := time.Date(2026, 3, 23, 12, 0, 0, 0, time.UTC)

	payload := json.RawMessage(`{"name":"alice"}`)

	if err := store.Put("user-1", payload, nil, now); err != nil {
		t.Fatalf("Put() error = %v, want nil", err)
	}

	item, err := store.Get("user-1", now)
	if err != nil {
		t.Fatalf("Get() error = %v, want nil", err)
	}

	if got := string(item.Payload); got != `{"name":"alice"}` {
		t.Fatalf("Get() payload = %s, want %s", got, `{"name":"alice"}`)
	}
}

// TestMemoryStorePutOverwrite tests that the Put method overwrites an existing object.
func TestMemoryStorePutOverwrite(t *testing.T) {
	store := NewMemoryStore()
	now := time.Date(2026, 3, 23, 12, 0, 0, 0, time.UTC)

	if err := store.Put("user-1", json.RawMessage(`{"name":"alice"}`), nil, now); err != nil {
		t.Fatalf("first Put() error = %v, want nil", err)
	}

	if err := store.Put("user-1", json.RawMessage(`{"name":"bob"}`), nil, now); err != nil {
		t.Fatalf("second Put() error = %v, want nil", err)
	}

	item, err := store.Get("user-1", now)
	if err != nil {
		t.Fatalf("Get() error = %v, want nil", err)
	}

	if got := string(item.Payload); got != `{"name":"bob"}` {
		t.Fatalf("Get() payload = %s, want %s", got, `{"name":"bob"}`)
	}
}

// TestMemoryStorePutValidation tests that the Put method validates the input data.
func TestMemoryStorePutValidation(t *testing.T) {
	store := NewMemoryStore()
	now := time.Date(2026, 3, 23, 12, 0, 0, 0, time.UTC)
	expiredAt := now.Add(-time.Second)

	tests := []struct {
		name      string
		key       string
		payload   json.RawMessage
		expiresAt *time.Time
		wantErr   error
	}{
		{
			name:    "empty key",
			key:     "",
			payload: json.RawMessage(`{"ok":true}`),
			wantErr: ErrInvalidKey,
		},
		{
			name:    "invalid json",
			key:     "user-1",
			payload: json.RawMessage(`{"bad":`),
			wantErr: ErrInvalidPayload,
		},
		{
			name:      "already expired",
			key:       "user-1",
			payload:   json.RawMessage(`{"ok":true}`),
			expiresAt: &expiredAt,
			wantErr:   ErrAlreadyExpired,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := store.Put(tt.key, tt.payload, tt.expiresAt, now)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Put() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

// TestMemoryStoreGetValidation tests that the Get method validates the input data.
func TestMemoryStoreGetValidation(t *testing.T) {
	store := NewMemoryStore()
	now := time.Date(2026, 3, 23, 12, 0, 0, 0, time.UTC)

	_, err := store.Get("", now)
	if !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("Get() error = %v, want %v", err, ErrInvalidKey)
	}

	_, err = store.Get("missing", now)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get() error = %v, want %v", err, ErrNotFound)
	}
}

// TestMemoryStoreGetExpiredDeletesLazily tests that the Get method deletes an expired object lazily.
func TestMemoryStoreGetExpiredDeletesLazily(t *testing.T) {
	store := NewMemoryStore()
	now := time.Date(2026, 3, 23, 12, 0, 0, 0, time.UTC)
	expiresAt := now.Add(time.Second)

	if err := store.Put("temp", json.RawMessage(`{"ttl":"short"}`), &expiresAt, now); err != nil {
		t.Fatalf("Put() error = %v, want nil", err)
	}

	_, err := store.Get("temp", now.Add(2*time.Second))
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get() error = %v, want %v", err, ErrNotFound)
	}

	if got := store.Len(now.Add(2 * time.Second)); got != 0 {
		t.Fatalf("Len() = %d, want 0", got)
	}
}

// TestMemoryStoreDeleteExpired tests that the DeleteExpired method deletes expired objects.
func TestMemoryStoreDeleteExpired(t *testing.T) {
	store := NewMemoryStore()
	now := time.Date(2026, 3, 23, 12, 0, 0, 0, time.UTC)
	expiredAt := now.Add(1 * time.Second)
	liveUntil := now.Add(10 * time.Second)

	if err := store.Put("expired", json.RawMessage(`{"id":1}`), &expiredAt, now); err != nil {
		t.Fatalf("Put(expired) error = %v, want nil", err)
	}

	if err := store.Put("alive", json.RawMessage(`{"id":2}`), &liveUntil, now); err != nil {
		t.Fatalf("Put(alive) error = %v, want nil", err)
	}

	deleted := store.DeleteExpired(now.Add(2 * time.Second))
	if deleted != 1 {
		t.Fatalf("DeleteExpired() = %d, want 1", deleted)
	}

	if got := store.Len(now.Add(2 * time.Second)); got != 1 {
		t.Fatalf("Len() = %d, want 1", got)
	}

	item, err := store.Get("alive", now.Add(2*time.Second))
	if err != nil {
		t.Fatalf("Get(alive) error = %v, want nil", err)
	}

	if got := string(item.Payload); got != `{"id":2}` {
		t.Fatalf("Get(alive) payload = %s, want %s", got, `{"id":2}`)
	}
}

// TestMemoryStoreLenCountsOnlyAliveItems tests that the Len method counts only alive objects.
func TestMemoryStoreLenCountsOnlyAliveItems(t *testing.T) {
	store := NewMemoryStore()
	now := time.Date(2026, 3, 23, 12, 0, 0, 0, time.UTC)
	expiredAt := now.Add(1 * time.Second)
	liveUntil := now.Add(1 * time.Hour)

	if err := store.Put("a", json.RawMessage(`{"id":1}`), &expiredAt, now); err != nil {
		t.Fatalf("Put(a) error = %v, want nil", err)
	}

	if err := store.Put("b", json.RawMessage(`{"id":2}`), &liveUntil, now); err != nil {
		t.Fatalf("Put(b) error = %v, want nil", err)
	}

	if err := store.Put("c", json.RawMessage(`{"id":3}`), nil, now); err != nil {
		t.Fatalf("Put(c) error = %v, want nil", err)
	}

	if got := store.Len(now); got != 3 {
		t.Fatalf("Len(now) = %d, want 3", got)
	}

	if got := store.Len(now.Add(2 * time.Second)); got != 2 {
		t.Fatalf("Len(later) = %d, want 2", got)
	}
}

// TestMemoryStoreSnapshotExcludesExpiredItems tests that the Snapshot method excludes expired items.
func TestMemoryStoreSnapshotExcludesExpiredItems(t *testing.T) {
	store := NewMemoryStore()
	now := time.Date(2026, 3, 23, 12, 0, 0, 0, time.UTC)
	expiredAt := now.Add(1 * time.Second)
	liveUntil := now.Add(1 * time.Hour)

	if err := store.Put("expired", json.RawMessage(`{"id":1}`), &expiredAt, now); err != nil {
		t.Fatalf("Put(expired) error = %v, want nil", err)
	}

	if err := store.Put("alive", json.RawMessage(`{"id":2}`), &liveUntil, now); err != nil {
		t.Fatalf("Put(alive) error = %v, want nil", err)
	}

	snapshot := store.Snapshot(now.Add(2 * time.Second))
	if len(snapshot.Items) != 1 {
		t.Fatalf("len(Snapshot.Items) = %d, want 1", len(snapshot.Items))
	}

	if snapshot.Items[0].Key != "alive" {
		t.Fatalf("Snapshot.Items[0].Key = %s, want alive", snapshot.Items[0].Key)
	}

	if got := string(snapshot.Items[0].Payload); got != `{"id":2}` {
		t.Fatalf("Snapshot.Items[0].Payload = %s, want %s", got, `{"id":2}`)
	}
}

// TestMemoryStoreSnapshotReturnsCopy tests that the Snapshot method returns a copy of the items.
func TestMemoryStoreSnapshotReturnsCopy(t *testing.T) {
	store := NewMemoryStore()
	now := time.Date(2026, 3, 23, 12, 0, 0, 0, time.UTC)

	if err := store.Put("user-1", json.RawMessage(`{"name":"alice"}`), nil, now); err != nil {
		t.Fatalf("Put() error = %v, want nil", err)
	}

	snapshot := store.Snapshot(now)
	if len(snapshot.Items) != 1 {
		t.Fatalf("len(Snapshot.Items) = %d, want 1", len(snapshot.Items))
	}

	snapshot.Items[0].Payload[9] = 'b' // alice -> blice

	item, err := store.Get("user-1", now)
	if err != nil {
		t.Fatalf("Get() error = %v, want nil", err)
	}

	if got := string(item.Payload); got != `{"name":"alice"}` {
		t.Fatalf("Get() payload = %s, want %s", got, `{"name":"alice"}`)
	}
}

// TestMemoryStoreRestore tests that the Restore method restores the items from the snapshot.
func TestMemoryStoreRestore(t *testing.T) {
	store := NewMemoryStore()
	now := time.Date(2026, 3, 23, 12, 0, 0, 0, time.UTC)
	expiredAt := now.Add(-time.Minute)
	liveUntil := now.Add(time.Hour)

	snapshot := Snapshot{
		SavedAt: now,
		Items: []SnapshotItem{
			{
				Key:     "alive",
				Payload: json.RawMessage(`{"id":1}`),
			},
			{
				Key:       "ttl-alive",
				Payload:   json.RawMessage(`{"id":2}`),
				ExpiresAt: &liveUntil,
			},
			{
				Key:       "ttl-expired",
				Payload:   json.RawMessage(`{"id":3}`),
				ExpiresAt: &expiredAt,
			},
		},
	}

	if err := store.Restore(snapshot, now); err != nil {
		t.Fatalf("Restore() error = %v, want nil", err)
	}

	if got := store.Len(now); got != 2 {
		t.Fatalf("Len() = %d, want 2", got)
	}

	_, err := store.Get("ttl-expired", now)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get(ttl-expired) error = %v, want %v", err, ErrNotFound)
	}
}

// TestMemoryStoreRestoreValidation tests that the Restore method validates the input data.
func TestMemoryStoreRestoreValidation(t *testing.T) {
	store := NewMemoryStore()
	now := time.Date(2026, 3, 23, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		snapshot Snapshot
		wantErr  error
	}{
		{
			name: "empty key",
			snapshot: Snapshot{
				Items: []SnapshotItem{
					{
						Key:     "",
						Payload: json.RawMessage(`{"id":1}`),
					},
				},
			},
			wantErr: ErrInvalidKey,
		},
		{
			name: "invalid payload",
			snapshot: Snapshot{
				Items: []SnapshotItem{
					{
						Key:     "bad",
						Payload: json.RawMessage(`{"id":`),
					},
				},
			},
			wantErr: ErrInvalidPayload,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := store.Restore(tt.snapshot, now)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Restore() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

// TestMemoryStorePutClonesInputPayload tests that the Put method clones the input payload.
func TestMemoryStorePutClonesInputPayload(t *testing.T) {
	store := NewMemoryStore()
	now := time.Date(2026, 3, 23, 12, 0, 0, 0, time.UTC)

	payload := json.RawMessage(`{"name":"alice"}`)
	if err := store.Put("user-1", payload, nil, now); err != nil {
		t.Fatalf("Put() error = %v, want nil", err)
	}

	payload[9] = 'b' // alice -> blice в исходном буфере

	item, err := store.Get("user-1", now)
	if err != nil {
		t.Fatalf("Get() error = %v, want nil", err)
	}

	if got := string(item.Payload); got != `{"name":"alice"}` {
		t.Fatalf("Get() payload = %s, want %s", got, `{"name":"alice"}`)
	}
}

// TestMemoryStoreGetReturnsClonedPayload tests that the Get method returns a cloned payload.
func TestMemoryStoreGetReturnsClonedPayload(t *testing.T) {
	store := NewMemoryStore()
	now := time.Date(2026, 3, 23, 12, 0, 0, 0, time.UTC)

	if err := store.Put("user-1", json.RawMessage(`{"name":"alice"}`), nil, now); err != nil {
		t.Fatalf("Put() error = %v, want nil", err)
	}

	item, err := store.Get("user-1", now)
	if err != nil {
		t.Fatalf("Get() error = %v, want nil", err)
	}

	item.Payload[9] = 'b' // alice -> blice только в копии

	itemAgain, err := store.Get("user-1", now)
	if err != nil {
		t.Fatalf("Get() second call error = %v, want nil", err)
	}

	if got := string(itemAgain.Payload); got != `{"name":"alice"}` {
		t.Fatalf("Get() second payload = %s, want %s", got, `{"name":"alice"}`)
	}
}
