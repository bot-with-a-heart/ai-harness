package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"ai-harness/internal/classification"
	repoctx "ai-harness/internal/context"
	"ai-harness/internal/patch"
	"ai-harness/internal/providers"
	"ai-harness/internal/router"

	"github.com/spf13/cobra"
)

var runPatchRunner patch.Runner = patch.ExecRunner{}

var errPatchDeclined = errors.New("patch declined")

func newRunCommand() *cobra.Command {
	localOpts := localProviderOptions{
		provider: "desktop",
		timeout:  10 * time.Minute,
	}
	codexOpts := codexProviderOptions{
		provider: "default",
		sandbox:  "read-only",
		timeout:  10 * time.Minute,
	}
	classifierOpts := localProviderOptions{
		provider: "desktop",
		timeout:  2 * time.Minute,
	}

	var classifyModel string
	var localModel string
	var codexModel string
	var heuristicOnly bool
	var noFallback bool
	var edit bool
	var localFirst bool
	var testCommand string

	cmd := &cobra.Command{
		Use:   "run [task]",
		Short: "Classify a task, choose a provider, and execute it",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			task := strings.Join(args, " ")
			localOpts.configPath = classifierOpts.configPath
			codexOpts.configPath = classifierOpts.configPath

			classifyFn, err := buildRunClassifier(cmd.Context(), classifierOpts, classifyModel, heuristicOnly, noFallback)
			if err != nil {
				return err
			}

			localProvider, err := loadLMStudioProvider(localOpts)
			if err != nil {
				return err
			}
			codexProvider, err := loadCodexProvider(codexOpts)
			if err != nil {
				return err
			}

			ctx, cancel := context.WithTimeout(cmd.Context(), maxDuration(localOpts.timeout, codexOpts.timeout, classifierOpts.timeout))
			defer cancel()

			if localFirst {
				return runLocalFirstEdit(ctx, cmd, safeEditOptions{
					Task:          task,
					Classify:      classifyFn,
					LocalProvider: localProvider,
					CodexProvider: codexProvider,
					LocalModel:    localModel,
					CodexModel:    codexModel,
					TestCommand:   testCommand,
					WorkingDir:    codexOpts.workingDir,
				})
			}

			if edit {
				return runSafeEdit(ctx, cmd, safeEditOptions{
					Task:          task,
					Classify:      classifyFn,
					LocalProvider: localProvider,
					CodexProvider: codexProvider,
					LocalModel:    localModel,
					CodexModel:    codexModel,
					TestCommand:   testCommand,
					WorkingDir:    codexOpts.workingDir,
				})
			}

			result, err := router.Run(ctx, router.Options{
				Task:          task,
				Classify:      classifyFn,
				LocalProvider: localProvider,
				CodexProvider: codexProvider,
				LocalModel:    localModel,
				CodexModel:    codexModel,
			})
			if err != nil {
				return fmt.Errorf("run task: %w", err)
			}

			printRunResult(cmd.OutOrStdout(), result)
			return nil
		},
	}
	cmd.Flags().StringVar(&classifierOpts.configPath, "config", "", "config file path")
	cmd.Flags().StringVar(&classifierOpts.provider, "classifier-provider", classifierOpts.provider, "LM Studio provider name for classification")
	cmd.Flags().StringVar(&classifyModel, "classifier-model", "", "LM Studio model ID for classification")
	cmd.Flags().DurationVar(&classifierOpts.timeout, "classify-timeout", classifierOpts.timeout, "classification timeout")
	cmd.Flags().BoolVar(&heuristicOnly, "heuristic", false, "classify without calling LM Studio")
	cmd.Flags().BoolVar(&noFallback, "no-fallback", false, "fail instead of using heuristic fallback when classification fails")

	cmd.Flags().StringVar(&localOpts.provider, "local-provider", localOpts.provider, "LM Studio provider name for local execution")
	cmd.Flags().StringVar(&localModel, "local-model", "", "LM Studio model ID for selected local execution")
	cmd.Flags().DurationVar(&localOpts.timeout, "local-timeout", localOpts.timeout, "LM Studio execution timeout")

	cmd.Flags().StringVar(&codexOpts.provider, "codex-provider", codexOpts.provider, "Codex provider name for Codex execution")
	cmd.Flags().StringVar(&codexOpts.profile, "codex-profile", "", "Codex config profile override")
	cmd.Flags().StringVar(&codexOpts.workingDir, "cd", "", "working directory for Codex execution")
	cmd.Flags().StringVar(&codexOpts.sandbox, "sandbox", codexOpts.sandbox, "Codex sandbox mode")
	cmd.Flags().StringVar(&codexModel, "codex-model", "", "Codex model override")
	cmd.Flags().DurationVar(&codexOpts.timeout, "codex-timeout", codexOpts.timeout, "Codex execution timeout")

	cmd.Flags().BoolVar(&edit, "edit", false, "generate a patch, require approval, apply it, and run tests")
	cmd.Flags().BoolVar(&localFirst, "local-first", false, "attempt a local patch first, then escalate to Codex if validation or tests fail")
	cmd.Flags().StringVar(&testCommand, "test-command", "auto", "test command after patch approval; auto detects common project types")

	return cmd
}

type safeEditOptions struct {
	Task          string
	Classify      router.ClassifyFunc
	LocalProvider providers.Provider
	CodexProvider providers.Provider
	LocalModel    string
	CodexModel    string
	TestCommand   string
	WorkingDir    string
}

func runSafeEdit(ctx context.Context, cmd *cobra.Command, opts safeEditOptions) error {
	decision, err := opts.Classify(ctx, opts.Task)
	if err != nil {
		return fmt.Errorf("classify task: %w", err)
	}

	provider, fallback, model, err := providerForEdit(decision.RecommendedProvider, opts)
	if err != nil {
		return err
	}

	root, err := editRoot(opts.WorkingDir)
	if err != nil {
		return err
	}

	snapshot, err := collectRepositoryContext(ctx, repoctx.Options{
		Root:         root,
		MaxDepth:     4,
		MaxFiles:     300,
		MaxFileBytes: 16 * 1024,
		IncludeDiff:  false,
	})
	if err != nil {
		return fmt.Errorf("collect repository context: %w", err)
	}

	err = runEditAttempt(ctx, cmd, editAttemptOptions{
		Task:           opts.Task,
		Decision:       decision,
		Provider:       provider,
		ProviderKey:    decision.RecommendedProvider,
		Fallback:       fallback,
		Model:          model,
		Prompt:         patch.BuildPrompt(opts.Task, snapshot),
		Root:           root,
		TestCommand:    opts.TestCommand,
		ApprovalReader: bufio.NewReader(cmd.InOrStdin()),
	})
	if errors.Is(err, errPatchDeclined) {
		return nil
	}
	return err
}

func runLocalFirstEdit(ctx context.Context, cmd *cobra.Command, opts safeEditOptions) error {
	decision, err := opts.Classify(ctx, opts.Task)
	if err != nil {
		return fmt.Errorf("classify task: %w", err)
	}
	if opts.LocalProvider == nil {
		return fmt.Errorf("selected LM Studio provider is not configured")
	}
	if opts.CodexProvider == nil {
		return fmt.Errorf("Codex provider is not configured")
	}

	root, err := editRoot(opts.WorkingDir)
	if err != nil {
		return err
	}

	snapshot, err := collectRepositoryContext(ctx, repoctx.Options{
		Root:         root,
		MaxDepth:     4,
		MaxFiles:     300,
		MaxFileBytes: 16 * 1024,
		IncludeDiff:  false,
	})
	if err != nil {
		return fmt.Errorf("collect repository context: %w", err)
	}

	localDecision := decision
	localDecision.RecommendedProvider = "lmstudio"
	approvalReader := bufio.NewReader(cmd.InOrStdin())
	fmt.Fprintln(cmd.OutOrStdout(), "Local-first attempt: lmstudio")
	localErr := runEditAttempt(ctx, cmd, editAttemptOptions{
		Task:           opts.Task,
		Decision:       localDecision,
		Provider:       opts.LocalProvider,
		ProviderKey:    "lmstudio",
		Fallback:       "codex",
		Model:          opts.LocalModel,
		Prompt:         patch.BuildPrompt(opts.Task, snapshot),
		Root:           root,
		TestCommand:    opts.TestCommand,
		ApprovalReader: approvalReader,
	})
	if localErr == nil {
		fmt.Fprintln(cmd.OutOrStdout(), "Local attempt completed successfully.")
		return nil
	}
	if errors.Is(localErr, errPatchDeclined) {
		return nil
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Local attempt failed validation: %v\n", localErr)
	fmt.Fprintln(cmd.OutOrStdout(), "Escalating to Codex.")
	fmt.Fprintln(cmd.OutOrStdout())

	escalationSnapshot, err := collectRepositoryContext(ctx, repoctx.Options{
		Root:         root,
		MaxDepth:     4,
		MaxFiles:     300,
		MaxFileBytes: 16 * 1024,
		MaxDiffBytes: 64 * 1024,
		IncludeDiff:  true,
	})
	if err != nil {
		return fmt.Errorf("collect escalation repository context: %w", err)
	}

	codexDecision := decision
	codexDecision.RecommendedProvider = "codex"
	codexDecision.Reason = "Escalated after local attempt failed: " + decision.Reason
	err = runEditAttempt(ctx, cmd, editAttemptOptions{
		Task:           opts.Task,
		Decision:       codexDecision,
		Provider:       opts.CodexProvider,
		ProviderKey:    "codex",
		Fallback:       "lmstudio",
		Model:          opts.CodexModel,
		Prompt:         patch.BuildEscalationPrompt(opts.Task, escalationSnapshot, localErr.Error()),
		Root:           root,
		TestCommand:    opts.TestCommand,
		ApprovalReader: approvalReader,
	})
	if errors.Is(err, errPatchDeclined) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("codex escalation failed: %w", err)
	}
	fmt.Fprintln(cmd.OutOrStdout(), "Codex escalation completed successfully.")
	return nil
}

type editAttemptOptions struct {
	Task           string
	Decision       classification.Decision
	Provider       providers.Provider
	ProviderKey    string
	Fallback       string
	Model          string
	Prompt         string
	Root           string
	TestCommand    string
	ApprovalReader *bufio.Reader
}

func runEditAttempt(ctx context.Context, cmd *cobra.Command, opts editAttemptOptions) error {
	if opts.ApprovalReader == nil {
		opts.ApprovalReader = bufio.NewReader(cmd.InOrStdin())
	}

	response, err := opts.Provider.Ask(ctx, providers.AskRequest{
		Model:  opts.Model,
		Prompt: opts.Prompt,
	})
	if err != nil {
		return fmt.Errorf("generate patch with %s provider: %w", opts.ProviderKey, err)
	}

	diff, err := patch.ExtractUnifiedDiff(response.Content)
	if err != nil {
		return fmt.Errorf("extract generated patch: %w", err)
	}

	printEditPlan(cmd.OutOrStdout(), opts.Decision, opts.Fallback, response.Model, diff)
	if !confirmPatchReader(opts.ApprovalReader, cmd.OutOrStdout()) {
		fmt.Fprintln(cmd.OutOrStdout(), "Patch not applied.")
		return errPatchDeclined
	}

	if err := patch.Apply(ctx, opts.Root, diff, runPatchRunner); err != nil {
		return fmt.Errorf("apply patch: %w", err)
	}
	fmt.Fprintln(cmd.OutOrStdout(), "Patch applied.")

	result, testErr := patch.RunTests(ctx, opts.Root, opts.TestCommand, runPatchRunner)
	printTestResult(cmd.OutOrStdout(), result)
	if testErr != nil {
		return fmt.Errorf("run tests: %w", testErr)
	}

	return nil
}

func providerForEdit(recommended string, opts safeEditOptions) (providers.Provider, string, string, error) {
	switch strings.ToLower(strings.TrimSpace(recommended)) {
	case "lmstudio":
		if opts.LocalProvider == nil {
			return nil, "", "", fmt.Errorf("selected LM Studio provider is not configured")
		}
		return opts.LocalProvider, "codex", opts.LocalModel, nil
	case "codex":
		if opts.CodexProvider == nil {
			return nil, "", "", fmt.Errorf("selected Codex provider is not configured")
		}
		return opts.CodexProvider, "lmstudio", opts.CodexModel, nil
	default:
		return nil, "", "", fmt.Errorf("unsupported recommended provider %q", recommended)
	}
}

func editRoot(workingDir string) (string, error) {
	if strings.TrimSpace(workingDir) != "" {
		return workingDir, nil
	}
	return os.Getwd()
}

func printEditPlan(w io.Writer, decision classification.Decision, fallback string, model string, diff string) {
	fmt.Fprintf(w, "Provider Selected: %s\n", decision.RecommendedProvider)
	fmt.Fprintf(w, "Reason: %s\n", decision.Reason)
	fmt.Fprintf(w, "Fallback Provider: %s\n", fallback)
	fmt.Fprintf(w, "Complexity: %s\n", decision.Complexity)
	fmt.Fprintf(w, "Risk: %s\n", decision.Risk)
	fmt.Fprintf(w, "Needs repo access: %t\n", decision.NeedsRepoAccess)
	fmt.Fprintf(w, "Needs edits: %t\n", decision.NeedsEdits)
	fmt.Fprintf(w, "Needs tests: %t\n", decision.NeedsTests)
	fmt.Fprintf(w, "Model: %s\n\n", model)
	fmt.Fprintln(w, "Generated Patch:")
	fmt.Fprintln(w, diff)
}

func confirmPatch(r io.Reader, w io.Writer) bool {
	return confirmPatchReader(bufio.NewReader(r), w)
}

func confirmPatchReader(r *bufio.Reader, w io.Writer) bool {
	fmt.Fprint(w, "Apply patch? Type 'yes' to apply: ")
	line, err := r.ReadString('\n')
	if err != nil && line == "" {
		fmt.Fprintln(w)
		return false
	}
	answer := strings.ReplaceAll(line, "\x00", "")
	answer = strings.ReplaceAll(answer, "\ufeff", "")
	answer = strings.ToLower(strings.TrimSpace(answer))
	return answer == "yes"
}

func printTestResult(w io.Writer, result patch.TestResult) {
	if result.Skipped {
		fmt.Fprintln(w, "Tests skipped: no test command detected.")
		return
	}
	fmt.Fprintf(w, "Tests: %s\n", result.Command)
	if result.Output != "" {
		fmt.Fprintln(w, result.Output)
	}
	if result.Passed {
		fmt.Fprintln(w, "Tests passed.")
	}
}

func buildRunClassifier(ctx context.Context, opts localProviderOptions, model string, heuristicOnly bool, noFallback bool) (router.ClassifyFunc, error) {
	if heuristicOnly {
		return func(ctx context.Context, task string) (classification.Decision, error) {
			return classification.Heuristic(task)
		}, nil
	}

	provider, err := loadLMStudioProvider(opts)
	if err != nil {
		return nil, err
	}

	return func(ctx context.Context, task string) (classification.Decision, error) {
		classifyCtx, cancel := context.WithTimeout(ctx, opts.timeout)
		defer cancel()

		decision, err := classification.Agent{Provider: provider}.Classify(classifyCtx, classification.Request{
			Task:  task,
			Model: model,
		})
		if err != nil && !noFallback {
			decision, err = classification.Heuristic(task)
			if err == nil {
				decision.Reason = "Heuristic fallback after local classifier failed: " + decision.Reason
			}
		}
		return decision, err
	}, nil
}

func printRunResult(w interface {
	Write([]byte) (int, error)
}, result router.Result) {
	fmt.Fprintf(w, "Provider Selected: %s\n", result.ProviderSelected)
	fmt.Fprintf(w, "Reason: %s\n", result.Decision.Reason)
	fmt.Fprintf(w, "Fallback Provider: %s\n", result.FallbackProvider)
	fmt.Fprintf(w, "Complexity: %s\n", result.Decision.Complexity)
	fmt.Fprintf(w, "Risk: %s\n", result.Decision.Risk)
	fmt.Fprintf(w, "Needs repo access: %t\n", result.Decision.NeedsRepoAccess)
	fmt.Fprintf(w, "Needs edits: %t\n", result.Decision.NeedsEdits)
	fmt.Fprintf(w, "Needs tests: %t\n", result.Decision.NeedsTests)
	fmt.Fprintf(w, "Model: %s\n\n", result.Response.Model)
	fmt.Fprintln(w, result.Response.Content)
}

func maxDuration(values ...time.Duration) time.Duration {
	var max time.Duration
	for _, value := range values {
		if value > max {
			max = value
		}
	}
	if max <= 0 {
		return 10 * time.Minute
	}
	return max
}
