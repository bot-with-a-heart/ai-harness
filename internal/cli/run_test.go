package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	appconfig "ai-harness/internal/config"
	"ai-harness/internal/patch"
	"ai-harness/internal/providers"
)

func TestRunCommandRoutesToLocalProvider(t *testing.T) {
	restoreLocal := stubLMStudioProvider(t, fakeLocalProvider{
		response: providers.AskResponse{Model: "local-model", Content: "local result"},
	})
	defer restoreLocal()
	restoreCodex := stubCodexProvider(t, fakeLocalProvider{
		response: providers.AskResponse{Model: "codex-model", Content: "codex result"},
	})
	defer restoreCodex()

	path := writeLocalConfig(t, "http://127.0.0.1:1234/v1")
	cmd := NewRootCommand("test")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"run", "--config", path, "--heuristic", "Explain", "this", "repository"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("run command failed: %v\n%s", err, out.String())
	}

	got := out.String()
	for _, want := range []string{"Provider Selected: lmstudio", "Fallback Provider: codex", "local result"} {
		if !strings.Contains(got, want) {
			t.Fatalf("run output missing %q:\n%s", want, got)
		}
	}
}

func TestRunCommandRoutesToCodexProvider(t *testing.T) {
	restoreLocal := stubLMStudioProvider(t, fakeLocalProvider{
		response: providers.AskResponse{Model: "local-model", Content: "local result"},
	})
	defer restoreLocal()
	restoreCodex := stubCodexProvider(t, fakeLocalProvider{
		response: providers.AskResponse{Model: "codex-model", Content: "codex result"},
	})
	defer restoreCodex()

	path := writeLocalConfig(t, "http://127.0.0.1:1234/v1")
	cmd := NewRootCommand("test")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"run", "--config", path, "--heuristic", "Refactor", "my", "AWS", "CDK", "application"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("run command failed: %v\n%s", err, out.String())
	}

	got := out.String()
	for _, want := range []string{"Provider Selected: codex", "Fallback Provider: lmstudio", "codex result"} {
		if !strings.Contains(got, want) {
			t.Fatalf("run output missing %q:\n%s", want, got)
		}
	}
}

func TestRunEditDeclineDoesNotApplyPatch(t *testing.T) {
	root := writeEditRepo(t)
	restoreLocal := stubLMStudioProvider(t, fakeLocalProvider{
		response: providers.AskResponse{Model: "local-model", Content: testUnifiedDiff()},
	})
	defer restoreLocal()
	restoreCodex := stubCodexProvider(t, fakeLocalProvider{
		response: providers.AskResponse{Model: "codex-model", Content: testUnifiedDiff()},
	})
	defer restoreCodex()
	runner := &fakeEditRunner{}
	restoreRunner := stubPatchRunner(runner)
	defer restoreRunner()

	path := writeLocalConfig(t, "http://127.0.0.1:1234/v1")
	cmd := NewRootCommand("test")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetIn(strings.NewReader("no\n"))
	cmd.SetArgs([]string{"run", "--config", path, "--edit", "--heuristic", "--cd", root, "Add", "unit", "tests"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("run --edit failed: %v\n%s", err, out.String())
	}

	got := out.String()
	for _, want := range []string{"Generated Patch:", "Apply patch? Type 'yes' to apply:", "Patch not applied."} {
		if !strings.Contains(got, want) {
			t.Fatalf("output missing %q:\n%s", want, got)
		}
	}
	if len(runner.commands) != 0 {
		t.Fatalf("patch runner was called after declined approval: %+v", runner.commands)
	}
}

func TestRunEditApprovalAppliesPatchAndRunsTests(t *testing.T) {
	root := writeEditRepo(t)
	restoreLocal := stubLMStudioProvider(t, fakeLocalProvider{
		response: providers.AskResponse{Model: "local-model", Content: testUnifiedDiff()},
	})
	defer restoreLocal()
	restoreCodex := stubCodexProvider(t, fakeLocalProvider{
		response: providers.AskResponse{Model: "codex-model", Content: testUnifiedDiff()},
	})
	defer restoreCodex()
	runner := &fakeEditRunner{}
	restoreRunner := stubPatchRunner(runner)
	defer restoreRunner()

	path := writeLocalConfig(t, "http://127.0.0.1:1234/v1")
	cmd := NewRootCommand("test")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetIn(strings.NewReader("yes\n"))
	cmd.SetArgs([]string{"run", "--config", path, "--edit", "--heuristic", "--cd", root, "--test-command", "echo ok", "Add", "unit", "tests"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("run --edit failed: %v\n%s", err, out.String())
	}

	got := out.String()
	for _, want := range []string{"Patch applied.", "Tests: echo ok", "Tests passed."} {
		if !strings.Contains(got, want) {
			t.Fatalf("output missing %q:\n%s", want, got)
		}
	}
	joined := strings.Join(runner.commands, "|")
	for _, want := range []string{"git apply --check -", "git apply -", "echo ok"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("commands missing %q: %+v", want, runner.commands)
		}
	}
}

func TestRunLocalFirstCompletesAfterLocalTestsPass(t *testing.T) {
	root := writeEditRepo(t)
	local := &recordingProvider{
		name:     "desktop",
		response: providers.AskResponse{Model: "local-model", Content: testUnifiedDiff()},
	}
	codex := &recordingProvider{
		name:     "codex",
		response: providers.AskResponse{Model: "codex-model", Content: testUnifiedDiff()},
	}
	restoreLocal := stubLMStudioProvider(t, local)
	defer restoreLocal()
	restoreCodex := stubCodexProvider(t, codex)
	defer restoreCodex()
	runner := &scriptedEditRunner{
		testResults: []scriptedRunnerResult{{stdout: "ok"}},
	}
	restoreRunner := stubPatchRunner(runner)
	defer restoreRunner()

	path := writeLocalConfig(t, "http://127.0.0.1:1234/v1")
	cmd := NewRootCommand("test")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetIn(strings.NewReader("yes\n"))
	cmd.SetArgs([]string{"run", "--config", path, "--local-first", "--heuristic", "--cd", root, "--test-command", "echo ok", "Add", "unit", "tests"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("run --local-first failed: %v\n%s", err, out.String())
	}

	got := out.String()
	for _, want := range []string{"Local-first attempt: lmstudio", "Local attempt completed successfully.", "Tests passed."} {
		if !strings.Contains(got, want) {
			t.Fatalf("output missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "Escalating to Codex") {
		t.Fatalf("unexpected escalation:\n%s", got)
	}
	if local.calls != 1 || codex.calls != 0 {
		t.Fatalf("provider calls: local=%d codex=%d", local.calls, codex.calls)
	}
}

func TestRunLocalFirstEscalatesWhenLocalPatchIsInvalid(t *testing.T) {
	root := writeEditRepo(t)
	local := &recordingProvider{
		name:     "desktop",
		response: providers.AskResponse{Model: "local-model", Content: "not a diff"},
	}
	codex := &recordingProvider{
		name:     "codex",
		response: providers.AskResponse{Model: "codex-model", Content: testUnifiedDiff()},
	}
	restoreLocal := stubLMStudioProvider(t, local)
	defer restoreLocal()
	restoreCodex := stubCodexProvider(t, codex)
	defer restoreCodex()
	runner := &scriptedEditRunner{
		testResults: []scriptedRunnerResult{{stdout: "ok"}},
	}
	restoreRunner := stubPatchRunner(runner)
	defer restoreRunner()

	path := writeLocalConfig(t, "http://127.0.0.1:1234/v1")
	cmd := NewRootCommand("test")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetIn(strings.NewReader("yes\n"))
	cmd.SetArgs([]string{"run", "--config", path, "--local-first", "--heuristic", "--cd", root, "--test-command", "echo ok", "Add", "unit", "tests"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("run --local-first failed: %v\n%s", err, out.String())
	}

	got := out.String()
	for _, want := range []string{"Local attempt failed validation:", "Escalating to Codex.", "Provider Selected: codex", "Codex escalation completed successfully."} {
		if !strings.Contains(got, want) {
			t.Fatalf("output missing %q:\n%s", want, got)
		}
	}
	if local.calls != 1 || codex.calls != 1 {
		t.Fatalf("provider calls: local=%d codex=%d", local.calls, codex.calls)
	}
	if len(codex.requests) != 1 || !strings.Contains(codex.requests[0].Prompt, "Escalation context:") {
		t.Fatalf("codex prompt missing escalation context:\n%s", codex.requests[0].Prompt)
	}
}

func TestRunLocalFirstEscalatesWhenLocalTestsFail(t *testing.T) {
	root := writeEditRepo(t)
	local := &recordingProvider{
		name:     "desktop",
		response: providers.AskResponse{Model: "local-model", Content: testUnifiedDiff()},
	}
	codex := &recordingProvider{
		name:     "codex",
		response: providers.AskResponse{Model: "codex-model", Content: testUnifiedDiff()},
	}
	restoreLocal := stubLMStudioProvider(t, local)
	defer restoreLocal()
	restoreCodex := stubCodexProvider(t, codex)
	defer restoreCodex()
	runner := &scriptedEditRunner{
		testResults: []scriptedRunnerResult{
			{stderr: "local tests failed", err: errors.New("exit 1")},
			{stdout: "ok"},
		},
	}
	restoreRunner := stubPatchRunner(runner)
	defer restoreRunner()

	path := writeLocalConfig(t, "http://127.0.0.1:1234/v1")
	cmd := NewRootCommand("test")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetIn(strings.NewReader("yes\nyes\n"))
	cmd.SetArgs([]string{"run", "--config", path, "--local-first", "--heuristic", "--cd", root, "--test-command", "echo ok", "Add", "unit", "tests"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("run --local-first failed: %v\n%s", err, out.String())
	}

	got := out.String()
	for _, want := range []string{"local tests failed", "Escalating to Codex.", "Codex escalation completed successfully."} {
		if !strings.Contains(got, want) {
			t.Fatalf("output missing %q:\n%s", want, got)
		}
	}
	if local.calls != 1 || codex.calls != 1 {
		t.Fatalf("provider calls: local=%d codex=%d", local.calls, codex.calls)
	}
}

func TestConfirmPatchAcceptsPowerShellPipelineEncoding(t *testing.T) {
	cases := []string{
		"\ufeffyes\n\r\n",
		"y\x00e\x00s\x00\n\x00",
	}
	for _, input := range cases {
		if !confirmPatch(strings.NewReader(input), io.Discard) {
			t.Fatalf("confirmPatch rejected encoded yes input %q", input)
		}
	}
}

func stubCodexProvider(t *testing.T, provider providers.Provider) func() {
	t.Helper()

	previous := newCodexProvider
	newCodexProvider = func(name string, cfg appconfig.Provider, workingDir string, sandbox string) (providers.Provider, error) {
		return provider, nil
	}
	return func() { newCodexProvider = previous }
}

func stubPatchRunner(runner patch.Runner) func() {
	previous := runPatchRunner
	runPatchRunner = runner
	return func() { runPatchRunner = previous }
}

func writeEditRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# Test\n"), 0o600); err != nil {
		t.Fatalf("write README: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module test\n"), 0o600); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	return root
}

func testUnifiedDiff() string {
	return "diff --git a/README.md b/README.md\n--- a/README.md\n+++ b/README.md\n@@ -1 +1,2 @@\n # Test\n+Edited\n"
}

type fakeEditRunner struct {
	commands []string
	stdin    []string
}

func (r *fakeEditRunner) Run(ctx context.Context, dir string, name string, args []string, stdin string) ([]byte, []byte, error) {
	r.commands = append(r.commands, name+" "+strings.Join(args, " "))
	r.stdin = append(r.stdin, stdin)
	return []byte("ok"), nil, nil
}

type recordingProvider struct {
	name     string
	response providers.AskResponse
	err      error
	calls    int
	requests []providers.AskRequest
}

func (p *recordingProvider) Name() string {
	return p.name
}

func (p *recordingProvider) Health(context.Context) error {
	return nil
}

func (p *recordingProvider) ListModels(context.Context) ([]providers.Model, error) {
	return nil, nil
}

func (p *recordingProvider) Ask(ctx context.Context, req providers.AskRequest) (providers.AskResponse, error) {
	p.calls++
	p.requests = append(p.requests, req)
	return p.response, p.err
}

type scriptedRunnerResult struct {
	stdout string
	stderr string
	err    error
}

type scriptedEditRunner struct {
	commands    []string
	stdin       []string
	testResults []scriptedRunnerResult
}

func (r *scriptedEditRunner) Run(ctx context.Context, dir string, name string, args []string, stdin string) ([]byte, []byte, error) {
	command := name + " " + strings.Join(args, " ")
	r.commands = append(r.commands, command)
	r.stdin = append(r.stdin, stdin)
	if strings.HasPrefix(command, "git apply") {
		return []byte("ok"), nil, nil
	}
	if len(r.testResults) == 0 {
		return []byte("ok"), nil, nil
	}
	result := r.testResults[0]
	r.testResults = r.testResults[1:]
	return []byte(result.stdout), []byte(result.stderr), result.err
}
