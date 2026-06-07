package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestVersionCommand(t *testing.T) {
	cmd := NewRootCommand("test")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"version"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("version command failed: %v", err)
	}

	got := out.String()
	want := "ai-harness test\n"
	if got != want {
		t.Fatalf("version output = %q, want %q", got, want)
	}
}

func TestHelpCommand(t *testing.T) {
	cmd := NewRootCommand("test")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("help command failed: %v", err)
	}

	got := out.String()
	for _, want := range []string{"AI Harness routes", "ask-codex", "ask-local", "classify", "config", "context", "history", "memory", "models", "run", "security", "version", "help", "--log-level", "--log-json"} {
		if !strings.Contains(got, want) {
			t.Fatalf("help output missing %q:\n%s", want, got)
		}
	}
}

func TestInvalidLogLevelFails(t *testing.T) {
	cmd := NewRootCommand("test")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--log-level", "chatty", "version"})

	if err := cmd.Execute(); err == nil {
		t.Fatal("command succeeded with invalid log level")
	}
	if !strings.Contains(out.String(), `invalid log level "chatty"`) {
		t.Fatalf("unexpected output:\n%s", out.String())
	}
}
