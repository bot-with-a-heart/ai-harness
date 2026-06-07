package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	appconfig "ai-harness/internal/config"
	"ai-harness/internal/security"
)

func TestSecurityPassphraseWorkflow(t *testing.T) {
	path := writeLocalConfig(t, "http://127.0.0.1:1234/v1")
	runCLI(t, []string{"classify", "--config", path, "--heuristic", "--summary", "Explain", "this", "repository"})

	historyDir := appconfig.HistoryDirForConfigPath(path)
	plainBefore, err := filepath.Glob(filepath.Join(historyDir, "*.json"))
	if err != nil {
		t.Fatalf("glob plaintext: %v", err)
	}
	if len(plainBefore) != 1 {
		t.Fatalf("plaintext before init = %+v", plainBefore)
	}

	out := runCLI(t, []string{"security", "--config", path, "init", "--provider", "passphrase", "--passphrase", "old-pass", "--required"})
	for _, want := range []string{"Security initialized", "Key provider: passphrase", "Run security export-recovery"} {
		if !strings.Contains(out, want) {
			t.Fatalf("security init output missing %q:\n%s", want, out)
		}
	}

	statusJSON := runCLI(t, []string{"security", "--config", path, "status", "--json"})
	var status securityStatus
	if err := json.Unmarshal([]byte(statusJSON), &status); err != nil {
		t.Fatalf("decode status: %v\n%s", err, statusJSON)
	}
	if !status.Enabled || !status.Required || !status.Initialized || status.KeyAvailable {
		t.Fatalf("status = %+v", status)
	}
	cmd := NewRootCommand("test")
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"security", "--config", path, "init", "--provider", "passphrase", "--passphrase", "new-pass", "--force"})
	if err := cmd.Execute(); err == nil {
		t.Fatalf("force init unexpectedly replaced initialized security:\n%s", buf.String())
	}

	out = runCLI(t, []string{"security", "--config", path, "migrate", "--passphrase", "old-pass"})
	if !strings.Contains(out, "Encrypted records: 1") || !strings.Contains(out, "Plaintext records: 0") {
		t.Fatalf("migrate output:\n%s", out)
	}

	verifyJSON := runCLI(t, []string{"security", "--config", path, "verify", "--passphrase", "old-pass", "--json"})
	var report security.HistoryReport
	if err := json.Unmarshal([]byte(verifyJSON), &report); err != nil {
		t.Fatalf("decode verify: %v\n%s", err, verifyJSON)
	}
	if report.Encrypted != 1 || report.Plaintext != 0 || report.Invalid != 0 {
		t.Fatalf("report = %+v", report)
	}
	plainAfter, err := filepath.Glob(filepath.Join(historyDir, "*.json"))
	if err != nil {
		t.Fatalf("glob plaintext after: %v", err)
	}
	if len(plainAfter) != 0 {
		t.Fatalf("plaintext after migrate = %+v", plainAfter)
	}

	t.Setenv(security.PassphraseEnv, "old-pass")
	out = runCLI(t, []string{"history", "--config", path, "list"})
	if !strings.Contains(out, "classify") {
		t.Fatalf("history list output:\n%s", out)
	}
}

func TestSecurityRecoveryAndRotatePassphrase(t *testing.T) {
	path := writeLocalConfig(t, "http://127.0.0.1:1234/v1")
	runCLI(t, []string{"classify", "--config", path, "--heuristic", "--summary", "Explain", "this", "repository"})
	runCLI(t, []string{"security", "--config", path, "init", "--provider", "passphrase", "--passphrase", "old-pass"})
	runCLI(t, []string{"security", "--config", path, "migrate", "--passphrase", "old-pass"})

	recoveryPath := filepath.Join(t.TempDir(), "recovery.json")
	out := runCLI(t, []string{"security", "--config", path, "export-recovery", "--passphrase", "old-pass", "--output", recoveryPath})
	if !strings.Contains(out, "Recovery material written") {
		t.Fatalf("recovery output:\n%s", out)
	}
	recovery, err := os.ReadFile(recoveryPath)
	if err != nil {
		t.Fatalf("read recovery: %v", err)
	}
	if !strings.Contains(string(recovery), `"key"`) {
		t.Fatalf("recovery missing key:\n%s", recovery)
	}
	runCLI(t, []string{"security", "--config", path, "verify", "--recovery-file", recoveryPath})
	runCLI(t, []string{"security", "--config", path, "unlock", "--recovery-file", recoveryPath})

	out = runCLI(t, []string{"security", "--config", path, "rotate-key", "--recovery-file", recoveryPath, "--new-passphrase", "new-pass"})
	if !strings.Contains(out, "Rotated key") {
		t.Fatalf("rotate output:\n%s", out)
	}
	runCLI(t, []string{"security", "--config", path, "verify", "--passphrase", "new-pass"})

	cmd := NewRootCommand("test")
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"security", "--config", path, "verify", "--passphrase", "old-pass"})
	if err := cmd.Execute(); err == nil {
		t.Fatalf("old passphrase still verified:\n%s", buf.String())
	}

	cmd = NewRootCommand("test")
	buf.Reset()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"security", "--config", path, "verify", "--recovery-file", recoveryPath})
	if err := cmd.Execute(); err == nil {
		t.Fatalf("old recovery material still verified after rotation:\n%s", buf.String())
	}
}

func runCLI(t *testing.T, args []string) string {
	t.Helper()

	cmd := NewRootCommand("test")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("command %v failed: %v\n%s", args, err, out.String())
	}
	return out.String()
}
