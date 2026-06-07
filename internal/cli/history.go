package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	appconfig "ai-harness/internal/config"
	historypkg "ai-harness/internal/history"
	"ai-harness/internal/security"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

type historyStore interface {
	Save(historypkg.Record) (historypkg.Record, error)
	List() ([]historypkg.Record, error)
	Load(string) (historypkg.Record, error)
}

func newHistoryCommand() *cobra.Command {
	var configPath string

	cmd := &cobra.Command{
		Use:   "history",
		Short: "Inspect execution history",
	}
	cmd.PersistentFlags().StringVar(&configPath, "config", "", "config file path")
	cmd.AddCommand(
		newHistoryListCommand(&configPath),
		newHistoryShowCommand(&configPath),
	)

	return cmd
}

func newHistoryListCommand(configPath *string) *cobra.Command {
	var limit int
	var asJSON bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List execution history records",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := historyStoreForConfigPath(*configPath)
			if err != nil {
				return err
			}
			records, err := store.List()
			if err != nil {
				return err
			}
			if limit > 0 && len(records) > limit {
				records = records[:limit]
			}

			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(records)
			}

			printHistoryList(cmd.OutOrStdout(), records)
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 20, "maximum records to show; use 0 for all")
	cmd.Flags().BoolVar(&asJSON, "json", false, "print records as JSON")

	return cmd
}

func newHistoryShowCommand(configPath *string) *cobra.Command {
	var asJSON bool

	cmd := &cobra.Command{
		Use:   "show <id>",
		Short: "Show one execution history record",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := historyStoreForConfigPath(*configPath)
			if err != nil {
				return err
			}
			record, err := store.Load(args[0])
			if err != nil {
				return err
			}

			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(record)
			}

			printHistoryRecord(cmd.OutOrStdout(), record)
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print the record as JSON")

	return cmd
}

func historyStoreForConfigPath(configPath string) (historyStore, error) {
	configPath = strings.TrimSpace(configPath)
	if configPath == "" {
		path, err := appconfig.DefaultPath()
		if err != nil {
			return nil, err
		}
		configPath = path
	}

	dir := appconfig.HistoryDirForConfigPath(configPath)
	cfg, err := appconfig.Load(configPath)
	if err != nil {
		return historypkg.NewStore(dir), nil
	}
	if cfg.Security.Required && cfg.Security.KeyID == "" {
		return nil, fmt.Errorf("security is required but not initialized; run security init")
	}
	if cfg.Security.Enabled && cfg.Security.EncryptHistory && cfg.Security.KeyID != "" {
		key, err := security.ResolveKey(cfg, "")
		if err != nil {
			return nil, err
		}
		return security.NewEncryptedHistoryStore(dir, key, cfg.Security.KeyID), nil
	}

	return historypkg.NewStore(dir), nil
}

func finalizeHistory(configPath string, record *historypkg.Record, runErr error) error {
	if record == nil || strings.TrimSpace(record.Command) == "" {
		return runErr
	}

	if runErr != nil {
		record.Success = false
		record.Status = "failed"
		record.Error = runErr.Error()
	} else {
		record.Success = true
		if strings.TrimSpace(record.Status) == "" {
			record.Status = "completed"
		}
		record.Error = ""
	}

	store, err := historyStoreForConfigPath(configPath)
	if err != nil {
		if runErr != nil {
			return fmt.Errorf("%w; resolve history store: %v", runErr, err)
		}
		return err
	}
	saved, err := store.Save(*record)
	if err != nil {
		if runErr != nil {
			return fmt.Errorf("%w; save history: %v", runErr, err)
		}
		return fmt.Errorf("save history: %w", err)
	}
	log.Debug().Str("history_id", saved.ID).Str("command", saved.Command).Msg("history record saved")

	return runErr
}

func printHistoryList(w io.Writer, records []historypkg.Record) {
	if len(records) == 0 {
		fmt.Fprintln(w, "No history records found.")
		return
	}

	fmt.Fprintf(w, "%-40s  %-20s  %-10s  %-12s  %-10s  %-9s  %s\n", "ID", "TIMESTAMP", "COMMAND", "PROVIDER", "STATUS", "ESCALATED", "TASK")
	for _, record := range records {
		fmt.Fprintf(
			w,
			"%-40s  %-20s  %-10s  %-12s  %-10s  %-9t  %s\n",
			record.ID,
			formatHistoryTime(record.Timestamp),
			emptyDash(record.Command),
			emptyDash(record.Provider),
			emptyDash(record.Status),
			record.Escalated,
			shortHistoryText(record.Task, 72),
		)
	}
}

func printHistoryRecord(w io.Writer, record historypkg.Record) {
	fmt.Fprintf(w, "ID: %s\n", record.ID)
	fmt.Fprintf(w, "Timestamp: %s\n", formatHistoryTime(record.Timestamp))
	fmt.Fprintf(w, "Command: %s\n", emptyDash(record.Command))
	fmt.Fprintf(w, "Task: %s\n", emptyDash(record.Task))
	fmt.Fprintf(w, "Provider: %s\n", emptyDash(record.Provider))
	fmt.Fprintf(w, "Model: %s\n", emptyDash(record.Model))
	fmt.Fprintf(w, "Status: %s\n", emptyDash(record.Status))
	fmt.Fprintf(w, "Success: %t\n", record.Success)
	fmt.Fprintf(w, "Escalated: %t\n", record.Escalated)
	if record.Error != "" {
		fmt.Fprintf(w, "Error: %s\n", record.Error)
	}

	if record.Classification != nil {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Classification:")
		fmt.Fprintf(w, "  Recommended Provider: %s\n", emptyDash(record.Classification.RecommendedProvider))
		fmt.Fprintf(w, "  Complexity: %s\n", emptyDash(record.Classification.Complexity))
		fmt.Fprintf(w, "  Risk: %s\n", emptyDash(record.Classification.Risk))
		fmt.Fprintf(w, "  Needs Repo Access: %t\n", record.Classification.NeedsRepoAccess)
		fmt.Fprintf(w, "  Needs Edits: %t\n", record.Classification.NeedsEdits)
		fmt.Fprintf(w, "  Needs Tests: %t\n", record.Classification.NeedsTests)
		fmt.Fprintf(w, "  Reason: %s\n", emptyDash(record.Classification.Reason))
	}

	if len(record.FilesTouched) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Files Touched:")
		for _, file := range record.FilesTouched {
			fmt.Fprintf(w, "  - %s\n", file)
		}
	}

	if len(record.TestsRun) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Tests Run:")
		for _, test := range record.TestsRun {
			fmt.Fprintf(w, "  - %s", emptyDash(test.Command))
			if test.Skipped {
				fmt.Fprint(w, " (skipped)")
			} else if test.Passed {
				fmt.Fprint(w, " (passed)")
			} else {
				fmt.Fprint(w, " (failed)")
			}
			fmt.Fprintln(w)
			if test.Output != "" {
				fmt.Fprintf(w, "    Output: %s\n", shortHistoryText(test.Output, 240))
			}
		}
	}
}

func formatHistoryTime(value time.Time) string {
	if value.IsZero() {
		return "-"
	}
	return value.Local().Format("2006-01-02 15:04:05")
}

func emptyDash(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "-"
	}
	return value
}

func shortHistoryText(value string, limit int) string {
	value = strings.Join(strings.Fields(value), " ")
	if limit <= 0 || len(value) <= limit {
		return value
	}
	if limit <= 3 {
		return value[:limit]
	}
	return value[:limit-3] + "..."
}
