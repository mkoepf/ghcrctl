package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/mhk/ghcrctl/internal/config"
	"github.com/mhk/ghcrctl/internal/gh"
	"github.com/spf13/cobra"
)

var jsonOutput bool

var imagesCmd = &cobra.Command{
	Use:   "images",
	Short: "List container images",
	Long:  `List all container images for the configured owner from GitHub Container Registry.`,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
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
		client, err := gh.NewClient(token)
		if err != nil {
			cmd.SilenceUsage = true
			return fmt.Errorf("failed to create GitHub client: %w", err)
		}

		// List packages
		ctx := context.Background()
		packages, err := client.ListPackages(ctx, owner, ownerType)
		if err != nil {
			cmd.SilenceUsage = true
			return fmt.Errorf("failed to list packages: %w", err)
		}

		// Output results
		if jsonOutput {
			return outputJSON(cmd.OutOrStdout(), packages)
		}
		return outputTable(cmd.OutOrStdout(), packages, owner)
	},
}

func outputJSON(w io.Writer, packages []string) error {
	data, err := json.MarshalIndent(packages, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}
	fmt.Fprintln(w, string(data))
	return nil
}

func outputTable(w io.Writer, packages []string, owner string) error {
	if len(packages) == 0 {
		fmt.Fprintf(w, "No container images found for %s\n", owner)
		return nil
	}

	fmt.Fprintf(w, "Container images for %s:\n\n", owner)
	for _, pkg := range packages {
		fmt.Fprintf(w, "  %s\n", pkg)
	}
	fmt.Fprintf(w, "\nTotal: %d image(s)\n", len(packages))

	return nil
}

func init() {
	rootCmd.AddCommand(imagesCmd)
	imagesCmd.Flags().BoolVar(&jsonOutput, "json", false, "Output in JSON format")
}
