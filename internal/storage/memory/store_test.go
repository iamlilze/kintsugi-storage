package memory

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"kintsugi-storage/internal/storage"
)

func TestStorePutAndGet(t *testing.T) {
	s := New()
	ctx := context.Background()

	payload := json.RawMessage(`{"name":"alice"}`)
	if err := s.Put(ctx, "user-1", payload, nil); err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	got, err := s.Get(ctx, "user-1")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if string(got) != `{"name":"alice"}` {
		t.Fatalf("Get() = %s, want %s", got, `{"name":"alice"}`)
	}
}

func TestStorePutOverwrite(t *testing.T) {
	s := New()
	ctx := context.Background()

	if err := s.Put(ctx, "user-1", json.RawMessage(`{"name":"alice"}`), nil); err != nil {
		t.Fatalf("first Put() error = %v", err)
	}
	if err := s.Put(ctx, "user-1", json.RawMessage(`{"name":"bob"}`), nil); err != nil {
		t.Fatalf("second Put() error = %v", err)
	}

	got, err := s.Get(ctx, "user-1")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if string(got) != `{"name":"bob"}` {
		t.Fatalf("Get() = %s, want %s", got, `{"name":"bob"}`)
	}
}

func TestStorePutValidation(t *testing.T) {
	s := New()
	ctx := context.Background()

	negativeTTL := -time.Second

	tests := []struct {
		name    string
		key     string
		payload json.RawMessage
		ttl     *time.Duration
		wantErr error
	}{
		{"empty key", "", json.RawMessage(`{"ok":true}`), nil, storage.ErrInvalidKey},
		{"invalid json", "user-1", json.RawMessage(`{"bad":`), nil, storage.ErrInvalidPayload},
		{"negative ttl", "user-1", json.RawMessage(`{"ok":true}`), &negativeTTL, storage.ErrAlreadyExpired},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := s.Put(ctx, tt.key, tt.payload, tt.ttl)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Put() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestStoreGetValidation(t *testing.T) {
	s := New()
	ctx := context.Background()

	_, err := s.Get(ctx, "")
	if !errors.Is(err, storage.ErrInvalidKey) {
		t.Fatalf("Get('') error = %v, want %v", err, storage.ErrInvalidKey)
	}

	_, err = s.Get(ctx, "missing")
	if !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("Get('missing') error = %v, want %v", err, storage.ErrNotFound)
	}
}

// при чтении expired записи она должна удалиться (lazy eviction)
func TestStoreGetExpiredDeletesLazily(t *testing.T) {
	now := time.Date(2026, 3, 23, 12, 0, 0, 0, time.UTC)
	s := New(WithClock(func() time.Time { return now }))
	ctx := context.Background()

	ttl := time.Second
	if err := s.Put(ctx, "temp", json.RawMessage(`{"ttl":"short"}`), &ttl); err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	now = now.Add(2 * time.Second)

	_, err := s.Get(ctx, "temp")
	if !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("Get() error = %v, want %v", err, storage.ErrNotFound)
	}

	n, _ := s.Len(ctx)
	if n != 0 {
		t.Fatalf("Len() = %d, want 0", n)
	}
}

func TestStoreDeleteExpired(t *testing.T) {
	now := time.Date(2026, 3, 23, 12, 0, 0, 0, time.UTC)
	s := New(WithClock(func() time.Time { return now }))
	ctx := context.Background()

	shortTTL := time.Second
	longTTL := 10 * time.Second

	if err := s.Put(ctx, "expired", json.RawMessage(`{"id":1}`), &shortTTL); err != nil {
		t.Fatalf("Put(expired) error = %v", err)
	}
	if err := s.Put(ctx, "alive", json.RawMessage(`{"id":2}`), &longTTL); err != nil {
		t.Fatalf("Put(alive) error = %v", err)
	}

	now = now.Add(2 * time.Second)

	deleted := s.DeleteExpired()
	if deleted != 1 {
		t.Fatalf("DeleteExpired() = %d, want 1", deleted)
	}

	n, _ := s.Len(ctx)
	if n != 1 {
		t.Fatalf("Len() = %d, want 1", n)
	}

	got, err := s.Get(ctx, "alive")
	if err != nil {
		t.Fatalf("Get(alive) error = %v", err)
	}
	if string(got) != `{"id":2}` {
		t.Fatalf("Get(alive) = %s, want %s", got, `{"id":2}`)
	}
}

func TestStoreLenCountsOnlyAlive(t *testing.T) {
	now := time.Date(2026, 3, 23, 12, 0, 0, 0, time.UTC)
	s := New(WithClock(func() time.Time { return now }))
	ctx := context.Background()

	shortTTL := time.Second
	longTTL := time.Hour

	_ = s.Put(ctx, "a", json.RawMessage(`{"id":1}`), &shortTTL)
	_ = s.Put(ctx, "b", json.RawMessage(`{"id":2}`), &longTTL)
	_ = s.Put(ctx, "c", json.RawMessage(`{"id":3}`), nil)

	n, _ := s.Len(ctx)
	if n != 3 {
		t.Fatalf("Len(now) = %d, want 3", n)
	}

	now = now.Add(2 * time.Second)

	n, _ = s.Len(ctx)
	if n != 2 {
		t.Fatalf("Len(later) = %d, want 2", n)
	}
}

func TestStoreSnapshotExcludesExpired(t *testing.T) {
	now := time.Date(2026, 3, 23, 12, 0, 0, 0, time.UTC)
	s := New(WithClock(func() time.Time { return now }))
	ctx := context.Background()

	shortTTL := time.Second
	longTTL := time.Hour

	_ = s.Put(ctx, "expired", json.RawMessage(`{"id":1}`), &shortTTL)
	_ = s.Put(ctx, "alive", json.RawMessage(`{"id":2}`), &longTTL)

	now = now.Add(2 * time.Second)

	snap := s.Snapshot()
	if len(snap.Items) != 1 {
		t.Fatalf("len(Snapshot.Items) = %d, want 1", len(snap.Items))
	}
	if snap.Items[0].Key != "alive" {
		t.Fatalf("Snapshot.Items[0].Key = %s, want alive", snap.Items[0].Key)
	}
}

func TestStoreSnapshotReturnsCopy(t *testing.T) {
	s := New()
	ctx := context.Background()

	_ = s.Put(ctx, "user-1", json.RawMessage(`{"name":"alice"}`), nil)

	snap := s.Snapshot()
	if len(snap.Items) != 1 {
		t.Fatalf("len(Snapshot.Items) = %d, want 1", len(snap.Items))
	}

	snap.Items[0].Payload[9] = 'b'

	got, _ := s.Get(ctx, "user-1")
	if string(got) != `{"name":"alice"}` {
		t.Fatalf("Get() = %s, want %s", got, `{"name":"alice"}`)
	}
}

func TestStoreRestore(t *testing.T) {
	now := time.Date(2026, 3, 23, 12, 0, 0, 0, time.UTC)
	s := New(WithClock(func() time.Time { return now }))
	ctx := context.Background()

	expiredAt := now.Add(-time.Minute)
	liveUntil := now.Add(time.Hour)

	snap := Snapshot{
		SavedAt: now,
		Items: []SnapshotItem{
			{Key: "alive", Payload: json.RawMessage(`{"id":1}`)},
			{Key: "ttl-alive", Payload: json.RawMessage(`{"id":2}`), ExpiresAt: &liveUntil},
			{Key: "ttl-expired", Payload: json.RawMessage(`{"id":3}`), ExpiresAt: &expiredAt},
		},
	}

	if err := s.Restore(snap); err != nil {
		t.Fatalf("Restore() error = %v", err)
	}

	n, _ := s.Len(ctx)
	if n != 2 {
		t.Fatalf("Len() = %d, want 2", n)
	}

	_, err := s.Get(ctx, "ttl-expired")
	if !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("Get(ttl-expired) error = %v, want %v", err, storage.ErrNotFound)
	}
}

func TestStoreRestoreValidation(t *testing.T) {
	s := New()

	tests := []struct {
		name    string
		snap    Snapshot
		wantErr error
	}{
		{
			name:    "empty key",
			snap:    Snapshot{Items: []SnapshotItem{{Key: "", Payload: json.RawMessage(`{"id":1}`)}}},
			wantErr: storage.ErrInvalidKey,
		},
		{
			name:    "invalid payload",
			snap:    Snapshot{Items: []SnapshotItem{{Key: "bad", Payload: json.RawMessage(`{"id":`)}}},
			wantErr: storage.ErrInvalidPayload,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := s.Restore(tt.snap)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Restore() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestStorePutClonesPayload(t *testing.T) {
	s := New()
	ctx := context.Background()

	payload := json.RawMessage(`{"name":"alice"}`)
	_ = s.Put(ctx, "user-1", payload, nil)

	payload[9] = 'b'

	got, _ := s.Get(ctx, "user-1")
	if string(got) != `{"name":"alice"}` {
		t.Fatalf("Get() = %s, want %s", got, `{"name":"alice"}`)
	}
}

func TestStoreGetReturnsCopy(t *testing.T) {
	s := New()
	ctx := context.Background()

	_ = s.Put(ctx, "user-1", json.RawMessage(`{"name":"alice"}`), nil)

	got1, _ := s.Get(ctx, "user-1")
	got1[9] = 'b'

	got2, _ := s.Get(ctx, "user-1")
	if string(got2) != `{"name":"alice"}` {
		t.Fatalf("Get() second call = %s, want %s", got2, `{"name":"alice"}`)
	}
}

// go test -race должен поймать гонки если они есть
func TestStoreConcurrentAccess(t *testing.T) {
	t.Parallel()

	s := New()
	ctx := context.Background()

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			key := fmt.Sprintf("key-%d", n)
			val := json.RawMessage(fmt.Sprintf(`{"n":%d}`, n))
			if err := s.Put(ctx, key, val, nil); err != nil {
				t.Errorf("Put(%s): %v", key, err)
			}
			if _, err := s.Get(ctx, key); err != nil {
				t.Errorf("Get(%s): %v", key, err)
			}
		}(i)
	}
	wg.Wait()

	n, _ := s.Len(ctx)
	if n != 50 {
		t.Fatalf("Len() = %d after concurrent writes, want 50", n)
	}
}
