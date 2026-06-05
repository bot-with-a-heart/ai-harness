package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

const appName = "ai-harness"

// NewRootCommand builds the command tree. Keeping construction separate from
// execution makes commands easy to exercise in tests and future integrations.
func NewRootCommand(version string) *cobra.Command {
	root := &cobra.Command{
		Use:          appName,
		Short:        "Local-first AI coding harness",
		Long:         "AI Harness routes software development tasks between local and hosted coding agents.",
		SilenceUsage: true,
	}

	root.AddCommand(
		newConfigCommand(),
		newClassifyCommand(),
		newContextCommand(),
		newModelsCommand(),
		newRunCommand(),
		newAskLocalCommand(),
		newAskCodexCommand(),
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
