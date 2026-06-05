package cli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	appconfig "ai-harness/internal/config"
)

func TestConfigInitAndShowCommands(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")

	initCmd := NewRootCommand("test")
	var initOut bytes.Buffer
	initCmd.SetOut(&initOut)
	initCmd.SetErr(&initOut)
	initCmd.SetArgs([]string{"config", "--path", path, "init"})
	if err := initCmd.Execute(); err != nil {
		t.Fatalf("config init failed: %v\n%s", err, initOut.String())
	}
	if !strings.Contains(initOut.String(), "Created config at") {
		t.Fatalf("unexpected init output:\n%s", initOut.String())
	}

	showCmd := NewRootCommand("test")
	var showOut bytes.Buffer
	showCmd.SetOut(&showOut)
	showCmd.SetErr(&showOut)
	showCmd.SetArgs([]string{"config", "--path", path, "show"})
	if err := showCmd.Execute(); err != nil {
		t.Fatalf("config show failed: %v\n%s", err, showOut.String())
	}

	got := showOut.String()
	for _, want := range []string{"default_mode = 'auto'", "[providers.lmstudio.desktop]", "[routing]"} {
		if !strings.Contains(got, want) {
			t.Fatalf("show output missing %q:\n%s", want, got)
		}
	}
}

func TestPrintDoctorReport(t *testing.T) {
	var out bytes.Buffer
	printDoctorReport(&out, appconfig.DoctorReport{
		Checks: []appconfig.Check{
			{Name: "config exists", Status: appconfig.CheckPass, Message: "ok"},
			{Name: "LM Studio endpoint reachable", Status: appconfig.CheckFail, Message: "nope"},
		},
	})

	got := out.String()
	for _, want := range []string{"PASS config exists - ok", "FAIL LM Studio endpoint reachable - nope"} {
		if !strings.Contains(got, want) {
			t.Fatalf("doctor report missing %q:\n%s", want, got)
		}
	}
}
