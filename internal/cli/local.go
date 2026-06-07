package cli

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	appconfig "ai-harness/internal/config"
	"ai-harness/internal/history"
	"ai-harness/internal/providers"
	"ai-harness/internal/providers/lmstudio"

	"github.com/spf13/cobra"
)

type localProviderOptions struct {
	configPath string
	provider   string
	timeout    time.Duration
}

var newLMStudioProvider = func(name string, cfg appconfig.Provider) (providers.Provider, error) {
	return lmstudio.New(name, cfg)
}

func newModelsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "models",
		Short: "Inspect configured model providers",
	}
	cmd.AddCommand(
		newModelsListCommand(),
		newModelsCatalogCommand(),
	)

	return cmd
}

func newModelsListCommand() *cobra.Command {
	opts := localProviderOptions{
		provider: "desktop",
		timeout:  10 * time.Second,
	}

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List models available from LM Studio",
		RunE: func(cmd *cobra.Command, args []string) error {
			provider, err := loadLMStudioProvider(opts)
			if err != nil {
				return err
			}

			ctx, cancel := context.WithTimeout(cmd.Context(), opts.timeout)
			defer cancel()

			models, err := provider.ListModels(ctx)
			if err != nil {
				return fmt.Errorf("list LM Studio models: %w", err)
			}
			if len(models) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No models returned by LM Studio provider %q\n", opts.provider)
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Provider: lmstudio/%s\n\n", opts.provider)
			for _, model := range models {
				fmt.Fprintln(cmd.OutOrStdout(), model.ID)
			}

			return nil
		},
	}
	addLocalProviderFlags(cmd, &opts)

	return cmd
}

func newAskLocalCommand() *cobra.Command {
	opts := localProviderOptions{
		provider: "desktop",
		timeout:  2 * time.Minute,
	}
	var model string

	cmd := &cobra.Command{
		Use:   "ask-local [prompt]",
		Short: "Ask LM Studio to answer a prompt",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) (runErr error) {
			prompt := strings.Join(args, " ")
			record := history.Record{Command: "ask-local", Task: prompt, Provider: "lmstudio"}
			defer func() {
				runErr = finalizeHistory(opts.configPath, &record, runErr)
			}()

			provider, err := loadLMStudioProvider(opts)
			if err != nil {
				return err
			}

			ctx, cancel := context.WithTimeout(cmd.Context(), opts.timeout)
			defer cancel()

			response, err := provider.Ask(ctx, providers.AskRequest{
				Model:  model,
				Prompt: prompt,
			})
			if err != nil {
				return fmt.Errorf("ask LM Studio: %w", err)
			}
			record.Model = response.Model

			fmt.Fprintf(cmd.OutOrStdout(), "Provider: lmstudio/%s\n", opts.provider)
			fmt.Fprintf(cmd.OutOrStdout(), "Model: %s\n\n", response.Model)
			fmt.Fprintln(cmd.OutOrStdout(), response.Content)

			return nil
		},
	}
	addLocalProviderFlags(cmd, &opts)
	cmd.Flags().StringVar(&model, "model", "", "LM Studio model ID; defaults to the first discovered model")

	return cmd
}

func addLocalProviderFlags(cmd *cobra.Command, opts *localProviderOptions) {
	cmd.Flags().StringVar(&opts.configPath, "config", "", "config file path")
	cmd.Flags().StringVar(&opts.provider, "provider", opts.provider, "LM Studio provider name from config")
	cmd.Flags().DurationVar(&opts.timeout, "timeout", opts.timeout, "LM Studio request timeout")
}

func loadLMStudioProvider(opts localProviderOptions) (providers.Provider, error) {
	cfg, err := appconfig.Load(opts.configPath)
	if err != nil {
		return nil, err
	}

	providerName := strings.TrimSpace(opts.provider)
	if providerName == "" {
		return nil, errors.New("provider name is required")
	}

	group, ok := cfg.Providers["lmstudio"]
	if !ok || len(group) == 0 {
		return nil, errors.New("no LM Studio providers configured")
	}

	providerCfg, ok := group[providerName]
	if !ok {
		return nil, fmt.Errorf("LM Studio provider %q not found; available providers: %s", providerName, strings.Join(providerNames(group), ", "))
	}
	if providerCfg.Type != "" && providerCfg.Type != "openai-compatible" {
		return nil, fmt.Errorf("LM Studio provider %q has unsupported type %q", providerName, providerCfg.Type)
	}

	return newLMStudioProvider(providerName, providerCfg)
}

func providerNames(group map[string]appconfig.Provider) []string {
	names := make([]string, 0, len(group))
	for name := range group {
		names = append(names, name)
	}
	sort.Strings(names)

	return names
}
