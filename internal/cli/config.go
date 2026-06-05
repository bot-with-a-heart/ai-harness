package cli

import (
	"fmt"
	"io"
	"time"

	appconfig "ai-harness/internal/config"

	"github.com/spf13/cobra"
)

func newConfigCommand() *cobra.Command {
	var configPath string

	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage ai-harness configuration",
	}
	cmd.PersistentFlags().StringVar(&configPath, "path", "", "config file path")

	resolvePath := func() (string, error) {
		if configPath != "" {
			return configPath, nil
		}
		return appconfig.DefaultPath()
	}

	cmd.AddCommand(
		newConfigInitCommand(resolvePath),
		newConfigShowCommand(resolvePath),
		newConfigDoctorCommand(resolvePath),
	)

	return cmd
}

func newConfigInitCommand(resolvePath func() (string, error)) *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create a default config file",
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := resolvePath()
			if err != nil {
				return err
			}

			if err := appconfig.Init(path, appconfig.InitOptions{Force: force}); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Created config at %s\n", path)
			fmt.Fprintf(cmd.OutOrStdout(), "Created history directory at %s\n", appconfig.HistoryDirForConfigPath(path))
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "overwrite an existing config file")

	return cmd
}

func newConfigShowCommand(resolvePath func() (string, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Print the active config file",
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := resolvePath()
			if err != nil {
				return err
			}

			contents, err := appconfig.Read(path)
			if err != nil {
				return err
			}

			_, err = cmd.OutOrStdout().Write(contents)
			return err
		},
	}
}

func newConfigDoctorCommand(resolvePath func() (string, error)) *cobra.Command {
	var timeout time.Duration

	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Validate config and local tool availability",
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := resolvePath()
			if err != nil {
				return err
			}

			report := appconfig.Doctor(appconfig.DoctorOptions{
				Path:    path,
				Timeout: timeout,
			})
			printDoctorReport(cmd.OutOrStdout(), report)
			if !report.Passed() {
				return fmt.Errorf("config doctor found failures")
			}

			return nil
		},
	}
	cmd.Flags().DurationVar(&timeout, "timeout", 3*time.Second, "network check timeout")

	return cmd
}

func printDoctorReport(w io.Writer, report appconfig.DoctorReport) {
	for _, check := range report.Checks {
		fmt.Fprintf(w, "%-4s %s - %s\n", check.Status, check.Name, check.Message)
		if check.Err != nil {
			fmt.Fprintf(w, "     %v\n", check.Err)
		}
	}
}
