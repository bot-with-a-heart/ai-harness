package patch

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	repoctx "ai-harness/internal/context"
)

func TestExtractUnifiedDiffFromFence(t *testing.T) {
	raw := "Here is the patch:\n```diff\ndiff --git a/a.txt b/a.txt\n--- a/a.txt\n+++ b/a.txt\n@@ -1 +1 @@\n-old\n+new\n```\n"

	diff, err := ExtractUnifiedDiff(raw)
	if err != nil {
		t.Fatalf("extract diff: %v", err)
	}
	if !strings.Contains(diff, "diff --git") || strings.Contains(diff, "```") {
		t.Fatalf("unexpected diff:\n%s", diff)
	}
}

func TestExtractUnifiedDiffRejectsNonDiff(t *testing.T) {
	if _, err := ExtractUnifiedDiff("I would edit the file."); err == nil {
		t.Fatal("extract succeeded, want error")
	}
}

func TestBuildEscalationPromptIncludesFailureAndDiff(t *testing.T) {
	prompt := BuildEscalationPrompt("Fix tests", testSnapshotWithDiff(), "local tests failed")

	for _, want := range []string{"Escalation context:", "local tests failed", "Git diff:", "+Edited"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q:\n%s", want, prompt)
		}
	}
}

func TestApplyChecksThenAppliesPatch(t *testing.T) {
	runner := &fakePatchRunner{}
	diff := "diff --git a/a.txt b/a.txt\n--- a/a.txt\n+++ b/a.txt\n@@ -1 +1 @@\n-old\n+new\n"

	if err := Apply(context.Background(), "C:\\repo", diff, runner); err != nil {
		t.Fatalf("apply: %v", err)
	}

	want := []string{"git apply --check -", "git apply -"}
	if strings.Join(runner.commands, "|") != strings.Join(want, "|") {
		t.Fatalf("commands = %+v", runner.commands)
	}
	if runner.stdin[0] != diff || runner.stdin[1] != diff {
		t.Fatalf("stdin = %+v", runner.stdin)
	}
}

func TestApplyWithExecRunnerChangesFile(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git executable not available")
	}

	root := t.TempDir()
	writePatchTestFile(t, root, "README.md", "# Test\n")
	gitInit := exec.Command("git", "init")
	gitInit.Dir = root
	if output, err := gitInit.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, output)
	}

	if err := Apply(context.Background(), root, testREADMEPatch(), ExecRunner{}); err != nil {
		t.Fatalf("apply: %v", err)
	}

	contents, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		t.Fatalf("read README: %v", err)
	}
	if strings.ReplaceAll(string(contents), "\r\n", "\n") != "# Test\nEdited\n" {
		t.Fatalf("README contents = %q", contents)
	}
}

func TestRunTestsAutoDetectsGo(t *testing.T) {
	root := t.TempDir()
	writePatchTestFile(t, root, "go.mod", "module test\n")
	runner := &fakePatchRunner{}

	result, err := RunTests(context.Background(), root, "auto", runner)
	if err != nil {
		t.Fatalf("run tests: %v", err)
	}
	if !result.Passed || result.Command != "go test ./..." {
		t.Fatalf("result = %+v", result)
	}
	if len(runner.commands) != 1 || !strings.Contains(runner.commands[0], "go test ./...") {
		t.Fatalf("commands = %+v", runner.commands)
	}
}

func TestRunTestsReturnsFailureOutput(t *testing.T) {
	runner := &fakePatchRunner{err: errors.New("exit 1"), stderr: "failed"}
	result, err := RunTests(context.Background(), "C:\\repo", "go test ./...", runner)
	if err == nil {
		t.Fatal("run tests succeeded, want error")
	}
	if result.Passed || !strings.Contains(result.Output, "failed") {
		t.Fatalf("result = %+v", result)
	}
}

func writePatchTestFile(t *testing.T, root string, rel string, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func testREADMEPatch() string {
	return "diff --git a/README.md b/README.md\n--- a/README.md\n+++ b/README.md\n@@ -1 +1,2 @@\n # Test\n+Edited\n"
}

func testSnapshotWithDiff() repoctx.Snapshot {
	return repoctx.Snapshot{
		Root: "C:\\repo",
		Git: repoctx.GitInfo{
			Available: true,
			Status:    "## main",
			Diff:      "diff --git a/README.md b/README.md\n+Edited\n",
		},
	}
}

type fakePatchRunner struct {
	commands []string
	stdin    []string
	stdout   string
	stderr   string
	err      error
}

func (r *fakePatchRunner) Run(ctx context.Context, dir string, name string, args []string, stdin string) ([]byte, []byte, error) {
	r.commands = append(r.commands, name+" "+strings.Join(args, " "))
	r.stdin = append(r.stdin, stdin)
	stdout := r.stdout
	if stdout == "" {
		stdout = "ok"
	}
	return []byte(stdout), []byte(r.stderr), r.err
}
