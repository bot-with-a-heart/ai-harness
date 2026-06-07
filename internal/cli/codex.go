package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	appconfig "ai-harness/internal/config"
	"ai-harness/internal/history"
	"ai-harness/internal/providers"
	codexprovider "ai-harness/internal/providers/codex"

	"github.com/spf13/cobra"
)

type codexProviderOptions struct {
	configPath string
	provider   string
	profile    string
	workingDir string
	sandbox    string
	timeout    time.Duration
}

var newCodexProvider = func(name string, cfg appconfig.Provider, workingDir string, sandbox string) (providers.Provider, error) {
	return codexprovider.New(
		name,
		cfg,
		codexprovider.WithWorkingDir(workingDir),
		codexprovider.WithSandbox(sandbox),
	)
}

func newAskCodexCommand() *cobra.Command {
	opts := codexProviderOptions{
		provider: "default",
		sandbox:  "read-only",
		timeout:  10 * time.Minute,
	}
	var model string

	cmd := &cobra.Command{
		Use:   "ask-codex [prompt]",
		Short: "Ask Codex CLI to answer a prompt",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) (runErr error) {
			prompt := strings.Join(args, " ")
			record := history.Record{Command: "ask-codex", Task: prompt, Provider: "codex"}
			defer func() {
				runErr = finalizeHistory(opts.configPath, &record, runErr)
			}()

			provider, err := loadCodexProvider(opts)
			if err != nil {
				return err
			}

			ctx, cancel := codexprovider.ContextWithTimeout(cmd.Context(), opts.timeout)
			defer cancel()

			response, err := provider.Ask(ctx, providers.AskRequest{
				Model:  model,
				Prompt: prompt,
			})
			if err != nil {
				return fmt.Errorf("ask Codex: %w", err)
			}
			record.Model = response.Model

			fmt.Fprintf(cmd.OutOrStdout(), "Provider: codex/%s\n", opts.provider)
			fmt.Fprintf(cmd.OutOrStdout(), "Model: %s\n\n", response.Model)
			fmt.Fprintln(cmd.OutOrStdout(), response.Content)

			return nil
		},
	}
	addCodexProviderFlags(cmd, &opts)
	cmd.Flags().StringVar(&model, "model", "", "Codex model override")

	return cmd
}

func addCodexProviderFlags(cmd *cobra.Command, opts *codexProviderOptions) {
	cmd.Flags().StringVar(&opts.configPath, "config", "", "config file path")
	cmd.Flags().StringVar(&opts.provider, "provider", opts.provider, "Codex provider name from config")
	cmd.Flags().StringVar(&opts.profile, "profile", "", "Codex config profile override")
	cmd.Flags().StringVar(&opts.workingDir, "cd", "", "working directory for codex exec")
	cmd.Flags().StringVar(&opts.sandbox, "sandbox", opts.sandbox, "Codex sandbox mode")
	cmd.Flags().DurationVar(&opts.timeout, "timeout", opts.timeout, "Codex request timeout")
}

func loadCodexProvider(opts codexProviderOptions) (providers.Provider, error) {
	cfg, err := appconfig.Load(opts.configPath)
	if err != nil {
		return nil, err
	}

	providerName := strings.TrimSpace(opts.provider)
	if providerName == "" {
		return nil, errors.New("provider name is required")
	}

	group, ok := cfg.Providers["codex"]
	if !ok || len(group) == 0 {
		return nil, errors.New("no Codex providers configured")
	}

	providerCfg, ok := group[providerName]
	if !ok {
		return nil, fmt.Errorf("Codex provider %q not found; available providers: %s", providerName, strings.Join(providerNames(group), ", "))
	}
	if providerCfg.Type != "" && providerCfg.Type != "codex-cli" {
		return nil, fmt.Errorf("Codex provider %q has unsupported type %q", providerName, providerCfg.Type)
	}
	if strings.TrimSpace(opts.profile) != "" {
		providerCfg.Profile = opts.profile
	}

	workingDir := opts.workingDir
	if strings.TrimSpace(workingDir) == "" {
		workingDir, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("resolve working directory: %w", err)
		}
	}

	return newCodexProvider(providerName, providerCfg, workingDir, opts.sandbox)
}
