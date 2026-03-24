package persistence

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"kintsugi-storage/internal/storage"
)

// TestFileSnapshotterSaveAndLoad tests that the Save and Load methods work correctly.
func TestFileSnapshotterSaveAndLoad(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "snapshot.json")

	snapshotter := NewFileSnapshotter(path)

	expiresAt := time.Date(2026, 3, 24, 12, 0, 0, 0, time.UTC)
	savedAt := time.Date(2026, 3, 23, 12, 0, 0, 0, time.UTC)

	want := storage.Snapshot{
		SavedAt: savedAt,
		Items: []storage.SnapshotItem{
			{
				Key:       "user-1",
				Payload:   json.RawMessage(`{"name":"alice"}`),
				ExpiresAt: &expiresAt,
			},
			{
				Key:     "user-2",
				Payload: json.RawMessage(`{"name":"bob"}`),
			},
		},
	}

	if err := snapshotter.Save(want); err != nil {
		t.Fatalf("Save() error = %v, want nil", err)
	}

	got, err := snapshotter.Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}

	assertSnapshotsEqual(t, got, want)
}

// TestFileSnapshotterSaveCreatesParentDirectories tests that the Save method creates parent directories if they don't exist.
func TestFileSnapshotterSaveCreatesParentDirectories(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "dir", "snapshot.json")

	snapshotter := NewFileSnapshotter(path)

	snapshot := storage.Snapshot{
		SavedAt: time.Date(2026, 3, 23, 12, 0, 0, 0, time.UTC),
		Items: []storage.SnapshotItem{
			{
				Key:     "user-1",
				Payload: json.RawMessage(`{"ok":true}`),
			},
		},
	}

	if err := snapshotter.Save(snapshot); err != nil {
		t.Fatalf("Save() error = %v, want nil", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("os.Stat() error = %v, want nil", err)
	}
}

// TestFileSnapshotterSaveOverwritesExistingFile tests that the Save method overwrites an existing file.
func TestFileSnapshotterSaveOverwritesExistingFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "snapshot.json")

	snapshotter := NewFileSnapshotter(path)

	first := storage.Snapshot{
		SavedAt: time.Date(2026, 3, 23, 12, 0, 0, 0, time.UTC),
		Items: []storage.SnapshotItem{
			{
				Key:     "user-1",
				Payload: json.RawMessage(`{"name":"alice"}`),
			},
		},
	}

	second := storage.Snapshot{
		SavedAt: time.Date(2026, 3, 23, 13, 0, 0, 0, time.UTC),
		Items: []storage.SnapshotItem{
			{
				Key:     "user-2",
				Payload: json.RawMessage(`{"name":"bob"}`),
			},
		},
	}

	if err := snapshotter.Save(first); err != nil {
		t.Fatalf("first Save() error = %v, want nil", err)
	}

	if err := snapshotter.Save(second); err != nil {
		t.Fatalf("second Save() error = %v, want nil", err)
	}

	got, err := snapshotter.Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}

	assertSnapshotsEqual(t, got, second)
}

// TestFileSnapshotterLoadMissingFileReturnsEmptySnapshot tests that the Load method returns an empty snapshot if the file is missing.
func TestFileSnapshotterLoadMissingFileReturnsEmptySnapshot(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "missing.json")

	snapshotter := NewFileSnapshotter(path)

	got, err := snapshotter.Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}

	if !got.SavedAt.IsZero() {
		t.Fatalf("Load().SavedAt = %v, want zero time", got.SavedAt)
	}

	if got.Items == nil {
		t.Fatal("Load().Items = nil, want empty slice")
	}

	if len(got.Items) != 0 {
		t.Fatalf("len(Load().Items) = %d, want 0", len(got.Items))
	}
}

// TestFileSnapshotterLoadInvalidJSON tests that the Load method returns an error if the file contains invalid JSON.
func TestFileSnapshotterLoadInvalidJSON(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "snapshot.json")

	if err := os.WriteFile(path, []byte(`{"broken":`), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v, want nil", err)
	}

	snapshotter := NewFileSnapshotter(path)

	_, err := snapshotter.Load()
	if err == nil {
		t.Fatal("Load() error = nil, want non-nil")
	}
}

// TestFileSnapshotterSaveEmptyPath tests that the Save method returns an error if the path is empty.
func TestFileSnapshotterSaveEmptyPath(t *testing.T) {
	t.Parallel()

	snapshotter := NewFileSnapshotter("")

	err := snapshotter.Save(storage.Snapshot{})
	if err == nil {
		t.Fatal("Save() error = nil, want non-nil")
	}
}

// TestFileSnapshotterLoadEmptyPath tests that the Load method returns an error if the path is empty.
func TestFileSnapshotterLoadEmptyPath(t *testing.T) {
	t.Parallel()

	snapshotter := NewFileSnapshotter("")

	_, err := snapshotter.Load()
	if err == nil {
		t.Fatal("Load() error = nil, want non-nil")
	}
}

// TestFileSnapshotterSavePersistsValidJSONFile tests that the Save method persists a valid JSON file.
func TestFileSnapshotterSavePersistsValidJSONFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "snapshot.json")

	snapshotter := NewFileSnapshotter(path)

	snapshot := storage.Snapshot{
		SavedAt: time.Date(2026, 3, 23, 12, 0, 0, 0, time.UTC),
		Items: []storage.SnapshotItem{
			{
				Key:     "user-1",
				Payload: json.RawMessage(`{"name":"alice"}`),
			},
		},
	}

	if err := snapshotter.Save(snapshot); err != nil {
		t.Fatalf("Save() error = %v, want nil", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v, want nil", err)
	}

	var decoded storage.Snapshot
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("saved file json.Unmarshal() error = %v, want nil", err)
	}

	assertSnapshotsEqual(t, decoded, snapshot)
}

// assertSnapshotsEqual is a helper function to assert that two snapshots are equal.
func assertSnapshotsEqual(t *testing.T, got, want storage.Snapshot) {
	t.Helper()

	if !got.SavedAt.Equal(want.SavedAt) {
		t.Fatalf("Snapshot.SavedAt = %v, want %v", got.SavedAt, want.SavedAt)
	}

	if len(got.Items) != len(want.Items) {
		t.Fatalf("len(Snapshot.Items) = %d, want %d", len(got.Items), len(want.Items))
	}

	for i := range want.Items {
		gotItem := got.Items[i]
		wantItem := want.Items[i]

		if gotItem.Key != wantItem.Key {
			t.Fatalf("Snapshot.Items[%d].Key = %q, want %q", i, gotItem.Key, wantItem.Key)
		}

		if string(gotItem.Payload) != string(wantItem.Payload) {
			t.Fatalf(
				"Snapshot.Items[%d].Payload = %s, want %s",
				i,
				string(gotItem.Payload),
				string(wantItem.Payload),
			)
		}

		switch {
		case gotItem.ExpiresAt == nil && wantItem.ExpiresAt == nil:
		case gotItem.ExpiresAt == nil || wantItem.ExpiresAt == nil:
			t.Fatalf(
				"Snapshot.Items[%d].ExpiresAt mismatch: got %v, want %v",
				i,
				gotItem.ExpiresAt,
				wantItem.ExpiresAt,
			)
		case !gotItem.ExpiresAt.Equal(*wantItem.ExpiresAt):
			t.Fatalf(
				"Snapshot.Items[%d].ExpiresAt = %v, want %v",
				i,
				*gotItem.ExpiresAt,
				*wantItem.ExpiresAt,
			)
		}
	}
}
