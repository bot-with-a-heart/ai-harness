package codex

import (
	"context"
	"os"
	"reflect"
	"strings"
	"testing"

	appconfig "ai-harness/internal/config"
	"ai-harness/internal/providers"
)

func TestAskRunsCodexExecAndReadsLastMessage(t *testing.T) {
	runner := &fakeRunner{
		lookPath: "codex",
		runFunc: func(ctx context.Context, name string, args []string, stdin string) ([]byte, []byte, error) {
			if name != "codex" {
				t.Fatalf("name = %q", name)
			}
			if stdin != "" {
				t.Fatalf("stdin = %q, want empty", stdin)
			}
			wantPrefix := []string{"exec", "--cd", "C:\\repo", "--sandbox", "read-only", "--ephemeral", "--color", "never", "--output-last-message"}
			if len(args) < len(wantPrefix)+2 {
				t.Fatalf("args too short: %+v", args)
			}
			if !reflect.DeepEqual(args[:len(wantPrefix)], wantPrefix) {
				t.Fatalf("args prefix = %+v", args[:len(wantPrefix)])
			}
			outputFile := args[len(wantPrefix)]
			if err := os.WriteFile(outputFile, []byte("codex answer"), 0o600); err != nil {
				t.Fatalf("write output file: %v", err)
			}
			if got := args[len(args)-1]; got != "Review this repository" {
				t.Fatalf("prompt arg = %q", got)
			}
			return []byte("progress output"), nil, nil
		},
	}
	client := newTestClient(t, runner)

	response, err := client.Ask(context.Background(), providers.AskRequest{Prompt: "Review this repository"})
	if err != nil {
		t.Fatalf("ask codex: %v", err)
	}
	if response.Content != "codex answer" {
		t.Fatalf("content = %q", response.Content)
	}
	if response.Model != "codex-cli" {
		t.Fatalf("model = %q", response.Model)
	}
}

func TestAskIncludesModelWhenProvided(t *testing.T) {
	runner := &fakeRunner{
		lookPath: "codex",
		runFunc: func(ctx context.Context, name string, args []string, stdin string) ([]byte, []byte, error) {
			joined := strings.Join(args, " ")
			if !strings.Contains(joined, "--model gpt-test") {
				t.Fatalf("args missing model: %+v", args)
			}
			outputFile := ""
			for i, arg := range args {
				if arg == "--output-last-message" && i+1 < len(args) {
					outputFile = args[i+1]
					break
				}
			}
			if outputFile == "" {
				t.Fatalf("args missing output file: %+v", args)
			}
			if err := os.WriteFile(outputFile, []byte("model answer"), 0o600); err != nil {
				t.Fatalf("write output file: %v", err)
			}
			return nil, nil, nil
		},
	}
	client := newTestClient(t, runner)

	response, err := client.Ask(context.Background(), providers.AskRequest{Model: "gpt-test", Prompt: "Hello"})
	if err != nil {
		t.Fatalf("ask codex: %v", err)
	}
	if response.Model != "gpt-test" {
		t.Fatalf("model = %q", response.Model)
	}
}

func TestAskIncludesNamedProfile(t *testing.T) {
	runner := &fakeRunner{
		lookPath: "codex",
		runFunc: func(ctx context.Context, name string, args []string, stdin string) ([]byte, []byte, error) {
			wantPrefix := []string{"exec", "--profile", "work", "--cd", "C:\\repo"}
			if len(args) < len(wantPrefix) || !reflect.DeepEqual(args[:len(wantPrefix)], wantPrefix) {
				t.Fatalf("args prefix = %+v", args)
			}
			outputFile := ""
			for i, arg := range args {
				if arg == "--output-last-message" && i+1 < len(args) {
					outputFile = args[i+1]
					break
				}
			}
			if outputFile == "" {
				t.Fatalf("args missing output file: %+v", args)
			}
			if err := os.WriteFile(outputFile, []byte("profile answer"), 0o600); err != nil {
				t.Fatalf("write output file: %v", err)
			}
			return nil, nil, nil
		},
	}
	client, err := New("default", appconfig.Provider{
		Type:    "codex-cli",
		Profile: "work",
	}, WithRunner(runner), WithWorkingDir("C:\\repo"))
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	response, err := client.Ask(context.Background(), providers.AskRequest{Prompt: "Hello"})
	if err != nil {
		t.Fatalf("ask codex: %v", err)
	}
	if response.Content != "profile answer" {
		t.Fatalf("content = %q", response.Content)
	}
}

func TestAskFallsBackToStdout(t *testing.T) {
	runner := &fakeRunner{
		lookPath: "codex",
		runFunc: func(ctx context.Context, name string, args []string, stdin string) ([]byte, []byte, error) {
			return []byte("stdout answer"), nil, nil
		},
	}
	client := newTestClient(t, runner)

	response, err := client.Ask(context.Background(), providers.AskRequest{Prompt: "Hello"})
	if err != nil {
		t.Fatalf("ask codex: %v", err)
	}
	if response.Content != "stdout answer" {
		t.Fatalf("content = %q", response.Content)
	}
}

func TestAskReturnsCapturedMessageWhenCodexExitsAfterAnswer(t *testing.T) {
	runner := &fakeRunner{
		lookPath: "codex",
		runFunc: func(ctx context.Context, name string, args []string, stdin string) ([]byte, []byte, error) {
			outputFile := ""
			for i, arg := range args {
				if arg == "--output-last-message" && i+1 < len(args) {
					outputFile = args[i+1]
					break
				}
			}
			if outputFile == "" {
				t.Fatalf("args missing output file: %+v", args)
			}
			if err := os.WriteFile(outputFile, []byte("captured answer"), 0o600); err != nil {
				t.Fatalf("write output file: %v", err)
			}
			return []byte("stdout noise"), []byte("session record failed"), os.ErrInvalid
		},
	}
	client := newTestClient(t, runner)

	response, err := client.Ask(context.Background(), providers.AskRequest{Prompt: "Hello"})
	if err != nil {
		t.Fatalf("ask codex: %v", err)
	}
	if response.Content != "captured answer" {
		t.Fatalf("content = %q", response.Content)
	}
}

func TestHealthRunsVersion(t *testing.T) {
	runner := &fakeRunner{
		lookPath: "codex",
		runFunc: func(ctx context.Context, name string, args []string, stdin string) ([]byte, []byte, error) {
			if !reflect.DeepEqual(args, []string{"--version"}) {
				t.Fatalf("args = %+v", args)
			}
			return []byte("codex-cli 0.125.0"), nil, nil
		},
	}
	client := newTestClient(t, runner)

	if err := client.Health(context.Background()); err != nil {
		t.Fatalf("health: %v", err)
	}
}

func newTestClient(t *testing.T, runner Runner) *Client {
	t.Helper()

	client, err := New("default", appconfig.Provider{
		Type:    "codex-cli",
		Profile: "default",
	}, WithRunner(runner), WithWorkingDir("C:\\repo"))
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	return client
}

type fakeRunner struct {
	lookPath string
	runFunc  func(context.Context, string, []string, string) ([]byte, []byte, error)
}

func (r *fakeRunner) Run(ctx context.Context, name string, args []string, stdin string) ([]byte, []byte, error) {
	return r.runFunc(ctx, name, args, stdin)
}

func (r *fakeRunner) LookPath(name string) (string, error) {
	if r.lookPath == "" {
		return "", os.ErrNotExist
	}
	return r.lookPath, nil
}
