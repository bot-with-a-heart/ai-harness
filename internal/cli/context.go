package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	repoctx "ai-harness/internal/context"

	"github.com/spf13/cobra"
)

var collectRepositoryContext = repoctx.Collect

func newContextCommand() *cobra.Command {
	var root string
	var jsonOutput bool
	var maxDepth int
	var maxFiles int
	var maxFileBytes int
	var maxDiffBytes int
	var noDiff bool
	var timeout time.Duration

	cmd := &cobra.Command{
		Use:   "context",
		Short: "Collect repository context",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 && strings.TrimSpace(root) == "" {
				root = args[0]
			}

			ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
			defer cancel()

			snapshot, err := collectRepositoryContext(ctx, repoctx.Options{
				Root:         root,
				MaxDepth:     maxDepth,
				MaxFiles:     maxFiles,
				MaxFileBytes: maxFileBytes,
				MaxDiffBytes: maxDiffBytes,
				IncludeDiff:  !noDiff,
			})
			if err != nil {
				return fmt.Errorf("collect repository context: %w", err)
			}
			if err := repoctx.ValidateSnapshot(snapshot); err != nil {
				return err
			}

			if jsonOutput {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(snapshot)
			}

			printRepositoryContext(cmd.OutOrStdout(), snapshot)
			return nil
		},
	}
	cmd.Flags().StringVar(&root, "path", "", "repository path")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "print repository context as JSON")
	cmd.Flags().IntVar(&maxDepth, "max-depth", 3, "maximum directory depth to include")
	cmd.Flags().IntVar(&maxFiles, "max-files", 300, "maximum directory entries to include")
	cmd.Flags().IntVar(&maxFileBytes, "max-file-bytes", 16*1024, "maximum bytes to read from each key file")
	cmd.Flags().IntVar(&maxDiffBytes, "max-diff-bytes", 64*1024, "maximum bytes to include from git diff")
	cmd.Flags().BoolVar(&noDiff, "no-diff", false, "skip git diff collection")
	cmd.Flags().DurationVar(&timeout, "timeout", 15*time.Second, "repository context collection timeout")

	return cmd
}

func printRepositoryContext(w interface {
	Write([]byte) (int, error)
}, snapshot repoctx.Snapshot) {
	fmt.Fprintf(w, "Repository: %s\n", snapshot.Root)
	fmt.Fprintf(w, "Generated: %s\n\n", snapshot.GeneratedAt.Format(time.RFC3339))

	fmt.Fprintln(w, "Languages:")
	if len(snapshot.Languages) == 0 {
		fmt.Fprintln(w, "- none detected")
	} else {
		for _, language := range snapshot.Languages {
			fmt.Fprintf(w, "- %s: %d files, %d bytes\n", language.Name, language.Files, language.Bytes)
		}
	}

	fmt.Fprintln(w, "\nFrameworks:")
	if len(snapshot.Frameworks) == 0 {
		fmt.Fprintln(w, "- none detected")
	} else {
		for _, framework := range snapshot.Frameworks {
			fmt.Fprintf(w, "- %s\n", framework)
		}
	}

	fmt.Fprintln(w, "\nGit:")
	if snapshot.Git.Available {
		fmt.Fprintln(w, "Status:")
		if strings.TrimSpace(snapshot.Git.Status) == "" {
			fmt.Fprintln(w, "(clean)")
		} else {
			fmt.Fprintln(w, snapshot.Git.Status)
		}
		if snapshot.Git.Diff != "" {
			fmt.Fprintln(w, "\nDiff:")
			fmt.Fprintln(w, snapshot.Git.Diff)
			if snapshot.Git.DiffTruncated {
				fmt.Fprintln(w, "[diff truncated]")
			}
		}
	} else {
		fmt.Fprintln(w, "unavailable")
	}

	fmt.Fprintln(w, "\nDirectory Structure:")
	if len(snapshot.DirectoryStructure) == 0 {
		fmt.Fprintln(w, "- none")
	} else {
		for _, entry := range snapshot.DirectoryStructure {
			suffix := ""
			if entry.Type == "dir" {
				suffix = "/"
			}
			fmt.Fprintf(w, "- %s%s\n", entry.Path, suffix)
		}
	}

	fmt.Fprintln(w, "\nKey Files:")
	if len(snapshot.KeyFiles) == 0 {
		fmt.Fprintln(w, "- none found")
	} else {
		for _, file := range snapshot.KeyFiles {
			truncated := ""
			if file.Truncated {
				truncated = " [truncated]"
			}
			fmt.Fprintf(w, "\n--- %s%s ---\n", file.Path, truncated)
			fmt.Fprintln(w, strings.TrimRight(file.Content, "\n"))
		}
	}

	fmt.Fprintln(w, "\nSensitive Exclusions:")
	for _, pattern := range snapshot.ExcludedPatterns {
		fmt.Fprintf(w, "- %s\n", pattern)
	}

	if len(snapshot.Warnings) > 0 {
		fmt.Fprintln(w, "\nWarnings:")
		for _, warning := range snapshot.Warnings {
			fmt.Fprintf(w, "- %s\n", warning)
		}
	}
}
