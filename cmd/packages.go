package cmd

import (
	"fmt"
	"io"

	"github.com/mkoepf/ghcrctl/internal/display"
	"github.com/mkoepf/ghcrctl/internal/gh"
	"github.com/spf13/cobra"
)

// newPackagesCmd creates the packages command with isolated flag state.
func newPackagesCmd() *cobra.Command {
	var (
		jsonOutput   bool
		outputFormat string
	)

	cmd := &cobra.Command{
		Use:   "packages <owner>",
		Short: "List container packages",
		Long: `List all container packages for the specified owner from GitHub Container Registry.

Examples:
  # List all packages for a user
  ghcrctl packages mkoepf

  # List all packages for an organization
  ghcrctl packages myorg

  # List packages in JSON format
  ghcrctl packages mkoepf --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			owner := args[0]

			// Handle output format flag (-o)
			if outputFormat != "" {
				switch outputFormat {
				case "json":
					jsonOutput = true
				case "table":
					jsonOutput = false
				default:
					cmd.SilenceUsage = true
					return fmt.Errorf("invalid output format %q. Supported formats: json, table", outputFormat)
				}
			}

			// Get GitHub token
			token, err := gh.GetToken()
			if err != nil {
				cmd.SilenceUsage = true
				return err
			}

			// Create GitHub client
			client, err := gh.NewClientWithContext(cmd.Context(), token)
			if err != nil {
				cmd.SilenceUsage = true
				return fmt.Errorf("failed to create GitHub client: %w", err)
			}

			ctx := cmd.Context()

			// Auto-detect owner type
			ownerType, err := client.GetOwnerType(ctx, owner)
			if err != nil {
				cmd.SilenceUsage = true
				return fmt.Errorf("failed to determine owner type: %w", err)
			}

			// List packages
			packages, err := client.ListPackages(ctx, owner, ownerType)
			if err != nil {
				cmd.SilenceUsage = true
				return fmt.Errorf("failed to list packages: %w", err)
			}

			// Output results
			if jsonOutput {
				return display.OutputJSON(cmd.OutOrStdout(), packages)
			}
			return outputPackagesTable(cmd.OutOrStdout(), packages, owner)
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output in JSON format")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "", "Output format (json, table)")

	return cmd
}

func outputPackagesTable(w io.Writer, packages []string, owner string) error {
	if len(packages) == 0 {
		fmt.Fprintf(w, "No packages found for %s\n", owner)
		return nil
	}

	fmt.Fprintf(w, "Packages for %s:\n\n", owner)
	for _, pkg := range packages {
		fmt.Fprintf(w, "  %s\n", pkg)
	}
	fmt.Fprintf(w, "\nTotal: %s package(s)\n", display.ColorCount(len(packages)))

	return nil
}
