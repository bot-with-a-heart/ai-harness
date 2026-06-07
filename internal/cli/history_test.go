package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	appconfig "ai-harness/internal/config"
	historypkg "ai-harness/internal/history"
)

func TestHistoryListCommand(t *testing.T) {
	path := writeLocalConfig(t, "http://127.0.0.1:1234/v1")
	store := historypkg.NewStore(appconfig.HistoryDirForConfigPath(path))
	if _, err := store.Save(historypkg.Record{
		ID:        "newer",
		Timestamp: time.Date(2026, 6, 5, 15, 0, 0, 0, time.UTC),
		Command:   "run",
		Task:      "Fix failing tests",
		Provider:  "codex",
		Status:    "completed",
		Success:   true,
		Escalated: true,
	}); err != nil {
		t.Fatalf("save newer record: %v", err)
	}
	if _, err := store.Save(historypkg.Record{
		ID:        "older",
		Timestamp: time.Date(2026, 6, 5, 14, 0, 0, 0, time.UTC),
		Command:   "classify",
		Task:      "Explain repository",
		Provider:  "heuristic",
		Status:    "completed",
		Success:   true,
	}); err != nil {
		t.Fatalf("save older record: %v", err)
	}

	cmd := NewRootCommand("test")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"history", "--config", path, "list", "--limit", "1"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("history list failed: %v\n%s", err, out.String())
	}

	got := out.String()
	for _, want := range []string{"ID", "newer", "run", "codex", "Fix failing tests"} {
		if !strings.Contains(got, want) {
			t.Fatalf("history list missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "older") {
		t.Fatalf("history list ignored --limit:\n%s", got)
	}
}

func TestHistoryShowCommandJSON(t *testing.T) {
	path := writeLocalConfig(t, "http://127.0.0.1:1234/v1")
	store := historypkg.NewStore(appconfig.HistoryDirForConfigPath(path))
	if _, err := store.Save(historypkg.Record{
		ID:        "record-1",
		Timestamp: time.Date(2026, 6, 5, 15, 0, 0, 0, time.UTC),
		Command:   "ask-local",
		Task:      "Explain repository",
		Provider:  "lmstudio",
		Model:     "model-a",
		Status:    "completed",
		Success:   true,
	}); err != nil {
		t.Fatalf("save record: %v", err)
	}

	cmd := NewRootCommand("test")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"history", "--config", path, "show", "record-1", "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("history show failed: %v\n%s", err, out.String())
	}

	var record historypkg.Record
	if err := json.Unmarshal(out.Bytes(), &record); err != nil {
		t.Fatalf("decode history JSON: %v\n%s", err, out.String())
	}
	if record.ID != "record-1" || record.Provider != "lmstudio" || record.Task != "Explain repository" {
		t.Fatalf("record = %+v", record)
	}
}

func TestHistoryListEmptyDirectory(t *testing.T) {
	path := writeLocalConfig(t, "http://127.0.0.1:1234/v1")
	cmd := NewRootCommand("test")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"history", "--config", path, "list"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("history list failed: %v\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "No history records found.") {
		t.Fatalf("unexpected empty history output:\n%s", out.String())
	}
}

func historyRecordsForConfig(t *testing.T, path string) []historypkg.Record {
	t.Helper()

	store := historypkg.NewStore(appconfig.HistoryDirForConfigPath(path))
	records, err := store.List()
	if err != nil {
		t.Fatalf("list history records: %v", err)
	}
	return records
}
