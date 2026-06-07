package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	appconfig "ai-harness/internal/config"
	"ai-harness/internal/providers"
)

func TestModelsListCommand(t *testing.T) {
	restore := stubLMStudioProvider(t, fakeLocalProvider{
		models: []providers.Model{{ID: "model-a"}, {ID: "model-b"}},
	})
	defer restore()

	path := writeLocalConfig(t, "http://127.0.0.1:1234/v1")
	cmd := NewRootCommand("test")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"models", "list", "--config", path, "--timeout", "5s"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("models list failed: %v\n%s", err, out.String())
	}

	got := out.String()
	for _, want := range []string{"Provider: lmstudio/desktop", "model-a", "model-b"} {
		if !strings.Contains(got, want) {
			t.Fatalf("models output missing %q:\n%s", want, got)
		}
	}
}

func TestAskLocalCommand(t *testing.T) {
	restore := stubLMStudioProvider(t, fakeLocalProvider{
		response: providers.AskResponse{
			Model:   "model-a",
			Content: "This is a test answer.",
		},
	})
	defer restore()

	path := writeLocalConfig(t, "http://127.0.0.1:1234/v1")
	cmd := NewRootCommand("test")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"ask-local", "--config", path, "--timeout", "5s", "Explain", "this", "repo"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("ask-local failed: %v\n%s", err, out.String())
	}

	got := out.String()
	for _, want := range []string{"Provider: lmstudio/desktop", "Model: model-a", "This is a test answer."} {
		if !strings.Contains(got, want) {
			t.Fatalf("ask-local output missing %q:\n%s", want, got)
		}
	}

	records := historyRecordsForConfig(t, path)
	if len(records) != 1 || records[0].Command != "ask-local" || records[0].Provider != "lmstudio" || records[0].Model != "model-a" {
		t.Fatalf("history records = %+v", records)
	}
}

func TestLoadLMStudioProviderReportsAvailableProviders(t *testing.T) {
	path := writeLocalConfig(t, "http://127.0.0.1:1234/v1")
	_, err := loadLMStudioProvider(localProviderOptions{
		configPath: path,
		provider:   "missing",
	})
	if err == nil {
		t.Fatal("load provider succeeded, want error")
	}
	if !strings.Contains(err.Error(), "available providers: desktop") {
		t.Fatalf("error missing available providers: %v", err)
	}
}

func writeLocalConfig(t *testing.T, baseURL string) string {
	t.Helper()

	cfg := appconfig.Default()
	provider := cfg.Providers["lmstudio"]["desktop"]
	provider.BaseURL = baseURL
	cfg.Providers["lmstudio"]["desktop"] = provider

	contents, err := appconfig.Marshal(cfg)
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

func overwriteCLIFile(path string, contents []byte) error {
	return os.WriteFile(path, contents, 0o600)
}

func stubLMStudioProvider(t *testing.T, provider providers.Provider) func() {
	t.Helper()

	previous := newLMStudioProvider
	newLMStudioProvider = func(name string, cfg appconfig.Provider) (providers.Provider, error) {
		return provider, nil
	}
	return func() { newLMStudioProvider = previous }
}

type fakeLocalProvider struct {
	models   []providers.Model
	response providers.AskResponse
}

func (p fakeLocalProvider) Name() string {
	return "desktop"
}

func (p fakeLocalProvider) Health(context.Context) error {
	return nil
}

func (p fakeLocalProvider) ListModels(context.Context) ([]providers.Model, error) {
	return p.models, nil
}

func (p fakeLocalProvider) Ask(context.Context, providers.AskRequest) (providers.AskResponse, error) {
	return p.response, nil
}
