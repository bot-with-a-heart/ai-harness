package router

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"ai-harness/internal/classification"
	"ai-harness/internal/providers"
)

type ClassifyFunc func(context.Context, string) (classification.Decision, error)

type Options struct {
	Task          string
	Classify      ClassifyFunc
	LocalProvider providers.Provider
	CodexProvider providers.Provider
	LocalModel    string
	CodexModel    string
}

type Result struct {
	Decision         classification.Decision
	ProviderSelected string
	FallbackProvider string
	Response         providers.AskResponse
}

func Run(ctx context.Context, opts Options) (Result, error) {
	task := strings.TrimSpace(opts.Task)
	if task == "" {
		return Result{}, errors.New("task is required")
	}
	if opts.Classify == nil {
		return Result{}, errors.New("classifier is required")
	}

	decision, err := opts.Classify(ctx, task)
	if err != nil {
		return Result{}, fmt.Errorf("classify task: %w", err)
	}

	selected, fallback, model, err := selectProvider(decision.RecommendedProvider, opts)
	if err != nil {
		return Result{}, err
	}

	response, err := selected.Ask(ctx, providers.AskRequest{
		Model:  model,
		Prompt: task,
	})
	if err != nil {
		return Result{}, fmt.Errorf("execute %s provider: %w", decision.RecommendedProvider, err)
	}

	return Result{
		Decision:         decision,
		ProviderSelected: decision.RecommendedProvider,
		FallbackProvider: fallback,
		Response:         response,
	}, nil
}

func selectProvider(recommended string, opts Options) (providers.Provider, string, string, error) {
	switch strings.ToLower(strings.TrimSpace(recommended)) {
	case "lmstudio":
		if opts.LocalProvider == nil {
			return nil, "", "", errors.New("selected LM Studio provider is not configured")
		}
		return opts.LocalProvider, "codex", opts.LocalModel, nil
	case "codex":
		if opts.CodexProvider == nil {
			return nil, "", "", errors.New("selected Codex provider is not configured")
		}
		return opts.CodexProvider, "lmstudio", opts.CodexModel, nil
	default:
		return nil, "", "", fmt.Errorf("unsupported recommended provider %q", recommended)
	}
}
