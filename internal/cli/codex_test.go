package cli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	appconfig "ai-harness/internal/config"
	"ai-harness/internal/providers"
)

func TestAskCodexCommandHelp(t *testing.T) {
	cmd := NewRootCommand("test")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"ask-codex", "--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("ask-codex help failed: %v", err)
	}

	got := out.String()
	for _, want := range []string{"Ask Codex CLI", "--profile", "--sandbox", "--timeout"} {
		if !strings.Contains(got, want) {
			t.Fatalf("help output missing %q:\n%s", want, got)
		}
	}
}

func TestAskCodexCommand(t *testing.T) {
	restore := stubCodexProvider(t, fakeLocalProvider{
		response: providers.AskResponse{
			Model:   "codex-model",
			Content: "This is a Codex test answer.",
		},
	})
	defer restore()

	path := writeCodexConfig(t)
	cmd := NewRootCommand("test")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"ask-codex", "--config", path, "--timeout", "5s", "Review", "this", "repo"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("ask-codex failed: %v\n%s", err, out.String())
	}

	got := out.String()
	for _, want := range []string{"Provider: codex/default", "Model: codex-model", "This is a Codex test answer."} {
		if !strings.Contains(got, want) {
			t.Fatalf("ask-codex output missing %q:\n%s", want, got)
		}
	}

	records := historyRecordsForConfig(t, path)
	if len(records) != 1 || records[0].Command != "ask-codex" || records[0].Provider != "codex" || records[0].Model != "codex-model" {
		t.Fatalf("history records = %+v", records)
	}
}

func TestLoadCodexProviderReportsAvailableProviders(t *testing.T) {
	path := writeCodexConfig(t)
	_, err := loadCodexProvider(codexProviderOptions{
		configPath: path,
		provider:   "missing",
	})
	if err == nil {
		t.Fatal("load provider succeeded, want error")
	}
	if !strings.Contains(err.Error(), "available providers: default") {
		t.Fatalf("error missing available providers: %v", err)
	}
}

func writeCodexConfig(t *testing.T) string {
	t.Helper()

	contents, err := appconfig.Marshal(appconfig.Default())
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}

	path := filepath.Join(t.TempDir(), "config.toml")
	if err := appconfig.Init(path, appconfig.InitOptions{}); err != nil {
		t.Fatalf("init config: %v", err)
	}
	if err := overwriteCLIFile(path, contents); err != nil {
		t.Fatalf("write config: %v", err)
	}

	return path
}
