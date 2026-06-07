package obsidian

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ai-harness/internal/catalog"
	"ai-harness/internal/history"
)

func TestBuildPlanIncludesManagedNotes(t *testing.T) {
	now := time.Date(2026, 6, 6, 12, 0, 0, 0, time.UTC)
	plan, err := BuildPlan(ExportOptions{
		VaultPath: t.TempDir(),
		Folder:    "Harness",
		Catalog: &catalog.Catalog{
			UpdatedAt: now,
			Source:    "test",
			Models: []catalog.Model{{
				ID:              "qwen",
				Loaded:          true,
				HardwareFit:     "fits",
				SpeedClass:      "fast",
				ComplexityClass: "medium",
				BestFor:         []string{"tests"},
			}},
		},
		IncludeHistory: true,
		History: []history.Record{{
			Timestamp: now,
			Command:   "run",
			Task:      "secret task",
			Provider:  "lmstudio",
			Status:    "completed",
		}},
		Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("build plan: %v", err)
	}
	if len(plan.Files) != 3 {
		t.Fatalf("files = %+v", plan.Files)
	}
	got := strings.Join([]string{plan.Files[0].RelativePath, plan.Files[1].RelativePath, plan.Files[2].RelativePath}, "|")
	for _, want := range []string{"Harness/Index.md", "Harness/Models/Model Catalog.md", "Harness/History/Recent Task Outcomes.md"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q from %q", want, got)
		}
	}
	if !strings.Contains(plan.Files[2].Contents, "Task: redacted") || strings.Contains(plan.Files[2].Contents, "secret task") {
		t.Fatalf("history task redaction failed:\n%s", plan.Files[2].Contents)
	}
}

func TestWritePlanDetectsUserEditedConflict(t *testing.T) {
	vault := t.TempDir()
	path := filepath.Join(vault, "AI Harness", "Index.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte("# user note\n"), 0o600); err != nil {
		t.Fatalf("write conflict: %v", err)
	}

	plan, err := BuildPlan(ExportOptions{VaultPath: vault})
	if err != nil {
		t.Fatalf("build plan: %v", err)
	}
	if !HasConflicts(plan) {
		t.Fatalf("plan should have conflict: %+v", plan.Files)
	}
	if err := WritePlan(plan, false); err == nil {
		t.Fatal("write succeeded despite conflict")
	}
}

func TestWritePlanWritesManagedFiles(t *testing.T) {
	vault := t.TempDir()
	plan, err := BuildPlan(ExportOptions{
		VaultPath: vault,
		Catalog: &catalog.Catalog{
			Source: "test",
			Models: []catalog.Model{{ID: "model-a"}},
		},
	})
	if err != nil {
		t.Fatalf("build plan: %v", err)
	}
	if err := WritePlan(plan, false); err != nil {
		t.Fatalf("write plan: %v", err)
	}
	contents, err := os.ReadFile(filepath.Join(vault, "AI Harness", "Models", "Model Catalog.md"))
	if err != nil {
		t.Fatalf("read note: %v", err)
	}
	if !strings.Contains(string(contents), managedMarker) || !strings.Contains(string(contents), "model-a") {
		t.Fatalf("unexpected note:\n%s", contents)
	}
}

func TestBuildPlanRejectsEscapingFolder(t *testing.T) {
	if _, err := BuildPlan(ExportOptions{VaultPath: t.TempDir(), Folder: "../outside"}); err == nil {
		t.Fatal("build plan accepted escaping folder")
	}
}
