package router

import (
	"context"
	"errors"
	"testing"

	"ai-harness/internal/classification"
	"ai-harness/internal/providers"
)

func TestRunExecutesSelectedLocalProvider(t *testing.T) {
	local := &fakeProvider{response: providers.AskResponse{Model: "local-model", Content: "local answer"}}
	codex := &fakeProvider{response: providers.AskResponse{Model: "codex-model", Content: "codex answer"}}

	result, err := Run(context.Background(), Options{
		Task: "Explain this repo",
		Classify: func(ctx context.Context, task string) (classification.Decision, error) {
			return classification.Decision{
				Complexity:          "low",
				Risk:                "low",
				RecommendedProvider: "lmstudio",
				Reason:              "Simple explanation",
			}, nil
		},
		LocalProvider: local,
		CodexProvider: codex,
		LocalModel:    "local-model",
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if result.ProviderSelected != "lmstudio" || result.FallbackProvider != "codex" {
		t.Fatalf("result providers = %+v", result)
	}
	if result.Response.Content != "local answer" {
		t.Fatalf("response = %+v", result.Response)
	}
	if local.lastRequest.Model != "local-model" || local.lastRequest.Prompt != "Explain this repo" {
		t.Fatalf("local request = %+v", local.lastRequest)
	}
	if codex.called {
		t.Fatal("codex provider was called unexpectedly")
	}
}

func TestRunExecutesSelectedCodexProvider(t *testing.T) {
	local := &fakeProvider{response: providers.AskResponse{Model: "local-model", Content: "local answer"}}
	codex := &fakeProvider{response: providers.AskResponse{Model: "codex-model", Content: "codex answer"}}

	result, err := Run(context.Background(), Options{
		Task: "Refactor this app",
		Classify: func(ctx context.Context, task string) (classification.Decision, error) {
			return classification.Decision{
				Complexity:          "high",
				Risk:                "medium",
				RecommendedProvider: "codex",
				Reason:              "Architecture work",
			}, nil
		},
		LocalProvider: local,
		CodexProvider: codex,
		CodexModel:    "codex-model",
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if result.ProviderSelected != "codex" || result.FallbackProvider != "lmstudio" {
		t.Fatalf("result providers = %+v", result)
	}
	if result.Response.Content != "codex answer" {
		t.Fatalf("response = %+v", result.Response)
	}
	if codex.lastRequest.Model != "codex-model" {
		t.Fatalf("codex request = %+v", codex.lastRequest)
	}
	if local.called {
		t.Fatal("local provider was called unexpectedly")
	}
}

func TestRunReturnsClassificationError(t *testing.T) {
	_, err := Run(context.Background(), Options{
		Task: "hello",
		Classify: func(ctx context.Context, task string) (classification.Decision, error) {
			return classification.Decision{}, errors.New("classifier unavailable")
		},
	})
	if err == nil {
		t.Fatal("run succeeded, want error")
	}
}

type fakeProvider struct {
	called      bool
	lastRequest providers.AskRequest
	response    providers.AskResponse
}

func (p *fakeProvider) Name() string {
	return "fake"
}

func (p *fakeProvider) Health(context.Context) error {
	return nil
}

func (p *fakeProvider) ListModels(context.Context) ([]providers.Model, error) {
	return nil, nil
}

func (p *fakeProvider) Ask(ctx context.Context, req providers.AskRequest) (providers.AskResponse, error) {
	p.called = true
	p.lastRequest = req
	return p.response, nil
}
