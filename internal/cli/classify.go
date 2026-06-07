package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"ai-harness/internal/classification"
	"ai-harness/internal/history"

	"github.com/spf13/cobra"
)

func newClassifyCommand() *cobra.Command {
	opts := localProviderOptions{
		provider: "desktop",
		timeout:  2 * time.Minute,
	}
	var model string
	var heuristicOnly bool
	var noFallback bool
	var summary bool

	cmd := &cobra.Command{
		Use:   "classify [task]",
		Short: "Classify a task and recommend LM Studio or Codex",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) (runErr error) {
			task := strings.Join(args, " ")
			record := history.Record{Command: "classify", Task: task}
			defer func() {
				runErr = finalizeHistory(opts.configPath, &record, runErr)
			}()

			var decision classification.Decision
			var err error
			if heuristicOnly {
				record.Provider = "heuristic"
				decision, err = classification.Heuristic(task)
			} else {
				record.Provider = "lmstudio"
				record.Model = model
				provider, loadErr := loadLMStudioProvider(opts)
				if loadErr != nil {
					return loadErr
				}
				ctx, cancel := context.WithTimeout(cmd.Context(), opts.timeout)
				defer cancel()

				decision, err = classification.Agent{Provider: provider}.Classify(ctx, classification.Request{
					Task:  task,
					Model: model,
				})
				if err != nil && !noFallback {
					decision, err = classification.Heuristic(task)
					if err == nil {
						record.Provider = "heuristic"
						record.Model = ""
						decision.Reason = "Heuristic fallback after local classifier failed: " + decision.Reason
					}
				}
			}
			if err != nil {
				return fmt.Errorf("classify task: %w", err)
			}
			setHistoryClassification(&record, decision)

			if summary {
				printClassificationSummary(cmd.OutOrStdout(), decision)
				return nil
			}

			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(decision)
		},
	}
	addLocalProviderFlags(cmd, &opts)
	cmd.Flags().StringVar(&model, "model", "", "LM Studio model ID for classification")
	cmd.Flags().BoolVar(&heuristicOnly, "heuristic", false, "classify without calling LM Studio")
	cmd.Flags().BoolVar(&noFallback, "no-fallback", false, "fail instead of using heuristic fallback when LM Studio output is invalid")
	cmd.Flags().BoolVar(&summary, "summary", false, "print a human-readable summary instead of JSON")

	return cmd
}

func printClassificationSummary(w interface {
	Write([]byte) (int, error)
}, decision classification.Decision) {
	fmt.Fprintf(w, "Provider: %s\n", decision.RecommendedProvider)
	fmt.Fprintf(w, "Complexity: %s\n", decision.Complexity)
	fmt.Fprintf(w, "Risk: %s\n", decision.Risk)
	fmt.Fprintf(w, "Needs repo access: %t\n", decision.NeedsRepoAccess)
	fmt.Fprintf(w, "Needs edits: %t\n", decision.NeedsEdits)
	fmt.Fprintf(w, "Needs tests: %t\n", decision.NeedsTests)
	fmt.Fprintf(w, "Reason: %s\n", decision.Reason)
}
