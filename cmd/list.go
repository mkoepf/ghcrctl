package cmd

import (
	"fmt"
	"io"
	"strings"

	"github.com/mkoepf/ghcrctl/internal/discover"
	"github.com/mkoepf/ghcrctl/internal/display"
	"github.com/mkoepf/ghcrctl/internal/filter"
	"github.com/mkoepf/ghcrctl/internal/gh"
	"github.com/mkoepf/ghcrctl/internal/quiet"
	"github.com/spf13/cobra"
)

// newListCmd creates the list parent command with its subcommands.
func newListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List resources (packages, versions, images)",
		Long: `List resources from GitHub Container Registry.

Available subcommands:
  packages    List all container packages for an owner
  versions    List all versions of a package
  images      List images with their artifact relationships`,
	}

	cmd.AddCommand(newListPackagesCmd())
	cmd.AddCommand(newListVersionsCmd())
	cmd.AddCommand(newListImagesCmd())

	return cmd
}

// newListPackagesCmd creates the list packages subcommand.
func newListPackagesCmd() *cobra.Command {
	var (
		jsonOutput   bool
		outputFormat string
	)

	cmd := &cobra.Command{
		Use:   "packages <owner>",
		Short: "List container packages for an owner",
		Long: `List all container packages for the specified owner from GitHub Container Registry.

Examples:
  # List all packages for a user
  ghcrctl list packages mkoepf

  # List all packages for an organization
  ghcrctl list packages myorg

  # List packages in JSON format
  ghcrctl list packages mkoepf --json`,
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
			return outputListPackagesTable(cmd.OutOrStdout(), packages, owner, quiet.IsQuiet(cmd.Context()))
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output in JSON format")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "", "Output format (json, table)")

	return cmd
}

func outputListPackagesTable(w io.Writer, packages []string, owner string, quietMode bool) error {
	if len(packages) == 0 {
		if !quietMode {
			fmt.Fprintf(w, "No packages found for %s\n", owner)
		}
		return nil
	}

	if !quietMode {
		fmt.Fprintf(w, "Packages for %s:\n\n", owner)
	}
	for _, pkg := range packages {
		fmt.Fprintf(w, "  %s\n", pkg)
	}
	if !quietMode {
		fmt.Fprintf(w, "\nTotal: %s package(s)\n", display.ColorCount(len(packages)))
	}

	return nil
}

// newListVersionsCmd creates the list versions subcommand.
func newListVersionsCmd() *cobra.Command {
	var (
		jsonOutput   bool
		tag          string
		tagPattern   string
		onlyTagged   bool
		onlyUntagged bool
		olderThan    string
		newerThan    string
		outputFormat string
		versionID    int64
		digest       string
	)

	cmd := &cobra.Command{
		Use:   "versions <owner/package>",
		Short: "List all versions of a package",
		Long: `List all versions of a container package.

This command displays all versions of a package with their version ID, digest,
tags, and creation date. The version ID can be used with the delete command.

To see artifact relationships (platform manifests, attestations, signatures),
use 'ghcrctl list images' instead.

Examples:
  # List all versions
  ghcrctl list versions mkoepf/myimage

  # Filter by specific tag
  ghcrctl list versions mkoepf/myimage --tag v1.0

  # List only tagged versions
  ghcrctl list versions mkoepf/myimage --tagged

  # List only untagged versions
  ghcrctl list versions mkoepf/myimage --untagged

  # List versions matching a tag pattern (regex)
  ghcrctl list versions mkoepf/myimage --tag-pattern "^v1\\..*"

  # List versions older than a specific date
  ghcrctl list versions mkoepf/myimage --older-than 2025-01-01

  # List versions newer than a specific date
  ghcrctl list versions mkoepf/myimage --newer-than 2025-11-01

  # List versions older than 30 days
  ghcrctl list versions mkoepf/myimage --older-than 30d

  # List versions from the last hour
  ghcrctl list versions mkoepf/myimage --newer-than 1h

  # Combine filters: untagged versions older than 7 days
  ghcrctl list versions mkoepf/myimage --untagged --older-than 7d

  # Filter by specific version ID
  ghcrctl list versions mkoepf/myimage --version 12345678

  # Filter by digest (supports prefix matching)
  ghcrctl list versions mkoepf/myimage --digest sha256:abc123

  # List versions in JSON format
  ghcrctl list versions mkoepf/myimage --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Parse owner/package reference (reject inline tags)
			owner, packageName, err := parsePackageRef(args[0])
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

			// List package versions
			allVersions, err := client.ListPackageVersions(ctx, owner, ownerType, packageName)
			if err != nil {
				cmd.SilenceUsage = true
				return fmt.Errorf("failed to list versions: %w", err)
			}

			// Build filter from command-line flags
			versionFilter, err := buildListVersionFilter(tag, tagPattern, onlyTagged, onlyUntagged,
				olderThan, newerThan, versionID, digest)
			if err != nil {
				cmd.SilenceUsage = true
				return fmt.Errorf("invalid filter options: %w", err)
			}

			// Apply filters to determine which versions to display
			filteredVersions := versionFilter.Apply(allVersions)
			if len(filteredVersions) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No versions found matching filter criteria")
				return nil
			}

			// JSON output
			if jsonOutput {
				return display.OutputJSON(cmd.OutOrStdout(), filteredVersions)
			}

			// Table output (default)
			return OutputVersionsTable(cmd.OutOrStdout(), filteredVersions, packageName, quiet.IsQuiet(cmd.Context()))
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output in JSON format")
	cmd.Flags().StringVar(&tag, "tag", "", "Filter versions by exact tag match")
	cmd.Flags().StringVar(&tagPattern, "tag-pattern", "", "Filter versions by tag regex pattern")
	cmd.Flags().BoolVar(&onlyTagged, "tagged", false, "Show only tagged versions")
	cmd.Flags().BoolVar(&onlyUntagged, "untagged", false, "Show only untagged versions")
	cmd.Flags().StringVar(&olderThan, "older-than", "", "Show versions older than date or duration (e.g., 2025-01-01, 7d, 24h, 30m)")
	cmd.Flags().StringVar(&newerThan, "newer-than", "", "Show versions newer than date or duration (e.g., 2025-01-01, 7d, 24h, 30m)")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "", "Output format (json, table)")
	cmd.Flags().Int64Var(&versionID, "version", 0, "Filter by exact version ID")
	cmd.Flags().StringVar(&digest, "digest", "", "Filter by digest (supports prefix matching)")

	// Mark mutually exclusive flags
	cmd.MarkFlagsMutuallyExclusive("tagged", "untagged")

	cmd.ValidArgsFunction = imageRefValidArgsFunc

	return cmd
}

// OutputVersionsTable outputs a flat list of versions (exported for testing).
// If quiet is true, informational headers and summaries are suppressed.
func OutputVersionsTable(w io.Writer, versions []gh.PackageVersionInfo, packageName string, quiet bool) error {
	if len(versions) == 0 {
		if !quiet {
			fmt.Fprintf(w, "No versions found for %s\n", packageName)
		}
		return nil
	}

	if !quiet {
		fmt.Fprintf(w, "Versions for %s:\n\n", packageName)
	}

	// Find column widths
	maxIDLen := len("VERSION ID")
	maxDigestLen := len("DIGEST")
	maxTagsLen := len("TAGS")

	for _, ver := range versions {
		if idLen := len(fmt.Sprintf("%d", ver.ID)); idLen > maxIDLen {
			maxIDLen = idLen
		}
		digestStr := display.ShortDigest(ver.Name)
		if len(digestStr) > maxDigestLen {
			maxDigestLen = len(digestStr)
		}
		if tagsStr := display.FormatTags(ver.Tags); len(tagsStr) > maxTagsLen {
			maxTagsLen = len(tagsStr)
		}
	}

	// Print header
	fmt.Fprintf(w, "  %s  %s  %s  %s\n",
		display.ColorHeader(fmt.Sprintf("%-*s", maxIDLen, "VERSION ID")),
		display.ColorHeader(fmt.Sprintf("%-*s", maxDigestLen, "DIGEST")),
		display.ColorHeader(fmt.Sprintf("%-*s", maxTagsLen, "TAGS")),
		display.ColorHeader("CREATED"))
	fmt.Fprintf(w, "  %s  %s  %s  %s\n",
		display.ColorSeparator(strings.Repeat("-", maxIDLen)),
		display.ColorSeparator(strings.Repeat("-", maxDigestLen)),
		display.ColorSeparator(strings.Repeat("-", maxTagsLen)),
		display.ColorSeparator(strings.Repeat("-", len("CREATED"))))

	// Print versions
	for _, ver := range versions {
		tagsStr := display.FormatTags(ver.Tags)
		digestStr := display.ShortDigest(ver.Name)

		fmt.Fprintf(w, "  %-*d  %s  %s%s  %s\n",
			maxIDLen, ver.ID,
			display.ColorDigest(fmt.Sprintf("%-*s", maxDigestLen, digestStr)),
			display.ColorTags(ver.Tags),
			strings.Repeat(" ", maxTagsLen-len(tagsStr)),
			ver.CreatedAt)
	}

	// Summary (only in non-quiet mode)
	if !quiet {
		versionWord := "versions"
		if len(versions) == 1 {
			versionWord = "version"
		}
		fmt.Fprintf(w, "\nTotal: %s %s.\n", display.ColorCount(len(versions)), versionWord)
	}

	return nil
}

// buildListVersionFilter creates a VersionFilter from command-line flags
func buildListVersionFilter(tag, tagPattern string, onlyTagged, onlyUntagged bool,
	olderThan, newerThan string,
	versionID int64, digest string) (*filter.VersionFilter, error) {
	// Check for conflicting flags
	if onlyTagged && onlyUntagged {
		return nil, fmt.Errorf("cannot use --tagged and --untagged together")
	}

	vf := &filter.VersionFilter{
		OnlyTagged:   onlyTagged,
		OnlyUntagged: onlyUntagged,
		TagPattern:   tagPattern,
		VersionID:    versionID,
		Digest:       digest,
	}

	// Handle exact tag match
	if tag != "" {
		vf.Tags = []string{tag}
	}

	// Parse date/duration filters
	if olderThan != "" {
		t, err := filter.ParseDateOrDuration(olderThan)
		if err != nil {
			return nil, fmt.Errorf("invalid --older-than value: %w", err)
		}
		vf.OlderThan = t
	}

	if newerThan != "" {
		t, err := filter.ParseDateOrDuration(newerThan)
		if err != nil {
			return nil, fmt.Errorf("invalid --newer-than value: %w", err)
		}
		vf.NewerThan = t
	}

	return vf, nil
}

// newListImagesCmd creates the list images subcommand.
func newListImagesCmd() *cobra.Command {
	var (
		jsonOutput    bool
		flatOutput    bool
		outputFormat  string
		filterVersion int64
		filterDigest  string
		filterTag     string
	)

	cmd := &cobra.Command{
		Use:   "images <owner/package>",
		Short: "List images with their artifact relationships",
		Long: `List all images (OCI artifacts) in a package, grouped by their relationships.

Each image includes its platform manifests, attestations (SBOM, provenance, etc.),
and signatures. By default, images are displayed in a tree format showing these
relationships. Use --flat for a simple table view.

Use --version, --digest, or --tag to filter output to only images containing
a specific version.

Examples:
  # List images with relationships (tree view, default)
  ghcrctl list images mkoepf/my-package

  # List images in flat table format
  ghcrctl list images mkoepf/my-package --flat

  # Output in JSON format
  ghcrctl list images mkoepf/my-package --json

  # Filter to images containing a specific tag
  ghcrctl list images mkoepf/my-package --tag v1.0.0

  # Filter to images containing a specific version ID
  ghcrctl list images mkoepf/my-package --version 12345678

  # Filter to images containing a specific digest
  ghcrctl list images mkoepf/my-package --digest sha256:abc123...`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Parse owner/package reference (reject inline tags)
			owner, packageName, err := parsePackageRef(args[0])
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

			// Apply tag filter if specified (resolve tag to digest first)
			if filterTag != "" {
				resolvedDigest, err := discover.ResolveTag(ctx, image, filterTag)
				if err != nil {
					cmd.SilenceUsage = true
					return fmt.Errorf("failed to resolve tag '%s': %w", filterTag, err)
				}
				filterDigest = resolvedDigest
			}

			// Apply version/digest filter if specified
			if filterVersion != 0 || filterDigest != "" {
				var targetDigest string

				if filterVersion != 0 {
					// Find digest by version ID
					found := false
					for _, v := range results {
						if v.ID == filterVersion {
							targetDigest = v.Digest
							found = true
							break
						}
					}
					if !found {
						cmd.SilenceUsage = true
						return fmt.Errorf("version ID %d not found", filterVersion)
					}
				} else {
					// Find full digest from short or full input
					var err error
					targetDigest, err = discover.FindDigestByShortDigest(allVersions, filterDigest)
					if err != nil {
						cmd.SilenceUsage = true
						return err
					}
				}

				// Filter to images containing this version
				results = discover.FindImagesContainingVersion(allVersions, targetDigest)
				if len(results) == 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "No images found containing the specified version\n")
					return nil
				}

				// Rebuild version map with filtered results
				allVersions = make(map[string]discover.VersionInfo)
				for _, v := range results {
					allVersions[v.Digest] = v
				}
			}

			// Output results
			if jsonOutput {
				return display.OutputJSON(cmd.OutOrStdout(), results)
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
	cmd.Flags().Int64Var(&filterVersion, "version", 0, "Filter to images containing this version ID")
	cmd.Flags().StringVar(&filterDigest, "digest", "", "Filter to images containing this digest")
	cmd.Flags().StringVar(&filterTag, "tag", "", "Filter to images containing this tag")
	cmd.MarkFlagsMutuallyExclusive("version", "digest", "tag")

	return cmd
}
