package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ai-harness/internal/catalog"
	appconfig "ai-harness/internal/config"
)

func TestMemoryObsidianStatusDisabled(t *testing.T) {
	path := writeLocalConfig(t, "http://127.0.0.1:1234/v1")
	cmd := NewRootCommand("test")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"memory", "--config", path, "obsidian", "status"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("status failed: %v\n%s", err, out.String())
	}

	got := out.String()
	for _, want := range []string{"Obsidian integration: disabled", "Vault path: -", "Folder: AI Harness"} {
		if !strings.Contains(got, want) {
			t.Fatalf("status missing %q:\n%s", want, got)
		}
	}
}

func TestMemoryObsidianInitWritesConfig(t *testing.T) {
	path := writeLocalConfig(t, "http://127.0.0.1:1234/v1")
	vault := t.TempDir()
	cmd := NewRootCommand("test")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"memory", "--config", path, "obsidian", "init", "--vault", vault, "--folder", "Harness Notes"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("init failed: %v\n%s", err, out.String())
	}

	cfg, err := appconfig.Load(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if !cfg.Memory.Obsidian.Enabled || cfg.Memory.Obsidian.VaultPath != vault || cfg.Memory.Obsidian.Folder != "Harness Notes" {
		t.Fatalf("obsidian config = %+v", cfg.Memory.Obsidian)
	}
	if _, err := os.Stat(filepath.Join(vault, "Harness Notes")); err != nil {
		t.Fatalf("export folder was not created: %v", err)
	}
}

func TestMemoryObsidianExportDryRun(t *testing.T) {
	path := writeLocalConfig(t, "http://127.0.0.1:1234/v1")
	vault := t.TempDir()
	catalogPath := writeObsidianTestCatalog(t)

	cmd := NewRootCommand("test")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"memory", "--config", path, "obsidian", "export", "--vault", vault, "--catalog", catalogPath, "--dry-run"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("export dry-run failed: %v\n%s", err, out.String())
	}
	got := out.String()
	for _, want := range []string{"Dry run: no files written.", "AI Harness/Index.md", "AI Harness/Models/Model Catalog.md"} {
		if !strings.Contains(got, want) {
			t.Fatalf("dry-run missing %q:\n%s", want, got)
		}
	}
	if _, err := os.Stat(filepath.Join(vault, "AI Harness")); !os.IsNotExist(err) {
		t.Fatalf("dry-run created files: %v", err)
	}
}

func TestMemoryObsidianExportWritesManagedNotes(t *testing.T) {
	path := writeLocalConfig(t, "http://127.0.0.1:1234/v1")
	vault := t.TempDir()
	catalogPath := writeObsidianTestCatalog(t)

	cmd := NewRootCommand("test")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"memory", "--config", path, "obsidian", "export", "--vault", vault, "--catalog", catalogPath})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("export failed: %v\n%s", err, out.String())
	}
	contents, err := os.ReadFile(filepath.Join(vault, "AI Harness", "Models", "Model Catalog.md"))
	if err != nil {
		t.Fatalf("read exported note: %v", err)
	}
	if !strings.Contains(string(contents), "ai_harness_managed: true") || !strings.Contains(string(contents), "model-a") {
		t.Fatalf("unexpected exported note:\n%s", contents)
	}
}

func TestMemoryObsidianExportRefusesUserEditedConflict(t *testing.T) {
	path := writeLocalConfig(t, "http://127.0.0.1:1234/v1")
	vault := t.TempDir()
	catalogPath := writeObsidianTestCatalog(t)
	indexPath := filepath.Join(vault, "AI Harness", "Index.md")
	if err := os.MkdirAll(filepath.Dir(indexPath), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(indexPath, []byte("# edited by user\n"), 0o600); err != nil {
		t.Fatalf("write conflict: %v", err)
	}

	cmd := NewRootCommand("test")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"memory", "--config", path, "obsidian", "export", "--vault", vault, "--catalog", catalogPath})

	if err := cmd.Execute(); err == nil {
		t.Fatalf("export succeeded despite conflict:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "conflict") {
		t.Fatalf("conflict was not reported:\n%s", out.String())
	}
}

func writeObsidianTestCatalog(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "model-catalog.json")
	err := catalog.Save(path, catalog.Catalog{
		UpdatedAt: time.Date(2026, 6, 6, 12, 0, 0, 0, time.UTC),
		Source:    "test",
		Models: []catalog.Model{{
			ID:              "model-a",
			Loaded:          true,
			HardwareFit:     "fits",
			SpeedClass:      "fast",
			ComplexityClass: "medium",
			BestFor:         []string{"code review"},
		}},
	})
	if err != nil {
		t.Fatalf("save catalog: %v", err)
	}
	return path
}
