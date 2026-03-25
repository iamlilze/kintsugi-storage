package memory

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFileSnapshotterSaveAndLoad(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "snapshot.json")
	snapshotter := NewFileSnapshotter(path)

	expiresAt := time.Date(2026, 3, 24, 12, 0, 0, 0, time.UTC)
	savedAt := time.Date(2026, 3, 23, 12, 0, 0, 0, time.UTC)

	want := Snapshot{
		SavedAt: savedAt,
		Items: []SnapshotItem{
			{Key: "user-1", Payload: json.RawMessage(`{"name":"alice"}`), ExpiresAt: &expiresAt},
			{Key: "user-2", Payload: json.RawMessage(`{"name":"bob"}`)},
		},
	}

	if err := snapshotter.Save(want); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	got, err := snapshotter.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	assertSnapshotsEqual(t, got, want)
}

func TestFileSnapshotterSaveCreatesParentDirs(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "dir", "snapshot.json")
	snapshotter := NewFileSnapshotter(path)

	snap := Snapshot{
		SavedAt: time.Date(2026, 3, 23, 12, 0, 0, 0, time.UTC),
		Items:   []SnapshotItem{{Key: "user-1", Payload: json.RawMessage(`{"ok":true}`)}},
	}

	if err := snapshotter.Save(snap); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("os.Stat() error = %v", err)
	}
}

func TestFileSnapshotterOverwrites(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "snapshot.json")
	snapshotter := NewFileSnapshotter(path)

	first := Snapshot{
		SavedAt: time.Date(2026, 3, 23, 12, 0, 0, 0, time.UTC),
		Items:   []SnapshotItem{{Key: "user-1", Payload: json.RawMessage(`{"name":"alice"}`)}},
	}
	second := Snapshot{
		SavedAt: time.Date(2026, 3, 23, 13, 0, 0, 0, time.UTC),
		Items:   []SnapshotItem{{Key: "user-2", Payload: json.RawMessage(`{"name":"bob"}`)}},
	}

	_ = snapshotter.Save(first)
	_ = snapshotter.Save(second)

	got, err := snapshotter.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	assertSnapshotsEqual(t, got, second)
}

func TestFileSnapshotterLoadMissing(t *testing.T) {
	t.Parallel()

	snapshotter := NewFileSnapshotter(filepath.Join(t.TempDir(), "missing.json"))

	got, err := snapshotter.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !got.SavedAt.IsZero() {
		t.Fatalf("SavedAt = %v, want zero", got.SavedAt)
	}
	if got.Items == nil || len(got.Items) != 0 {
		t.Fatalf("Items = %v, want empty slice", got.Items)
	}
}

func TestFileSnapshotterLoadInvalidJSON(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "snapshot.json")
	_ = os.WriteFile(path, []byte(`{"broken":`), 0o644)

	snapshotter := NewFileSnapshotter(path)
	_, err := snapshotter.Load()
	if err == nil {
		t.Fatal("Load() error = nil, want non-nil")
	}
}

func TestFileSnapshotterEmptyPath(t *testing.T) {
	t.Parallel()

	snapshotter := NewFileSnapshotter("")

	if err := snapshotter.Save(Snapshot{}); err == nil {
		t.Fatal("Save('') error = nil, want non-nil")
	}
	if _, err := snapshotter.Load(); err == nil {
		t.Fatal("Load('') error = nil, want non-nil")
	}
}

func assertSnapshotsEqual(t *testing.T, got, want Snapshot) {
	t.Helper()

	if !got.SavedAt.Equal(want.SavedAt) {
		t.Fatalf("SavedAt = %v, want %v", got.SavedAt, want.SavedAt)
	}
	if len(got.Items) != len(want.Items) {
		t.Fatalf("len(Items) = %d, want %d", len(got.Items), len(want.Items))
	}
	for i := range want.Items {
		g, w := got.Items[i], want.Items[i]
		if g.Key != w.Key {
			t.Fatalf("Items[%d].Key = %q, want %q", i, g.Key, w.Key)
		}
		if string(g.Payload) != string(w.Payload) {
			t.Fatalf("Items[%d].Payload = %s, want %s", i, g.Payload, w.Payload)
		}
		switch {
		case g.ExpiresAt == nil && w.ExpiresAt == nil:
		case g.ExpiresAt == nil || w.ExpiresAt == nil:
			t.Fatalf("Items[%d].ExpiresAt mismatch: got %v, want %v", i, g.ExpiresAt, w.ExpiresAt)
		case !g.ExpiresAt.Equal(*w.ExpiresAt):
			t.Fatalf("Items[%d].ExpiresAt = %v, want %v", i, *g.ExpiresAt, *w.ExpiresAt)
		}
	}
}
