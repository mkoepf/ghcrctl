package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/mkoepf/ghcrctl/internal/discover"
	"github.com/mkoepf/ghcrctl/internal/display"
	"github.com/mkoepf/ghcrctl/internal/gh"
	"github.com/spf13/cobra"
)

// artifactConfig defines the configuration for a specific artifact type command.
type artifactConfig struct {
	// Command metadata
	Name       string // "sbom" or "provenance"
	Short      string // Short description
	Long       string // Long description with examples
	NoFoundMsg string // Message when no artifacts found

	// Artifact filtering
	Role string // OCI artifact role to filter for
}

// newArtifactCmd creates a command for displaying OCI artifacts of a specific type.
// This is a generic factory used by both sbom and provenance commands.
func newArtifactCmd(cfg artifactConfig) *cobra.Command {
	var (
		digest       string
		all          bool
		jsonOutput   bool
		outputFormat string
	)

	cmd := &cobra.Command{
		Use:   cfg.Name + " <owner/package[:tag]>",
		Short: cfg.Short,
		Long:  cfg.Long,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Parse owner/image[:tag] reference
			owner, imageName, tag, err := parseImageRef(args[0])
			if err != nil {
				cmd.SilenceUsage = true
				return err
			}

			// Default to latest if no tag specified
			if tag == "" {
				tag = "latest"
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
			fullImage := fmt.Sprintf("ghcr.io/%s/%s", owner, imageName)

			ctx := cmd.Context()

			// Resolve tag to digest
			resolvedDigest, err := discover.ResolveTag(ctx, fullImage, tag)
			if err != nil {
				cmd.SilenceUsage = true
				return fmt.Errorf("failed to resolve tag '%s': %w", tag, err)
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
			allVersions, err := ghClient.ListPackageVersions(ctx, owner, ownerType, imageName)
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
				return fmt.Errorf("%s for image %s (tag: %s)", cfg.NoFoundMsg, imageName, tag)
			}

			// If specific digest requested, fetch that one
			if digest != "" {
				return fetchAndDisplayArtifact(cmd.OutOrStdout(), ctx, fullImage, digest, jsonOutput, cfg.Name)
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

			return listArtifacts(cmd.OutOrStdout(), artifacts, imageName, cfg.Name)
		},
	}

	cmd.Flags().StringVar(&digest, "digest", "", fmt.Sprintf("Specific %s digest to fetch", cfg.Name))
	cmd.Flags().BoolVar(&all, "all", false, fmt.Sprintf("Show all %s documents", cfg.Name))
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output in JSON format")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "", "Output format (json, table)")

	// Enable dynamic completion for image reference
	cmd.ValidArgsFunction = imageRefValidArgsFunc

	return cmd
}

// fetchAndDisplayArtifact fetches and displays a single artifact
func fetchAndDisplayArtifact(w io.Writer, ctx context.Context, image, digest string, jsonOutput bool, artifactType string) error {
	// Fetch the artifact content
	content, err := discover.FetchArtifactContent(ctx, image, digest)
	if err != nil {
		return fmt.Errorf("failed to fetch %s: %w", artifactType, err)
	}

	// Display the content
	if jsonOutput {
		return display.OutputJSON(w, content)
	}
	return outputArtifactReadable(w, content, digest, artifactType)
}

// fetchAndDisplayAllArtifacts fetches and displays all artifacts
func fetchAndDisplayAllArtifacts(w io.Writer, ctx context.Context, image string, artifacts []discover.VersionInfo, jsonOutput bool, artifactType string) error {
	allContent := make([]interface{}, 0, len(artifacts))

	for _, artifact := range artifacts {
		content, err := discover.FetchArtifactContent(ctx, image, artifact.Digest)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to fetch %s %s: %v\n", artifactType, artifact.Digest, err)
			continue
		}

		if jsonOutput {
			allContent = append(allContent, map[string]interface{}{
				"digest":  artifact.Digest,
				"content": content,
			})
		} else {
			fmt.Fprintf(w, "\n=== %s: %s ===\n", capitalizeFirst(artifactType), display.ShortDigest(artifact.Digest))
			if err := outputArtifactReadable(w, content, artifact.Digest, artifactType); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to display %s %s: %v\n", artifactType, artifact.Digest, err)
			}
		}
	}

	if jsonOutput {
		data, err := json.MarshalIndent(allContent, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Fprintln(w, string(data))
	}

	return nil
}

// listArtifacts lists available artifacts without fetching their content
func listArtifacts(w io.Writer, artifacts []discover.VersionInfo, imageName, artifactType string) error {
	fmt.Fprintf(w, "Multiple %s documents found for %s\n\n", artifactType, imageName)
	fmt.Fprintf(w, "Use --digest <digest> to select one, or --all to show all:\n\n")

	for i, artifact := range artifacts {
		fmt.Fprintf(w, "  %d. %s\n", i+1, artifact.Digest)
	}

	fmt.Fprintf(w, "\nExample: ghcrctl %s %s --digest %s\n", artifactType, imageName, display.ShortDigest(artifacts[0].Digest))

	return nil
}

// outputArtifactReadable outputs artifact content in human-readable format
func outputArtifactReadable(w io.Writer, content []map[string]interface{}, digest, artifactType string) error {
	fmt.Fprintf(w, "%s: %s\n\n", capitalizeFirst(artifactType), display.ShortDigest(digest))

	for _, attestation := range content {
		// Pretty-print the JSON
		data, err := json.MarshalIndent(attestation, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to format content: %w", err)
		}
		fmt.Fprintln(w, string(data))
	}

	return nil
}

// capitalizeFirst returns the string with the first letter capitalized
func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	return string(s[0]-32) + s[1:]
}
