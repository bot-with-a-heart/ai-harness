package cli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ai-harness/internal/catalog"
)

func TestModelsCatalogShowCommand(t *testing.T) {
	path := writeTestCatalog(t)

	cmd := NewRootCommand("test")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"models", "catalog", "show", "--catalog", path})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("catalog show failed: %v\n%s", err, out.String())
	}

	got := out.String()
	for _, want := range []string{"Updated:", "qwen/qwen3-coder-next", "large refactors"} {
		if !strings.Contains(got, want) {
			t.Fatalf("show output missing %q:\n%s", want, got)
		}
	}
}

func TestModelsCatalogExplainCommand(t *testing.T) {
	path := writeTestCatalog(t)

	cmd := NewRootCommand("test")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"models", "catalog", "explain", "--catalog", path, "qwen"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("catalog explain failed: %v\n%s", err, out.String())
	}

	got := out.String()
	for _, want := range []string{"Qwen3 Coder Next", "Hardware fit: fits", "Best for:", "JSON:"} {
		if !strings.Contains(got, want) {
			t.Fatalf("explain output missing %q:\n%s", want, got)
		}
	}
}

func writeTestCatalog(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "model-catalog.json")
	err := catalog.Save(path, catalog.Catalog{
		UpdatedAt: time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC),
		Source:    "lmstudio",
		Models: []catalog.Model{
			{
				ID:                     "qwen/qwen3-coder-next",
				ModelKey:               "qwen/qwen3-coder-next",
				DisplayName:            "Qwen3 Coder Next",
				Type:                   "llm",
				Downloaded:             true,
				Loaded:                 true,
				Architecture:           "qwen3next",
				Params:                 "80B",
				MaxContextLength:       262144,
				EstimatedGPUMemoryGB:   48,
				EstimatedTotalMemoryGB: 48,
				HardwareFit:            "fits",
				SpeedClass:             "slow",
				ComplexityClass:        "high",
				BestFor:                []string{"code generation", "large refactors"},
				AvoidFor:               []string{"quick answers when a smaller model is sufficient"},
				Notes:                  "slow speed; high complexity; fits hardware fit.",
			},
		},
	})
	if err != nil {
		t.Fatalf("save test catalog: %v", err)
	}

	return path
}
