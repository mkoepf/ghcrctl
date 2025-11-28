package cmd

import (
	"fmt"
	"io"

	"github.com/mkoepf/ghcrctl/internal/config"
	"github.com/mkoepf/ghcrctl/internal/display"
	"github.com/mkoepf/ghcrctl/internal/gh"
	"github.com/spf13/cobra"
)

// newImagesCmd creates the images command with isolated flag state.
func newImagesCmd() *cobra.Command {
	var (
		jsonOutput   bool
		outputFormat string
	)

	cmd := &cobra.Command{
		Use:   "images",
		Short: "List container images",
		Long: `List all container images for the configured owner from GitHub Container Registry.

Examples:
  # List all images for configured owner
  ghcrctl images

  # List images in JSON format
  ghcrctl images --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
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

			// Load configuration
			cfg := config.New()
			owner, ownerType, err := cfg.GetOwner()
			if err != nil {
				cmd.SilenceUsage = true
				return fmt.Errorf("failed to read configuration: %w", err)
			}

			if owner == "" || ownerType == "" {
				cmd.SilenceUsage = true
				return fmt.Errorf("owner not configured. Use 'ghcrctl config org <name>' or 'ghcrctl config user <name>' to set owner")
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

			// List packages
			ctx := cmd.Context()
			packages, err := client.ListPackages(ctx, owner, ownerType)
			if err != nil {
				cmd.SilenceUsage = true
				return fmt.Errorf("failed to list packages: %w", err)
			}

			// Output results
			if jsonOutput {
				return display.OutputJSON(cmd.OutOrStdout(), packages)
			}
			return outputImagesTable(cmd.OutOrStdout(), packages, owner)
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output in JSON format")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "", "Output format (json, table)")

	return cmd
}

func outputImagesTable(w io.Writer, packages []string, owner string) error {
	if len(packages) == 0 {
		fmt.Fprintf(w, "No container images found for %s\n", owner)
		return nil
	}

	fmt.Fprintf(w, "Container images for %s:\n\n", owner)
	for _, pkg := range packages {
		fmt.Fprintf(w, "  %s\n", pkg)
	}
	fmt.Fprintf(w, "\nTotal: %s image(s)\n", display.ColorCount(len(packages)))

	return nil
}
