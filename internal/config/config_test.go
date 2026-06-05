package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitLoadAndRead(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")

	if err := Init(path, InitOptions{}); err != nil {
		t.Fatalf("init config: %v", err)
	}

	if _, err := os.Stat(HistoryDirForConfigPath(path)); err != nil {
		t.Fatalf("history directory was not created: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.DefaultMode != "auto" {
		t.Fatalf("default mode = %q, want auto", cfg.DefaultMode)
	}
	if cfg.Providers["lmstudio"]["desktop"].BaseURL == "" {
		t.Fatal("lmstudio desktop base URL was empty")
	}

	contents, err := Read(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if !strings.Contains(string(contents), "[providers.lmstudio.desktop]") {
		t.Fatalf("config contents missing lmstudio provider:\n%s", contents)
	}
}

func TestInitDoesNotOverwriteByDefault(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := Init(path, InitOptions{}); err != nil {
		t.Fatalf("init config: %v", err)
	}

	if err := Init(path, InitOptions{}); err == nil {
		t.Fatal("second init succeeded, want overwrite protection error")
	}
}

func TestValidateRequiresLMStudioBaseURL(t *testing.T) {
	cfg := Default()
	provider := cfg.Providers["lmstudio"]["desktop"]
	provider.BaseURL = ""
	cfg.Providers["lmstudio"]["desktop"] = provider

	if err := Validate(cfg); err == nil {
		t.Fatal("validate succeeded without lmstudio base_url")
	}
}
