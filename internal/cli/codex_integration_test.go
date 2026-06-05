package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestAskCodexIntegration(t *testing.T) {
	if os.Getenv("AI_HARNESS_INTEGRATION_CODEX") != "1" {
		t.Skip("set AI_HARNESS_INTEGRATION_CODEX=1 to run the real Codex CLI integration test")
	}

	cmd := NewRootCommand("test")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"ask-codex", "--timeout", "5m", "Reply with exactly: codex-ok"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("ask-codex integration failed: %v\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "codex-ok") {
		t.Fatalf("ask-codex output missing codex-ok:\n%s", out.String())
	}
}
