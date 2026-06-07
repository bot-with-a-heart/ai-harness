package cli

import (
	"fmt"

	"ai-harness/internal/logging"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

const appName = "ai-harness"

// NewRootCommand builds the command tree. Keeping construction separate from
// execution makes commands easy to exercise in tests and future integrations.
func NewRootCommand(version string) *cobra.Command {
	var logLevel string
	var logJSON bool

	root := &cobra.Command{
		Use:          appName,
		Short:        "Local-first AI coding harness",
		Long:         "AI Harness routes software development tasks between local and hosted coding agents.",
		SilenceUsage: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if err := logging.Configure(logging.Options{
				Level: logLevel,
				JSON:  logJSON,
				Out:   cmd.ErrOrStderr(),
			}); err != nil {
				return err
			}
			log.Debug().Str("command", cmd.CommandPath()).Msg("command started")
			return nil
		},
	}
	root.PersistentFlags().StringVar(&logLevel, "log-level", "warn", "log level: debug, info, warn, error, or disabled")
	root.PersistentFlags().BoolVar(&logJSON, "log-json", false, "write logs as JSON to stderr")

	root.AddCommand(
		newConfigCommand(),
		newClassifyCommand(),
		newContextCommand(),
		newHistoryCommand(),
		newMemoryCommand(),
		newModelsCommand(),
		newRunCommand(),
		newAskLocalCommand(),
		newAskCodexCommand(),
		newSecurityCommand(),
		newVersionCommand(version),
	)

	return root
}

func Execute(version string) error {
	return NewRootCommand(version).Execute()
}

func newVersionCommand(version string) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the ai-harness version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n", appName, version)
		},
	}
}
