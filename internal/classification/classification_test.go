package classification

import (
	"context"
	"strings"
	"testing"

	"ai-harness/internal/providers"
)

func TestParseExtractsJSONFromMarkdown(t *testing.T) {
	raw := "```json\n{\"complexity\":\"low\",\"risk\":\"low\",\"needsRepoAccess\":true,\"needsEdits\":false,\"needsTests\":false,\"recommendedProvider\":\"lmstudio\",\"reason\":\"Simple explanation task\"}\n```"

	decision, err := Parse(raw)
	if err != nil {
		t.Fatalf("parse classification: %v", err)
	}
	if decision.RecommendedProvider != "lmstudio" {
		t.Fatalf("provider = %q", decision.RecommendedProvider)
	}
}

func TestParseRejectsInvalidProvider(t *testing.T) {
	raw := `{"complexity":"low","risk":"low","needsRepoAccess":false,"needsEdits":false,"needsTests":false,"recommendedProvider":"unknown","reason":"bad"}`

	if _, err := Parse(raw); err == nil {
		t.Fatal("parse succeeded, want invalid provider error")
	}
}

func TestAgentClassifyUsesProvider(t *testing.T) {
	agent := Agent{Provider: fakeClassifierProvider{
		response: providers.AskResponse{
			Model: "local",
			Content: `{
				"complexity": "high",
				"risk": "medium",
				"needsRepoAccess": true,
				"needsEdits": true,
				"needsTests": true,
				"recommendedProvider": "codex",
				"reason": "Multi-file refactor"
			}`,
		},
	}}

	decision, err := agent.Classify(context.Background(), Request{
		Task:  "Refactor my AWS CDK application",
		Model: "model-a",
	})
	if err != nil {
		t.Fatalf("classify: %v", err)
	}
	if decision.RecommendedProvider != "codex" || decision.Complexity != "high" {
		t.Fatalf("decision = %+v", decision)
	}
}

func TestHeuristicRoutesSecurityToCodex(t *testing.T) {
	decision, err := Heuristic("Fix authentication permission checks")
	if err != nil {
		t.Fatalf("heuristic: %v", err)
	}
	if decision.RecommendedProvider != "codex" || decision.Risk != "high" || !decision.NeedsRepoAccess {
		t.Fatalf("decision = %+v", decision)
	}
}

func TestBuildPromptIncludesRoutingRules(t *testing.T) {
	prompt := BuildPrompt("Explain this repo")
	for _, want := range []string{"Use LM Studio", "Use Codex", "Return only valid JSON", "Explain this repo"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q:\n%s", want, prompt)
		}
	}
}

type fakeClassifierProvider struct {
	response providers.AskResponse
}

func (p fakeClassifierProvider) Name() string {
	return "fake"
}

func (p fakeClassifierProvider) Health(context.Context) error {
	return nil
}

func (p fakeClassifierProvider) ListModels(context.Context) ([]providers.Model, error) {
	return nil, nil
}

func (p fakeClassifierProvider) Ask(context.Context, providers.AskRequest) (providers.AskResponse, error) {
	return p.response, nil
}
