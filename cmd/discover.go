package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/mkoepf/ghcrctl/internal/discover"
	"github.com/mkoepf/ghcrctl/internal/gh"
	"github.com/spf13/cobra"
)

func newDiscoverCmd() *cobra.Command {
	var (
		jsonOutput   bool
		treeOutput   bool
		outputFormat string
	)

	cmd := &cobra.Command{
		Use:   "discover <owner/image>",
		Short: "Discover package versions and their relationships",
		Long: `Discover all versions of a package, resolve their types, and show relationships.

Examples:
  # Discover versions of a package
  ghcrctl discover mkoepf/my-image

  # Show in tree format
  ghcrctl discover mkoepf/my-image --tree

  # Output in JSON format
  ghcrctl discover mkoepf/my-image --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Parse owner/image reference
			owner, packageName, _, err := parseImageRef(args[0])
			if err != nil {
				cmd.SilenceUsage = true
				return err
			}

			// Handle output format flag (-o)
			if outputFormat != "" {
				switch outputFormat {
				case "json":
					jsonOutput = true
				case "table":
					jsonOutput = false
				case "tree":
					treeOutput = true
				default:
					cmd.SilenceUsage = true
					return fmt.Errorf("invalid output format %q. Supported formats: json, table, tree", outputFormat)
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

			// List package versions
			versions, err := client.ListPackageVersions(ctx, owner, ownerType, packageName)
			if err != nil {
				cmd.SilenceUsage = true
				return fmt.Errorf("failed to list package versions: %w", err)
			}

			if len(versions) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No versions found for %s\n", packageName)
				return nil
			}

			// Collect all tags for cosign discovery
			var allTags []string
			for _, v := range versions {
				allTags = append(allTags, v.Tags...)
			}

			// Build image reference
			image := fmt.Sprintf("ghcr.io/%s/%s", owner, packageName)

			// Discover versions and relationships
			discoverer := discover.NewPackageDiscoverer()
			results, err := discoverer.DiscoverPackage(ctx, image, versions, allTags)
			if err != nil {
				cmd.SilenceUsage = true
				return fmt.Errorf("failed to discover package: %w", err)
			}

			// Build version map for output
			allVersions := make(map[string]discover.VersionInfo)
			for _, v := range results {
				allVersions[v.Digest] = v
			}

			// Output results
			if jsonOutput {
				data, err := json.MarshalIndent(results, "", "  ")
				if err != nil {
					return fmt.Errorf("failed to marshal JSON: %w", err)
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(data))
				return nil
			}

			if treeOutput {
				discover.FormatTree(cmd.OutOrStdout(), results, allVersions)
			} else {
				discover.FormatTable(cmd.OutOrStdout(), results, allVersions)
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output in JSON format")
	cmd.Flags().BoolVar(&treeOutput, "tree", false, "Output in tree format")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "", "Output format (json, table, tree)")

	return cmd
}
