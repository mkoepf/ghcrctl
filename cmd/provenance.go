package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/mkoepf/ghcrctl/internal/display"
	"github.com/mkoepf/ghcrctl/internal/gh"
	"github.com/mkoepf/ghcrctl/internal/oras"
	"github.com/spf13/cobra"
)

// newProvenanceCmd creates the provenance command with isolated flag state.
func newProvenanceCmd() *cobra.Command {
	var (
		digest       string
		all          bool
		jsonOutput   bool
		outputFormat string
	)

	cmd := &cobra.Command{
		Use:   "provenance <owner/image[:tag]>",
		Short: "Display provenance attestation",
		Long: `Display the provenance attestation for a container image. If multiple provenance documents exist, use --digest to select one or --all to show all.

Examples:
  # Show provenance for latest tag
  ghcrctl provenance mkoepf/myimage

  # Show provenance for specific tag
  ghcrctl provenance mkoepf/myimage:v1.0.0

  # Show specific provenance by digest
  ghcrctl provenance mkoepf/myimage --digest abc123def456

  # Show all provenance documents
  ghcrctl provenance mkoepf/myimage --all

  # Output in JSON format
  ghcrctl provenance mkoepf/myimage --json`,
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

			// Filter for provenance artifacts
			var provenances []oras.ChildArtifact
			for _, child := range children {
				if child.Type.Role == "provenance" {
					provenances = append(provenances, child)
				}
			}

			// Check if no provenance found
			if len(provenances) == 0 {
				cmd.SilenceUsage = true
				return fmt.Errorf("no provenance found for image %s (tag: %s)", imageName, tag)
			}

			// If specific digest requested, fetch that one
			if digest != "" {
				return fetchAndDisplayProvenance(cmd.OutOrStdout(), ctx, fullImage, digest, jsonOutput, token)
			}

			// If --all flag, show all provenances
			if all {
				return fetchAndDisplayAllProvenances(cmd.OutOrStdout(), ctx, fullImage, provenances, jsonOutput, token)
			}

			// Smart behavior: if only one provenance, show it; otherwise list them
			if len(provenances) == 1 {
				return fetchAndDisplayProvenance(cmd.OutOrStdout(), ctx, fullImage, provenances[0].Digest, jsonOutput, token)
			}

			// Multiple provenances: if JSON output requested, show all; otherwise list them
			if jsonOutput {
				return fetchAndDisplayAllProvenances(cmd.OutOrStdout(), ctx, fullImage, provenances, jsonOutput, token)
			}

			return listProvenances(cmd.OutOrStdout(), provenances, imageName)
		},
	}

	cmd.Flags().StringVar(&digest, "digest", "", "Specific provenance digest to fetch")
	cmd.Flags().BoolVar(&all, "all", false, "Show all provenance documents")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output in JSON format")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "", "Output format (json, table)")

	// Enable dynamic completion for image reference
	cmd.ValidArgsFunction = imageRefValidArgsFunc

	return cmd
}

// fetchAndDisplayProvenance fetches and displays a single provenance
func fetchAndDisplayProvenance(w io.Writer, ctx context.Context, image, digest string, jsonOutput bool, token string) error {
	// Fetch the provenance content
	content, err := oras.FetchArtifactContent(ctx, image, digest)
	if err != nil {
		return fmt.Errorf("failed to fetch provenance: %w", err)
	}

	// Display the content
	if jsonOutput {
		return display.OutputJSON(w, content)
	}
	return outputProvenanceReadable(w, content, digest)
}

// fetchAndDisplayAllProvenances fetches and displays all provenances
func fetchAndDisplayAllProvenances(w io.Writer, ctx context.Context, image string, provenances []oras.ChildArtifact, jsonOutput bool, token string) error {
	allContent := make([]interface{}, 0, len(provenances))

	for _, prov := range provenances {
		content, err := oras.FetchArtifactContent(ctx, image, prov.Digest)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to fetch provenance %s: %v\n", prov.Digest, err)
			continue
		}

		if jsonOutput {
			allContent = append(allContent, map[string]interface{}{
				"digest":  prov.Digest,
				"content": content,
			})
		} else {
			fmt.Fprintf(w, "\n=== Provenance: %s ===\n", shortProvenanceDigest(prov.Digest))
			if err := outputProvenanceReadable(w, content, prov.Digest); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to display provenance %s: %v\n", prov.Digest, err)
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

// listProvenances lists available provenances without fetching their content
func listProvenances(w io.Writer, provenances []oras.ChildArtifact, imageName string) error {
	fmt.Fprintf(w, "Multiple provenance documents found for %s\n\n", imageName)
	fmt.Fprintf(w, "Use --digest <digest> to select one, or --all to show all:\n\n")

	for i, prov := range provenances {
		fmt.Fprintf(w, "  %d. %s\n", i+1, prov.Digest)
	}

	fmt.Fprintf(w, "\nExample: ghcrctl provenance %s --digest %s\n", imageName, shortProvenanceDigest(provenances[0].Digest))

	return nil
}

// outputProvenanceReadable outputs provenance content in human-readable format
func outputProvenanceReadable(w io.Writer, content []map[string]interface{}, digest string) error {
	fmt.Fprintf(w, "Provenance: %s\n\n", shortProvenanceDigest(digest))

	// Try to extract key information from the provenance
	// Provenance format varies (SLSA v0.2, v1.0, etc.), so we'll do our best
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

// shortProvenanceDigest returns a shortened version of a digest for display
func shortProvenanceDigest(digest string) string {
	// Remove sha256: prefix and take first 12 characters
	digest = strings.TrimPrefix(digest, "sha256:")
	if len(digest) > 12 {
		return digest[:12]
	}
	return digest
}
