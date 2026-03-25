package memory

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type FileSnapshotter struct {
	path string
}

func NewFileSnapshotter(path string) *FileSnapshotter {
	return &FileSnapshotter{path: path}
}

// Save сохраняет snapshot на диск атомарно: temp file → rename.
func (s *FileSnapshotter) Save(snapshot Snapshot) error {
	if s.path == "" {
		return fmt.Errorf("save snapshot: empty path")
	}

	data, err := json.Marshal(snapshot)
	if err != nil {
		return fmt.Errorf("save snapshot: marshal: %w", err)
	}

	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("save snapshot: create dir: %w", err)
	}

	tmpFile, err := os.CreateTemp(dir, "snapshot-*.tmp")
	if err != nil {
		return fmt.Errorf("save snapshot: create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	defer func() { _ = os.Remove(tmpPath) }()

	if _, err := tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("save snapshot: write temp file: %w", err)
	}
	if err := tmpFile.Sync(); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("save snapshot: sync temp file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("save snapshot: close temp file: %w", err)
	}
	if err := os.Rename(tmpPath, s.path); err != nil {
		return fmt.Errorf("save snapshot: rename temp file: %w", err)
	}
	return nil
}

// Если файла нет — возвращает пустой snapshot.
func (s *FileSnapshotter) Load() (Snapshot, error) {
	if s.path == "" {
		return Snapshot{}, fmt.Errorf("load snapshot: empty path")
	}

	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Snapshot{
				SavedAt: time.Time{},
				Items:   []SnapshotItem{},
			}, nil
		}
		return Snapshot{}, fmt.Errorf("load snapshot: read file: %w", err)
	}

	var snapshot Snapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return Snapshot{}, fmt.Errorf("load snapshot: unmarshal: %w", err)
	}

	if snapshot.Items == nil {
		snapshot.Items = []SnapshotItem{}
	}
	return snapshot, nil
}
