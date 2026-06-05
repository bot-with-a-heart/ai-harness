package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	repoctx "ai-harness/internal/context"
)

func TestContextCommandPrintsSummary(t *testing.T) {
	restore := stubContextCollector(t, sampleSnapshot())
	defer restore()

	cmd := NewRootCommand("test")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"context", "--path", "C:\\repo"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("context command failed: %v\n%s", err, out.String())
	}

	got := out.String()
	for _, want := range []string{"Repository: C:\\repo", "Languages:", "Go", "Frameworks:", "cobra", "Directory Structure:", "README.md", "Sensitive Exclusions:"} {
		if !strings.Contains(got, want) {
			t.Fatalf("context output missing %q:\n%s", want, got)
		}
	}
}

func TestContextCommandPrintsJSON(t *testing.T) {
	restore := stubContextCollector(t, sampleSnapshot())
	defer restore()

	cmd := NewRootCommand("test")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"context", "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("context command failed: %v\n%s", err, out.String())
	}

	var snapshot repoctx.Snapshot
	if err := json.Unmarshal(out.Bytes(), &snapshot); err != nil {
		t.Fatalf("decode JSON output: %v\n%s", err, out.String())
	}
	if snapshot.Root != "C:\\repo" || len(snapshot.KeyFiles) != 1 {
		t.Fatalf("snapshot = %+v", snapshot)
	}
}

func stubContextCollector(t *testing.T, snapshot repoctx.Snapshot) func() {
	t.Helper()

	previous := collectRepositoryContext
	collectRepositoryContext = func(ctx context.Context, opts repoctx.Options) (repoctx.Snapshot, error) {
		return snapshot, nil
	}
	return func() { collectRepositoryContext = previous }
}

func sampleSnapshot() repoctx.Snapshot {
	return repoctx.Snapshot{
		Root:        "C:\\repo",
		GeneratedAt: time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC),
		KeyFiles: []repoctx.KeyFile{
			{Path: "README.md", Content: "# Repo"},
		},
		Git: repoctx.GitInfo{
			Available: true,
			Status:    "## main",
		},
		DirectoryStructure: []repoctx.TreeEntry{
			{Path: "README.md", Type: "file", Depth: 1},
		},
		Languages: []repoctx.LanguageInfo{
			{Name: "Go", Files: 2, Bytes: 200},
		},
		Frameworks:       []string{"cobra"},
		ExcludedPatterns: []string{".env"},
	}
}
