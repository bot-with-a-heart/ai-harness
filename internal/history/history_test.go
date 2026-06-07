package history

import (
	"path/filepath"
	"testing"
	"time"
)

func TestSaveListAndLoad(t *testing.T) {
	now := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	store := Store{
		Dir: t.TempDir(),
		Now: func() time.Time {
			return now
		},
	}

	record, err := store.Save(Record{
		Command:      "run",
		Task:         "Fix tests",
		Provider:     "codex",
		FilesTouched: []string{"b.go", "a.go", "a.go"},
		TestsRun: []TestRun{
			{Command: "go test ./...", Passed: true, Output: "ok"},
		},
		Success: true,
	})
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if record.ID == "" || record.Timestamp != now {
		t.Fatalf("record identity = %+v", record)
	}
	if len(record.FilesTouched) != 2 || record.FilesTouched[0] != "a.go" {
		t.Fatalf("files were not normalized: %+v", record.FilesTouched)
	}

	records, err := store.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(records) != 1 || records[0].ID != record.ID {
		t.Fatalf("records = %+v", records)
	}

	loaded, err := store.Load(record.ID)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.Task != "Fix tests" || !loaded.Success {
		t.Fatalf("loaded = %+v", loaded)
	}
}

func TestListMissingDirectoryReturnsEmpty(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "missing"))

	records, err := store.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(records) != 0 {
		t.Fatalf("records = %+v", records)
	}
}

func TestLoadRejectsUnsafeID(t *testing.T) {
	store := NewStore(t.TempDir())

	if _, err := store.Load("../secret"); err == nil {
		t.Fatal("load succeeded with unsafe id")
	}
}
