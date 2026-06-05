package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"ai-harness/internal/classification"
	"ai-harness/internal/providers"
)

func TestClassifyCommandUsesLMStudio(t *testing.T) {
	restore := stubLMStudioProvider(t, fakeLocalProvider{
		response: providers.AskResponse{
			Model: "model-a",
			Content: `{
				"complexity": "low",
				"risk": "low",
				"needsRepoAccess": true,
				"needsEdits": false,
				"needsTests": false,
				"recommendedProvider": "lmstudio",
				"reason": "Simple explanation task"
			}`,
		},
	})
	defer restore()

	path := writeLocalConfig(t, "http://127.0.0.1:1234/v1")
	cmd := NewRootCommand("test")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"classify", "--config", path, "Explain", "this", "repository"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("classify failed: %v\n%s", err, out.String())
	}

	var decision classification.Decision
	if err := json.Unmarshal(out.Bytes(), &decision); err != nil {
		t.Fatalf("decode output: %v\n%s", err, out.String())
	}
	if decision.RecommendedProvider != "lmstudio" || decision.Complexity != "low" {
		t.Fatalf("decision = %+v", decision)
	}
}

func TestClassifyCommandHeuristic(t *testing.T) {
	cmd := NewRootCommand("test")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"classify", "--heuristic", "--summary", "Refactor", "my", "AWS", "CDK", "application"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("heuristic classify failed: %v\n%s", err, out.String())
	}
	got := out.String()
	for _, want := range []string{"Provider: codex", "Complexity: high"} {
		if !strings.Contains(got, want) {
			t.Fatalf("summary missing %q:\n%s", want, got)
		}
	}
}

func TestClassifyCommandFallsBackOnInvalidModelOutput(t *testing.T) {
	restore := stubLMStudioProvider(t, fakeLocalProvider{
		response: providers.AskResponse{
			Model:   "model-a",
			Content: "not json",
		},
	})
	defer restore()

	path := writeLocalConfig(t, "http://127.0.0.1:1234/v1")
	cmd := NewRootCommand("test")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"classify", "--config", path, "Fix", "authentication", "bug"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("classify fallback failed: %v\n%s", err, out.String())
	}

	var decision classification.Decision
	if err := json.Unmarshal(out.Bytes(), &decision); err != nil {
		t.Fatalf("decode output: %v\n%s", err, out.String())
	}
	if decision.RecommendedProvider != "codex" || !strings.Contains(decision.Reason, "Heuristic fallback") {
		t.Fatalf("decision = %+v", decision)
	}
}

func TestFakeLocalProviderImplementsProvider(t *testing.T) {
	var _ providers.Provider = fakeLocalProvider{}
	_, _ = fakeLocalProvider{}.Ask(context.Background(), providers.AskRequest{})
}
