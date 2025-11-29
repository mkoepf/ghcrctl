package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/mkoepf/ghcrctl/internal/discover"
	"github.com/mkoepf/ghcrctl/internal/gh"
	"github.com/spf13/cobra"
)

func newImagesCmd() *cobra.Command {
	var (
		jsonOutput   bool
		flatOutput   bool
		outputFormat string
	)

	cmd := &cobra.Command{
		Use:   "images <owner/package>",
		Short: "Show all images in a package with their related artifacts",
		Long: `Show all images (OCI artifacts) in a package, grouped by their relationships.

Each image includes its platform manifests, attestations (SBOM, provenance, etc.),
and signatures. By default, images are displayed in a tree format showing these
relationships. Use --flat for a simple table view.

Examples:
  # Show images with relationships (tree view, default)
  ghcrctl images mkoepf/my-package

  # Show images in flat table format
  ghcrctl images mkoepf/my-package --flat

  # Output in JSON format
  ghcrctl images mkoepf/my-package --json`,
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
				case "table", "flat":
					flatOutput = true
				case "tree":
					flatOutput = false
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
				fmt.Fprintf(cmd.OutOrStdout(), "No images found for %s\n", packageName)
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
				return fmt.Errorf("failed to discover images: %w", err)
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

			// Default is tree output; --flat switches to table
			if flatOutput {
				discover.FormatTable(cmd.OutOrStdout(), results, allVersions)
			} else {
				discover.FormatTree(cmd.OutOrStdout(), results, allVersions)
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output in JSON format")
	cmd.Flags().BoolVar(&flatOutput, "flat", false, "Output in flat table format (default is tree)")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "", "Output format (json, table, tree)")

	return cmd
}
