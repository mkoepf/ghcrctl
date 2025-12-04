package cmd

import (
	"context"
	"fmt"
	"io"
	"sort"

	"github.com/mkoepf/ghcrctl/internal/discover"
	"github.com/mkoepf/ghcrctl/internal/display"
	"github.com/mkoepf/ghcrctl/internal/gh"
	"github.com/mkoepf/ghcrctl/internal/quiet"
	"github.com/spf13/cobra"
)

// newGetCmd creates the get parent command with its subcommands.
func newGetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get attributes of a package version (labels, sbom, provenance)",
		Long: `Get attributes of a specific package version from GitHub Container Registry.

Requires a selector flag to identify the version: --tag, --digest, or --version.

Available subcommands:
  labels       Get OCI labels from a container image
  sbom         Get SBOM (Software Bill of Materials) attestation
  provenance   Get provenance attestation`,
	}

	cmd.AddCommand(newGetLabelsCmd())
	cmd.AddCommand(newGetSBOMCmd())
	cmd.AddCommand(newGetProvenanceCmd())

	return cmd
}

// newGetLabelsCmd creates the get labels subcommand.
func newGetLabelsCmd() *cobra.Command {
	var (
		tag          string
		digest       string
		versionID    int64
		key          string
		jsonOutput   bool
		outputFormat string
	)

	cmd := &cobra.Command{
		Use:   "labels <owner/package>",
		Short: "Get OCI labels from a container image",
		Long: `Get OCI labels (annotations/metadata) from a container image.

Labels are key-value pairs embedded in the image config at build time using
Docker LABEL instructions or equivalent mechanisms. Common labels include:
  - org.opencontainers.image.source
  - org.opencontainers.image.description
  - org.opencontainers.image.version
  - org.opencontainers.image.licenses

Requires a selector: --tag, --digest, or --version.

Examples:
  # Get all labels from a tagged version
  ghcrctl get labels mkoepf/myimage --tag v1.0.0

  # Get labels by version ID
  ghcrctl get labels mkoepf/myimage --version 12345678

  # Get labels by digest (short form supported)
  ghcrctl get labels mkoepf/myimage --digest abc123

  # Get a specific label key
  ghcrctl get labels mkoepf/myimage --tag v1.0.0 --key org.opencontainers.image.source

  # JSON output
  ghcrctl get labels mkoepf/myimage --tag latest --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Parse owner/package reference (reject inline tags)
			owner, packageName, err := parsePackageRef(args[0])
			if err != nil {
				cmd.SilenceUsage = true
				return err
			}

			// Require at least one selector
			if tag == "" && digest == "" && versionID == 0 {
				cmd.SilenceUsage = true
				return fmt.Errorf("selector required: use --tag, --digest, or --version to specify which version")
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

			// Construct full image reference
			fullImage := fmt.Sprintf("ghcr.io/%s/%s", owner, packageName)

			ctx := cmd.Context()

			// Resolve the selector to a full digest
			var targetDigest string

			if tag != "" {
				targetDigest, err = discover.ResolveTag(ctx, fullImage, tag)
				if err != nil {
					cmd.SilenceUsage = true
					return fmt.Errorf("failed to resolve tag '%s': %w", tag, err)
				}
			} else if versionID != 0 || digest != "" {
				// Need to fetch versions to resolve version ID or short digest
				token, err := gh.GetToken()
				if err != nil {
					cmd.SilenceUsage = true
					return err
				}

				ghClient, err := gh.NewClient(token)
				if err != nil {
					cmd.SilenceUsage = true
					return fmt.Errorf("failed to create GitHub client: %w", err)
				}

				ownerType, err := ghClient.GetOwnerType(ctx, owner)
				if err != nil {
					cmd.SilenceUsage = true
					return fmt.Errorf("failed to determine owner type: %w", err)
				}

				allVersions, err := ghClient.ListPackageVersions(ctx, owner, ownerType, packageName)
				if err != nil {
					cmd.SilenceUsage = true
					return fmt.Errorf("failed to list package versions: %w", err)
				}

				discoverer := discover.NewPackageDiscoverer()
				versions, err := discoverer.DiscoverPackage(ctx, fullImage, allVersions, nil)
				if err != nil {
					cmd.SilenceUsage = true
					return fmt.Errorf("failed to discover package: %w", err)
				}

				versionMap := discover.ToMap(versions)

				if versionID != 0 {
					targetDigest, err = discover.FindDigestByVersionID(versionMap, versionID)
					if err != nil {
						cmd.SilenceUsage = true
						return fmt.Errorf("failed to find version ID %d: %w", versionID, err)
					}
				} else {
					targetDigest, err = discover.FindDigestByShortDigest(versionMap, digest)
					if err != nil {
						cmd.SilenceUsage = true
						return fmt.Errorf("failed to find digest '%s': %w", digest, err)
					}
				}
			}

			// Get labels from image
			labels, err := getImageLabelsFromDigest(ctx, fullImage, targetDigest)
			if err != nil {
				cmd.SilenceUsage = true
				return fmt.Errorf("failed to get labels: %w", err)
			}

			// Filter by key if specified
			if key != "" {
				if value, ok := labels[key]; ok {
					labels = map[string]string{key: value}
				} else {
					cmd.SilenceUsage = true
					return fmt.Errorf("label key %q not found", key)
				}
			}

			// Output results
			if jsonOutput {
				return display.OutputJSON(cmd.OutOrStdout(), labels)
			}
			return outputGetLabelsTable(cmd.OutOrStdout(), labels, packageName, tag, targetDigest)
		},
	}

	cmd.Flags().StringVar(&tag, "tag", "", "Select version by tag")
	cmd.Flags().StringVar(&digest, "digest", "", "Select version by digest (supports short form)")
	cmd.Flags().Int64Var(&versionID, "version", 0, "Select version by ID")
	cmd.Flags().StringVar(&key, "key", "", "Show only specific label key")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output in JSON format")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "", "Output format (json, table)")
	cmd.MarkFlagsMutuallyExclusive("tag", "digest", "version")

	cmd.ValidArgsFunction = imageRefValidArgsFunc

	return cmd
}

func getImageLabelsFromDigest(ctx context.Context, image, digest string) (map[string]string, error) {
	// Fetch image config to get labels
	config, err := discover.GetImageConfig(ctx, image, digest)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch image config: %w", err)
	}

	return config.Config.Labels, nil
}

func outputGetLabelsTable(w io.Writer, labels map[string]string, packageName, tag, digest string) error {
	// Build display string for selector
	selector := display.ShortDigest(digest)
	if tag != "" {
		selector = tag
	}

	if len(labels) == 0 {
		fmt.Fprintf(w, "No labels found for %s (%s)\n", packageName, selector)
		return nil
	}

	fmt.Fprintf(w, "Labels for %s (%s):\n\n", packageName, selector)

	// Sort keys for consistent output
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Find max key length for alignment
	maxKeyLen := 0
	for _, k := range keys {
		if len(k) > maxKeyLen {
			maxKeyLen = len(k)
		}
	}

	// Print labels
	for _, k := range keys {
		fmt.Fprintf(w, "  %-*s  %s\n", maxKeyLen, k, labels[k])
	}

	fmt.Fprintf(w, "\nTotal: %d label(s)\n", len(labels))
	return nil
}

// newGetSBOMCmd creates the get sbom subcommand.
func newGetSBOMCmd() *cobra.Command {
	return newGetArtifactCmd(getArtifactParams{
		Name:       "sbom",
		Short:      "Get SBOM (Software Bill of Materials) attestation",
		NoFoundMsg: "no SBOM found",
		Role:       "sbom",
		Long: `Get the SBOM for a container image or version.

If --digest or --version points directly to an SBOM, it is displayed.
Otherwise, the command finds SBOMs in the image containing that version.
If multiple SBOMs exist, use --all to show all or select a specific one by its digest.

Requires a selector: --tag, --digest, or --version.

Examples:
  # Get SBOM for a tagged image
  ghcrctl get sbom mkoepf/myimage --tag v1.0.0

  # Get SBOM by version ID (from 'list versions' output)
  ghcrctl get sbom mkoepf/myimage --version 12345678

  # Get SBOM by digest (short form supported)
  ghcrctl get sbom mkoepf/myimage --digest abc123

  # Get all SBOMs for an image
  ghcrctl get sbom mkoepf/myimage --tag v1.0.0 --all

  # Output in JSON format
  ghcrctl get sbom mkoepf/myimage --tag v1.0.0 --json`,
	})
}

// newGetProvenanceCmd creates the get provenance subcommand.
func newGetProvenanceCmd() *cobra.Command {
	return newGetArtifactCmd(getArtifactParams{
		Name:       "provenance",
		Short:      "Get provenance attestation",
		NoFoundMsg: "no provenance found",
		Role:       "provenance",
		Long: `Get the provenance attestation for a container image or version.

If --digest or --version points directly to a provenance attestation, it is displayed.
Otherwise, the command finds provenance in the image containing that version.
If multiple provenance documents exist, use --all to show all or select a specific one by its digest.

Requires a selector: --tag, --digest, or --version.

Examples:
  # Get provenance for a tagged image
  ghcrctl get provenance mkoepf/myimage --tag v1.0.0

  # Get provenance by version ID (from 'list versions' output)
  ghcrctl get provenance mkoepf/myimage --version 12345678

  # Get provenance by digest (short form supported)
  ghcrctl get provenance mkoepf/myimage --digest abc123

  # Get all provenance documents for an image
  ghcrctl get provenance mkoepf/myimage --tag v1.0.0 --all

  # Output in JSON format
  ghcrctl get provenance mkoepf/myimage --tag v1.0.0 --json`,
	})
}

// getArtifactParams defines the configuration for a specific artifact type command.
type getArtifactParams struct {
	Name       string // "sbom" or "provenance"
	Short      string // Short description
	Long       string // Long description with examples
	NoFoundMsg string // Message when no artifacts found
	Role       string // OCI artifact role to filter for
}

// newGetArtifactCmd creates a command for getting OCI artifacts of a specific type.
func newGetArtifactCmd(cfg getArtifactParams) *cobra.Command {
	var (
		tag          string
		digest       string
		versionID    int64
		all          bool
		jsonOutput   bool
		outputFormat string
	)

	cmd := &cobra.Command{
		Use:   cfg.Name + " <owner/package>",
		Short: cfg.Short,
		Long:  cfg.Long,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Parse owner/package reference (reject inline tags)
			owner, packageName, err := parsePackageRef(args[0])
			if err != nil {
				cmd.SilenceUsage = true
				return err
			}

			// Require at least one selector
			if tag == "" && digest == "" && versionID == 0 {
				cmd.SilenceUsage = true
				return fmt.Errorf("selector required: use --tag, --digest, or --version to specify which version")
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

			// Verify GitHub token is available
			token, err := gh.GetToken()
			if err != nil {
				cmd.SilenceUsage = true
				return err
			}

			// Construct full image reference
			fullImage := fmt.Sprintf("ghcr.io/%s/%s", owner, packageName)

			ctx := cmd.Context()

			// Create GitHub client to get owner type
			ghClient, err := gh.NewClient(token)
			if err != nil {
				cmd.SilenceUsage = true
				return fmt.Errorf("failed to create GitHub client: %w", err)
			}

			ownerType, err := ghClient.GetOwnerType(ctx, owner)
			if err != nil {
				cmd.SilenceUsage = true
				return fmt.Errorf("failed to determine owner type: %w", err)
			}

			// Get all versions for this package
			allVersions, err := ghClient.ListPackageVersions(ctx, owner, ownerType, packageName)
			if err != nil {
				cmd.SilenceUsage = true
				return fmt.Errorf("failed to list package versions: %w", err)
			}

			// Use discover to get version info with relationships
			discoverer := discover.NewPackageDiscoverer()
			versions, err := discoverer.DiscoverPackage(ctx, fullImage, allVersions, nil)
			if err != nil {
				cmd.SilenceUsage = true
				return fmt.Errorf("failed to discover package: %w", err)
			}

			versionMap := discover.ToMap(versions)

			// Resolve the selector to a full digest
			var resolvedDigest string
			var selectorType string  // "tag", "digest", or "version"
			var selectorValue string // The actual value used

			if tag != "" {
				resolvedDigest, err = discover.ResolveTag(ctx, fullImage, tag)
				if err != nil {
					cmd.SilenceUsage = true
					return fmt.Errorf("failed to resolve tag '%s': %w", tag, err)
				}
				selectorType = "tag"
				selectorValue = tag
			} else if versionID != 0 {
				resolvedDigest, err = discover.FindDigestByVersionID(versionMap, versionID)
				if err != nil {
					cmd.SilenceUsage = true
					return fmt.Errorf("failed to find version ID %d: %w", versionID, err)
				}
				selectorType = "version"
				selectorValue = fmt.Sprintf("%d", versionID)
			} else {
				// Resolve short digest to full digest
				resolvedDigest, err = discover.FindDigestByShortDigest(versionMap, digest)
				if err != nil {
					cmd.SilenceUsage = true
					return fmt.Errorf("failed to find digest '%s': %w", digest, err)
				}
				selectorType = "digest"
				selectorValue = display.ShortDigest(resolvedDigest)
			}

			// Check if the selected version is itself an artifact of the requested type
			selectedVersion, exists := versionMap[resolvedDigest]
			if exists {
				for _, t := range selectedVersion.Types {
					if t == cfg.Role {
						// The selected version IS the artifact - display it directly
						return fetchAndDisplayArtifact(cmd.OutOrStdout(), ctx, fullImage, resolvedDigest, jsonOutput, cfg.Name)
					}
				}
			}

			// The selected version is not an artifact of this type
			// Print informational message about searching in the containing graph
			if !quiet.IsQuiet(ctx) && !jsonOutput {
				fmt.Fprintf(cmd.OutOrStdout(), "Version %s is not a %s. Searching in containing graph...\n\n", selectorValue, cfg.Name)
			}

			// Find the graph it belongs to and look for artifacts there
			graphVersions := discover.FindGraphsContainingVersion(versionMap, resolvedDigest)
			if len(graphVersions) == 0 {
				// Fall back to treating it as a root and looking for children
				graphVersions = discover.FindGraphByDigest(versionMap, resolvedDigest)
			}

			// Filter for artifacts of the specified role
			var artifacts []discover.VersionInfo
			for _, v := range graphVersions {
				// Skip the selected version itself
				if v.Digest == resolvedDigest {
					continue
				}
				// Check if any of the types match the requested role
				for _, t := range v.Types {
					if t == cfg.Role {
						artifacts = append(artifacts, v)
						break
					}
				}
			}

			// Check if no artifacts found
			if len(artifacts) == 0 {
				cmd.SilenceUsage = true
				return fmt.Errorf("%s for %s (%s)", cfg.NoFoundMsg, packageName, selectorValue)
			}

			// If --all flag, show all artifacts
			if all {
				return fetchAndDisplayAllArtifacts(cmd.OutOrStdout(), ctx, fullImage, artifacts, jsonOutput, cfg.Name)
			}

			// Smart behavior: if only one artifact, show it; otherwise list them
			if len(artifacts) == 1 {
				return fetchAndDisplayArtifact(cmd.OutOrStdout(), ctx, fullImage, artifacts[0].Digest, jsonOutput, cfg.Name)
			}

			// Multiple artifacts: if JSON output requested, show all; otherwise list them
			if jsonOutput {
				return fetchAndDisplayAllArtifacts(cmd.OutOrStdout(), ctx, fullImage, artifacts, jsonOutput, cfg.Name)
			}

			return listArtifacts(cmd.OutOrStdout(), artifacts, packageName, cfg.Name, selectorType, selectorValue)
		},
	}

	cmd.Flags().StringVar(&tag, "tag", "", "Select version by tag")
	cmd.Flags().StringVar(&digest, "digest", "", "Select version by digest (supports short form)")
	cmd.Flags().Int64Var(&versionID, "version", 0, "Select version by ID")
	cmd.Flags().BoolVar(&all, "all", false, fmt.Sprintf("Show all %s documents", cfg.Name))
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output in JSON format")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "", "Output format (json, table)")
	cmd.MarkFlagsMutuallyExclusive("tag", "digest", "version")

	cmd.ValidArgsFunction = imageRefValidArgsFunc

	return cmd
}
