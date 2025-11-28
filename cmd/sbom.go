package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/mkoepf/ghcrctl/internal/display"
	"github.com/mkoepf/ghcrctl/internal/gh"
	"github.com/mkoepf/ghcrctl/internal/oras"
	"github.com/spf13/cobra"
)

// newSBOMCmd creates the sbom command with isolated flag state.
func newSBOMCmd() *cobra.Command {
	var (
		digest       string
		all          bool
		jsonOutput   bool
		outputFormat string
	)

	cmd := &cobra.Command{
		Use:   "sbom <owner/image[:tag]>",
		Short: "Display SBOM (Software Bill of Materials)",
		Long: `Display the SBOM for a container image. If multiple SBOMs exist, use --digest to select one or --all to show all.

Examples:
  # Show SBOM for latest tag
  ghcrctl sbom mkoepf/myimage

  # Show SBOM for specific tag
  ghcrctl sbom mkoepf/myimage:v1.0.0

  # Show specific SBOM by digest
  ghcrctl sbom mkoepf/myimage --digest abc123def456

  # Show all SBOMs
  ghcrctl sbom mkoepf/myimage --all

  # Output in JSON format
  ghcrctl sbom mkoepf/myimage --json`,
		Args: cobra.ExactArgs(1),
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

			// Get GitHub token
			token, err := gh.GetToken()
			if err != nil {
				cmd.SilenceUsage = true
				return err
			}

			// Construct full image reference
			fullImage := fmt.Sprintf("ghcr.io/%s/%s", owner, imageName)

			// Resolve tag to digest
			ctx := cmd.Context()
			resolvedDigest, err := oras.ResolveTag(ctx, fullImage, tag)
			if err != nil {
				cmd.SilenceUsage = true
				return fmt.Errorf("failed to resolve tag '%s': %w", tag, err)
			}

			// Discover children
			children, err := oras.DiscoverChildren(ctx, fullImage, resolvedDigest, nil)
			if err != nil {
				cmd.SilenceUsage = true
				return fmt.Errorf("failed to discover children: %w", err)
			}

			// Filter for SBOM artifacts
			var sboms []oras.ChildArtifact
			for _, child := range children {
				if child.Type.Role == "sbom" {
					sboms = append(sboms, child)
				}
			}

			// Check if no SBOMs found
			if len(sboms) == 0 {
				cmd.SilenceUsage = true
				return fmt.Errorf("no SBOM found for image %s (tag: %s)", imageName, tag)
			}

			// If specific digest requested, fetch that one
			if digest != "" {
				return fetchAndDisplaySBOM(cmd.OutOrStdout(), ctx, fullImage, digest, jsonOutput, token)
			}

			// If --all flag, show all SBOMs
			if all {
				return fetchAndDisplayAllSBOMs(cmd.OutOrStdout(), ctx, fullImage, sboms, jsonOutput, token)
			}

			// Smart behavior: if only one SBOM, show it; otherwise list them
			if len(sboms) == 1 {
				return fetchAndDisplaySBOM(cmd.OutOrStdout(), ctx, fullImage, sboms[0].Digest, jsonOutput, token)
			}

			// Multiple SBOMs: if JSON output requested, show all; otherwise list them
			if jsonOutput {
				return fetchAndDisplayAllSBOMs(cmd.OutOrStdout(), ctx, fullImage, sboms, jsonOutput, token)
			}

			return listSBOMs(cmd.OutOrStdout(), sboms, imageName)
		},
	}

	cmd.Flags().StringVar(&digest, "digest", "", "Specific SBOM digest to fetch")
	cmd.Flags().BoolVar(&all, "all", false, "Show all SBOMs")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output in JSON format")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "", "Output format (json, table)")

	// Enable dynamic completion for image reference
	cmd.ValidArgsFunction = imageRefValidArgsFunc

	return cmd
}

// fetchAndDisplaySBOM fetches and displays a single SBOM
func fetchAndDisplaySBOM(w io.Writer, ctx context.Context, image, digest string, jsonOutput bool, token string) error {
	// Fetch the SBOM content
	content, err := oras.FetchArtifactContent(ctx, image, digest)
	if err != nil {
		return fmt.Errorf("failed to fetch SBOM: %w", err)
	}

	// Display the content
	if jsonOutput {
		return display.OutputJSON(w, content)
	}
	return outputSBOMReadable(w, content, digest)
}

// fetchAndDisplayAllSBOMs fetches and displays all SBOMs
func fetchAndDisplayAllSBOMs(w io.Writer, ctx context.Context, image string, sboms []oras.ChildArtifact, jsonOutput bool, token string) error {
	allContent := make([]interface{}, 0, len(sboms))

	for _, sbom := range sboms {
		content, err := oras.FetchArtifactContent(ctx, image, sbom.Digest)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to fetch SBOM %s: %v\n", sbom.Digest, err)
			continue
		}

		if jsonOutput {
			allContent = append(allContent, map[string]interface{}{
				"digest":  sbom.Digest,
				"content": content,
			})
		} else {
			fmt.Fprintf(w, "\n=== SBOM: %s ===\n", display.ShortDigest(sbom.Digest))
			if err := outputSBOMReadable(w, content, sbom.Digest); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to display SBOM %s: %v\n", sbom.Digest, err)
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

// listSBOMs lists available SBOMs without fetching their content
func listSBOMs(w io.Writer, sboms []oras.ChildArtifact, imageName string) error {
	fmt.Fprintf(w, "Multiple SBOMs found for %s\n\n", imageName)
	fmt.Fprintf(w, "Use --digest <digest> to select one, or --all to show all:\n\n")

	for i, sbom := range sboms {
		fmt.Fprintf(w, "  %d. %s\n", i+1, sbom.Digest)
	}

	fmt.Fprintf(w, "\nExample: ghcrctl sbom %s --digest %s\n", imageName, display.ShortDigest(sboms[0].Digest))

	return nil
}

// outputSBOMReadable outputs SBOM content in human-readable format
func outputSBOMReadable(w io.Writer, content []map[string]interface{}, digest string) error {
	fmt.Fprintf(w, "SBOM: %s\n\n", display.ShortDigest(digest))

	// Try to extract key information from the SBOM
	// SBOM format varies (SPDX, CycloneDX, etc.), so we'll do our best
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
