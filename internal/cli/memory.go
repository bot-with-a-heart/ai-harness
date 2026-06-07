package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"ai-harness/internal/catalog"
	appconfig "ai-harness/internal/config"
	"ai-harness/internal/history"
	"ai-harness/internal/obsidian"

	"github.com/spf13/cobra"
)

func newMemoryCommand() *cobra.Command {
	var configPath string

	cmd := &cobra.Command{
		Use:   "memory",
		Short: "Manage optional memory integrations",
	}
	cmd.PersistentFlags().StringVar(&configPath, "config", "", "config file path")
	cmd.AddCommand(newMemoryObsidianCommand(&configPath))

	return cmd
}

func newMemoryObsidianCommand(configPath *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "obsidian",
		Short: "Export selected memory to an optional Obsidian vault",
	}
	cmd.AddCommand(
		newMemoryObsidianStatusCommand(configPath),
		newMemoryObsidianInitCommand(configPath),
		newMemoryObsidianExportCommand(configPath),
	)

	return cmd
}

func newMemoryObsidianStatusCommand(configPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show Obsidian memory integration status",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, path, loadErr := loadMemoryConfig(*configPath)
			if loadErr != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "Config: %s\n", path)
				fmt.Fprintf(cmd.OutOrStdout(), "Obsidian integration: disabled\n")
				fmt.Fprintf(cmd.OutOrStdout(), "Reason: %v\n", loadErr)
				return nil
			}
			printObsidianStatus(cmd.OutOrStdout(), path, cfg.Memory.Obsidian)
			return nil
		},
	}
}

func newMemoryObsidianInitCommand(configPath *string) *cobra.Command {
	var vaultPath string
	var folder string
	var createVault bool

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Configure optional Obsidian vault export",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			vaultPath = strings.TrimSpace(vaultPath)
			if vaultPath == "" {
				return errors.New("--vault is required")
			}
			if folder == "" {
				folder = appconfig.DefaultObsidianFolder
			}
			if _, err := obsidian.BuildPlan(obsidian.ExportOptions{VaultPath: vaultPath, Folder: folder}); err != nil {
				return err
			}
			if err := ensureDirectory(vaultPath, createVault); err != nil {
				return err
			}

			cfg, path, err := loadMemoryConfig(*configPath)
			if err != nil {
				return err
			}
			cfg.Memory.Obsidian.Enabled = true
			cfg.Memory.Obsidian.VaultPath = vaultPath
			cfg.Memory.Obsidian.Folder = folder
			if cfg.Memory.Obsidian.ExportHistoryLimit <= 0 {
				cfg.Memory.Obsidian.ExportHistoryLimit = appconfig.DefaultObsidianHistoryLimit
			}
			if err := appconfig.Write(path, cfg); err != nil {
				return err
			}

			folderPath := filepath.Join(vaultPath, filepath.FromSlash(folder))
			if err := os.MkdirAll(folderPath, 0o700); err != nil {
				return fmt.Errorf("create Obsidian export folder: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Configured Obsidian export in %s\n", path)
			fmt.Fprintf(cmd.OutOrStdout(), "Vault: %s\n", vaultPath)
			fmt.Fprintf(cmd.OutOrStdout(), "Folder: %s\n", folder)
			return nil
		},
	}
	cmd.Flags().StringVar(&vaultPath, "vault", "", "Obsidian vault directory")
	cmd.Flags().StringVar(&folder, "folder", appconfig.DefaultObsidianFolder, "folder inside the vault for AI Harness notes")
	cmd.Flags().BoolVar(&createVault, "create-vault", false, "create the vault directory if it does not exist")

	return cmd
}

func newMemoryObsidianExportCommand(configPath *string) *cobra.Command {
	var vaultPath string
	var folder string
	var catalogPath string
	var dryRun bool
	var force bool
	var includeHistory bool
	var includeTaskText bool
	var historyLimit int

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export selected memory notes to Obsidian",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, path, err := loadMemoryConfig(*configPath)
			if err != nil {
				return err
			}

			explicitVault := strings.TrimSpace(vaultPath) != ""
			if !explicitVault {
				if !cfg.Memory.Obsidian.Enabled {
					fmt.Fprintln(cmd.OutOrStdout(), "Obsidian integration is disabled. Run memory obsidian init --vault <path> or pass --vault.")
					return nil
				}
				vaultPath = cfg.Memory.Obsidian.VaultPath
			}
			if strings.TrimSpace(vaultPath) == "" {
				fmt.Fprintln(cmd.OutOrStdout(), "No Obsidian vault path is configured. Run memory obsidian init --vault <path> or pass --vault.")
				return nil
			}
			if folder == "" {
				folder = cfg.Memory.Obsidian.Folder
			}
			if folder == "" {
				folder = appconfig.DefaultObsidianFolder
			}
			if historyLimit <= 0 {
				historyLimit = cfg.Memory.Obsidian.ExportHistoryLimit
			}
			if historyLimit <= 0 {
				historyLimit = appconfig.DefaultObsidianHistoryLimit
			}

			if !dryRun {
				if err := ensureDirectory(vaultPath, false); err != nil {
					return err
				}
			}

			modelCatalog, warnings := loadOptionalCatalog(catalogPath)
			records, historyWarnings := loadOptionalHistory(path, includeHistory, historyLimit)
			warnings = append(warnings, historyWarnings...)

			plan, err := obsidian.BuildPlan(obsidian.ExportOptions{
				VaultPath:       vaultPath,
				Folder:          folder,
				Catalog:         modelCatalog,
				History:         records,
				IncludeHistory:  includeHistory,
				IncludeTaskText: includeTaskText,
			})
			if err != nil {
				return err
			}
			plan.Warnings = append(plan.Warnings, warnings...)

			printObsidianPlan(cmd.OutOrStdout(), plan, dryRun)
			if dryRun {
				return nil
			}
			if err := obsidian.WritePlan(plan, force); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Exported %d Obsidian note(s).\n", len(plan.Files))
			return nil
		},
	}
	cmd.Flags().StringVar(&vaultPath, "vault", "", "Obsidian vault directory override")
	cmd.Flags().StringVar(&folder, "folder", "", "folder inside the vault for AI Harness notes")
	cmd.Flags().StringVar(&catalogPath, "catalog", "", "model catalog path; defaults to the harness model catalog")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "preview files without writing")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite existing notes even when they are not marked as AI Harness managed")
	cmd.Flags().BoolVar(&includeHistory, "include-history", false, "include recent task outcome summaries")
	cmd.Flags().BoolVar(&includeTaskText, "include-task-text", false, "include task text in history exports")
	cmd.Flags().IntVar(&historyLimit, "history-limit", 0, "maximum history records to export")

	return cmd
}

func loadMemoryConfig(configPath string) (appconfig.Config, string, error) {
	path := strings.TrimSpace(configPath)
	if path == "" {
		var err error
		path, err = appconfig.DefaultPath()
		if err != nil {
			return appconfig.Config{}, "", err
		}
	}
	cfg, err := appconfig.Load(path)
	if err != nil {
		return appconfig.Config{}, path, err
	}
	return cfg, path, nil
}

func ensureDirectory(path string, create bool) error {
	if create {
		if err := os.MkdirAll(path, 0o700); err != nil {
			return fmt.Errorf("create directory: %w", err)
		}
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("directory unavailable %s: %w", path, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("directory unavailable %s: not a directory", path)
	}
	return nil
}

func loadOptionalCatalog(path string) (*catalog.Catalog, []string) {
	modelCatalog, err := catalog.Load(path)
	if err != nil {
		return nil, []string{fmt.Sprintf("model catalog was not exported: %v", err)}
	}
	return &modelCatalog, nil
}

func loadOptionalHistory(configPath string, include bool, limit int) ([]history.Record, []string) {
	if !include {
		return nil, nil
	}
	store, err := historyStoreForConfigPath(configPath)
	if err != nil {
		return nil, []string{fmt.Sprintf("history was not exported: %v", err)}
	}
	records, err := store.List()
	if err != nil {
		return nil, []string{fmt.Sprintf("history was not exported: %v", err)}
	}
	if limit > 0 && len(records) > limit {
		records = records[:limit]
	}
	return records, nil
}

func printObsidianStatus(w io.Writer, configPath string, cfg appconfig.Obsidian) {
	fmt.Fprintf(w, "Config: %s\n", configPath)
	if cfg.Enabled {
		fmt.Fprintln(w, "Obsidian integration: enabled")
	} else {
		fmt.Fprintln(w, "Obsidian integration: disabled")
	}
	fmt.Fprintf(w, "Vault path: %s\n", emptyDash(cfg.VaultPath))
	fmt.Fprintf(w, "Folder: %s\n", emptyDash(cfg.Folder))
	fmt.Fprintf(w, "History export limit: %d\n", cfg.ExportHistoryLimit)
	if cfg.VaultPath == "" {
		fmt.Fprintln(w, "Vault available: false")
		return
	}
	info, err := os.Stat(cfg.VaultPath)
	fmt.Fprintf(w, "Vault available: %t\n", err == nil && info.IsDir())
	if err != nil {
		fmt.Fprintf(w, "Vault issue: %v\n", err)
	}
}

func printObsidianPlan(w io.Writer, plan obsidian.Plan, dryRun bool) {
	if dryRun {
		fmt.Fprintln(w, "Dry run: no files written.")
	}
	fmt.Fprintf(w, "Vault: %s\n", plan.VaultPath)
	fmt.Fprintf(w, "Folder: %s\n", plan.Folder)
	if len(plan.Files) == 0 {
		fmt.Fprintln(w, "No files planned.")
	} else {
		fmt.Fprintln(w, "Planned files:")
		for _, file := range plan.Files {
			status := "new"
			switch {
			case file.Conflict:
				status = "conflict"
			case file.Exists:
				status = "update"
			}
			fmt.Fprintf(w, "- %s (%s)\n", file.RelativePath, status)
		}
	}
	if len(plan.Warnings) > 0 {
		fmt.Fprintln(w, "Warnings:")
		for _, warning := range plan.Warnings {
			fmt.Fprintf(w, "- %s\n", warning)
		}
	}
}
