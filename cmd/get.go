package cmd

import (
	"context"
	"fmt"
	"io"
	"sort"

	"github.com/mkoepf/ghcrctl/internal/discover"
	"github.com/mkoepf/ghcrctl/internal/display"
	"github.com/mkoepf/ghcrctl/internal/gh"
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

Requires a selector: --tag or --digest.

Examples:
  # Get all labels from a tagged version
  ghcrctl get labels mkoepf/myimage --tag v1.0.0

  # Get labels by digest
  ghcrctl get labels mkoepf/myimage --digest sha256:abc123...

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
			if tag == "" && digest == "" {
				cmd.SilenceUsage = true
				return fmt.Errorf("selector required: use --tag or --digest to specify which version")
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

			// Resolve to digest if tag was provided
			targetDigest := digest
			if tag != "" {
				var err error
				targetDigest, err = discover.ResolveTag(ctx, fullImage, tag)
				if err != nil {
					cmd.SilenceUsage = true
					return fmt.Errorf("failed to resolve tag '%s': %w", tag, err)
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
	cmd.Flags().StringVar(&digest, "digest", "", "Select version by digest")
	cmd.Flags().StringVar(&key, "key", "", "Show only specific label key")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output in JSON format")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "", "Output format (json, table)")
	cmd.MarkFlagsMutuallyExclusive("tag", "digest")

	cmd.ValidArgsFunction = imageRefValidArgsFunc

	return cmd
}

func getImageLabelsFromDigest(ctx context.Context, image, digest string) (map[string]string, error) {
	// Fetch image config to get labels
	config, err := discover.FetchImageConfig(ctx, image, digest)
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
	return newGetArtifactCmd(getArtifactConfig{
		Name:       "sbom",
		Short:      "Get SBOM (Software Bill of Materials) attestation",
		NoFoundMsg: "no SBOM found",
		Role:       "sbom",
		Long: `Get the SBOM for a container image.

If multiple SBOMs exist, use --digest to select a specific one or --all to show all.

Requires a selector: --tag or --digest.

Examples:
  # Get SBOM for a tagged version
  ghcrctl get sbom mkoepf/myimage --tag v1.0.0

  # Get SBOM by image digest
  ghcrctl get sbom mkoepf/myimage --digest sha256:abc123...

  # Get specific SBOM by its digest
  ghcrctl get sbom mkoepf/myimage --tag v1.0.0 --sbom-digest abc123def456

  # Get all SBOMs
  ghcrctl get sbom mkoepf/myimage --tag v1.0.0 --all

  # Output in JSON format
  ghcrctl get sbom mkoepf/myimage --tag v1.0.0 --json`,
	})
}

// newGetProvenanceCmd creates the get provenance subcommand.
func newGetProvenanceCmd() *cobra.Command {
	return newGetArtifactCmd(getArtifactConfig{
		Name:       "provenance",
		Short:      "Get provenance attestation",
		NoFoundMsg: "no provenance found",
		Role:       "provenance",
		Long: `Get the provenance attestation for a container image.

If multiple provenance documents exist, use --digest to select a specific one or --all to show all.

Requires a selector: --tag or --digest.

Examples:
  # Get provenance for a tagged version
  ghcrctl get provenance mkoepf/myimage --tag v1.0.0

  # Get provenance by image digest
  ghcrctl get provenance mkoepf/myimage --digest sha256:abc123...

  # Get specific provenance by its digest
  ghcrctl get provenance mkoepf/myimage --tag v1.0.0 --provenance-digest abc123def456

  # Get all provenance documents
  ghcrctl get provenance mkoepf/myimage --tag v1.0.0 --all

  # Output in JSON format
  ghcrctl get provenance mkoepf/myimage --tag v1.0.0 --json`,
	})
}

// getArtifactConfig defines the configuration for a specific artifact type command.
type getArtifactConfig struct {
	Name       string // "sbom" or "provenance"
	Short      string // Short description
	Long       string // Long description with examples
	NoFoundMsg string // Message when no artifacts found
	Role       string // OCI artifact role to filter for
}

// newGetArtifactCmd creates a command for getting OCI artifacts of a specific type.
func newGetArtifactCmd(cfg getArtifactConfig) *cobra.Command {
	var (
		tag            string
		digest         string
		artifactDigest string
		all            bool
		jsonOutput     bool
		outputFormat   string
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
			if tag == "" && digest == "" {
				cmd.SilenceUsage = true
				return fmt.Errorf("selector required: use --tag or --digest to specify which version")
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

			// Resolve to digest if tag was provided
			resolvedDigest := digest
			if tag != "" {
				var err error
				resolvedDigest, err = discover.ResolveTag(ctx, fullImage, tag)
				if err != nil {
					cmd.SilenceUsage = true
					return fmt.Errorf("failed to resolve tag '%s': %w", tag, err)
				}
			}

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

			// Find children of the resolved digest and filter by role
			versionMap := discover.ToMap(versions)
			imageVersions := discover.FindImageByDigest(versionMap, resolvedDigest)

			// Filter for artifacts of the specified role
			var artifacts []discover.VersionInfo
			for _, v := range imageVersions {
				// Skip the root itself
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
				selector := display.ShortDigest(resolvedDigest)
				if tag != "" {
					selector = tag
				}
				return fmt.Errorf("%s for %s (%s)", cfg.NoFoundMsg, packageName, selector)
			}

			// If specific artifact digest requested, fetch that one
			if artifactDigest != "" {
				return fetchAndDisplayArtifact(cmd.OutOrStdout(), ctx, fullImage, artifactDigest, jsonOutput, cfg.Name)
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

			return listArtifacts(cmd.OutOrStdout(), artifacts, packageName, cfg.Name)
		},
	}

	cmd.Flags().StringVar(&tag, "tag", "", "Select version by tag")
	cmd.Flags().StringVar(&digest, "digest", "", "Select version by digest")
	cmd.Flags().StringVar(&artifactDigest, cfg.Name+"-digest", "", fmt.Sprintf("Specific %s digest to fetch", cfg.Name))
	cmd.Flags().BoolVar(&all, "all", false, fmt.Sprintf("Show all %s documents", cfg.Name))
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output in JSON format")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "", "Output format (json, table)")
	cmd.MarkFlagsMutuallyExclusive("tag", "digest")

	cmd.ValidArgsFunction = imageRefValidArgsFunc

	return cmd
}
