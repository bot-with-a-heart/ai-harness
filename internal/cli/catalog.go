package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"ai-harness/internal/catalog"

	"github.com/spf13/cobra"
)

type catalogOptions struct {
	path    string
	timeout time.Duration
}

func newModelsCatalogCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "catalog",
		Short: "Manage cached LM Studio model knowledge",
	}
	cmd.AddCommand(
		newModelsCatalogUpdateCommand(),
		newModelsCatalogShowCommand(),
		newModelsCatalogExplainCommand(),
	)

	return cmd
}

func newModelsCatalogUpdateCommand() *cobra.Command {
	opts := catalogOptions{
		timeout: 2 * time.Minute,
	}
	var lmsPath string
	var skipEstimates bool

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Refresh the cached LM Studio model catalog",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), opts.timeout)
			defer cancel()

			modelCatalog, err := catalog.UpdateFromLMStudio(ctx, catalog.UpdateOptions{
				LMSPath:          lmsPath,
				IncludeEstimates: !skipEstimates,
				Timeout:          opts.timeout,
			})
			if err != nil {
				return err
			}

			path, err := resolveCatalogPath(opts.path)
			if err != nil {
				return err
			}
			if err := catalog.Save(path, modelCatalog); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Updated model catalog at %s\n", path)
			fmt.Fprintf(cmd.OutOrStdout(), "Models: %d\n", len(modelCatalog.Models))
			if len(modelCatalog.Warnings) > 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "Warnings:")
				for _, warning := range modelCatalog.Warnings {
					fmt.Fprintf(cmd.OutOrStdout(), "- %s\n", warning)
				}
			}

			return nil
		},
	}
	addCatalogPathFlag(cmd, &opts)
	cmd.Flags().DurationVar(&opts.timeout, "timeout", opts.timeout, "LM Studio catalog refresh timeout")
	cmd.Flags().StringVar(&lmsPath, "lms-path", "", "path to the lms executable")
	cmd.Flags().BoolVar(&skipEstimates, "skip-estimates", false, "skip lms load --estimate-only checks")

	return cmd
}

func newModelsCatalogShowCommand() *cobra.Command {
	opts := catalogOptions{}
	var asJSON bool

	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show the cached model catalog",
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := resolveCatalogPath(opts.path)
			if err != nil {
				return err
			}

			if asJSON {
				contents, err := os.ReadFile(path)
				if err != nil {
					return fmt.Errorf("read model catalog: %w", err)
				}
				_, err = cmd.OutOrStdout().Write(contents)
				return err
			}

			modelCatalog, err := catalog.Load(path)
			if err != nil {
				return err
			}
			printCatalogSummary(cmd.OutOrStdout(), modelCatalog)

			return nil
		},
	}
	addCatalogPathFlag(cmd, &opts)
	cmd.Flags().BoolVar(&asJSON, "json", false, "print raw catalog JSON")

	return cmd
}

func newModelsCatalogExplainCommand() *cobra.Command {
	opts := catalogOptions{}

	cmd := &cobra.Command{
		Use:   "explain <model-id>",
		Short: "Explain cached suitability details for a model",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := resolveCatalogPath(opts.path)
			if err != nil {
				return err
			}

			modelCatalog, err := catalog.Load(path)
			if err != nil {
				return err
			}
			model, ok := catalog.FindModel(modelCatalog, args[0])
			if !ok {
				return fmt.Errorf("model %q not found in catalog", args[0])
			}

			printModelExplanation(cmd.OutOrStdout(), model)
			return nil
		},
	}
	addCatalogPathFlag(cmd, &opts)

	return cmd
}

func addCatalogPathFlag(cmd *cobra.Command, opts *catalogOptions) {
	cmd.Flags().StringVar(&opts.path, "catalog", "", "model catalog path")
}

func resolveCatalogPath(path string) (string, error) {
	if path != "" {
		return path, nil
	}
	return catalog.DefaultPath()
}

func printCatalogSummary(w interface {
	Write([]byte) (int, error)
}, modelCatalog catalog.Catalog) {
	fmt.Fprintf(w, "Updated: %s\n", modelCatalog.UpdatedAt.Format(time.RFC3339))
	fmt.Fprintf(w, "Source: %s\n\n", modelCatalog.Source)

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "MODEL\tTYPE\tLOADED\tFIT\tSPEED\tCOMPLEXITY\tBEST FOR")
	for _, model := range modelCatalog.Models {
		fmt.Fprintf(
			tw,
			"%s\t%s\t%t\t%s\t%s\t%s\t%s\n",
			model.ID,
			model.Type,
			model.Loaded,
			model.HardwareFit,
			model.SpeedClass,
			model.ComplexityClass,
			strings.Join(model.BestFor, ", "),
		)
	}
	_ = tw.Flush()

	if len(modelCatalog.Warnings) > 0 {
		fmt.Fprintln(w, "\nWarnings:")
		for _, warning := range modelCatalog.Warnings {
			fmt.Fprintf(w, "- %s\n", warning)
		}
	}
}

func printModelExplanation(w interface {
	Write([]byte) (int, error)
}, model catalog.Model) {
	asJSON, _ := json.MarshalIndent(model, "", "  ")
	fmt.Fprintf(w, "%s\n", model.ID)
	fmt.Fprintf(w, "Name: %s\n", model.DisplayName)
	fmt.Fprintf(w, "Type: %s\n", model.Type)
	fmt.Fprintf(w, "Downloaded: %t\n", model.Downloaded)
	fmt.Fprintf(w, "Loaded: %t\n", model.Loaded)
	fmt.Fprintf(w, "Architecture: %s\n", model.Architecture)
	fmt.Fprintf(w, "Params: %s\n", model.Params)
	fmt.Fprintf(w, "Context: %d\n", model.MaxContextLength)
	fmt.Fprintf(w, "Hardware fit: %s\n", model.HardwareFit)
	if model.EstimatedTotalMemoryGB > 0 {
		fmt.Fprintf(w, "Estimated memory: %.2f GiB total, %.2f GiB GPU\n", model.EstimatedTotalMemoryGB, model.EstimatedGPUMemoryGB)
	}
	fmt.Fprintf(w, "Speed: %s\n", model.SpeedClass)
	fmt.Fprintf(w, "Complexity: %s\n", model.ComplexityClass)
	fmt.Fprintf(w, "Best for: %s\n", strings.Join(model.BestFor, ", "))
	if len(model.AvoidFor) > 0 {
		fmt.Fprintf(w, "Avoid for: %s\n", strings.Join(model.AvoidFor, ", "))
	}
	if model.Notes != "" {
		fmt.Fprintf(w, "Notes: %s\n", model.Notes)
	}
	fmt.Fprintf(w, "\nJSON:\n%s\n", asJSON)
}
