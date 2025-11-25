package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/mhk/ghcrctl/internal/config"
	"github.com/mhk/ghcrctl/internal/gh"
	"github.com/mhk/ghcrctl/internal/oras"
	"github.com/spf13/cobra"
)

var (
	provenanceTag    string
	provenanceDigest string
	provenanceAll    bool
	provenanceJSON   bool
)

var provenanceCmd = &cobra.Command{
	Use:   "provenance <image>",
	Short: "Display provenance attestation",
	Long:  `Display the provenance attestation for a container image. If multiple provenance documents exist, use --digest to select one or --all to show all.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		imageName := args[0]

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

		// Construct full image reference
		fullImage := fmt.Sprintf("ghcr.io/%s/%s", owner, imageName)

		// Resolve tag to digest
		ctx := cmd.Context()
		digest, err := oras.ResolveTag(ctx, fullImage, provenanceTag)
		if err != nil {
			cmd.SilenceUsage = true
			return fmt.Errorf("failed to resolve tag '%s': %w", provenanceTag, err)
		}

		// Discover referrers
		referrers, err := oras.DiscoverReferrers(ctx, fullImage, digest)
		if err != nil {
			cmd.SilenceUsage = true
			return fmt.Errorf("failed to discover referrers: %w", err)
		}

		// Filter for provenance artifacts
		var provenances []oras.ReferrerInfo
		for _, ref := range referrers {
			if ref.ArtifactType == "provenance" {
				provenances = append(provenances, ref)
			}
		}

		// Check if no provenance found
		if len(provenances) == 0 {
			cmd.SilenceUsage = true
			return fmt.Errorf("no provenance found for image %s (tag: %s)", imageName, provenanceTag)
		}

		// If specific digest requested, fetch that one
		if provenanceDigest != "" {
			return fetchAndDisplayProvenance(cmd.OutOrStdout(), ctx, fullImage, provenanceDigest, provenanceJSON, token)
		}

		// If --all flag, show all provenances
		if provenanceAll {
			return fetchAndDisplayAllProvenances(cmd.OutOrStdout(), ctx, fullImage, provenances, provenanceJSON, token)
		}

		// Smart behavior: if only one provenance, show it; otherwise list them
		if len(provenances) == 1 {
			return fetchAndDisplayProvenance(cmd.OutOrStdout(), ctx, fullImage, provenances[0].Digest, provenanceJSON, token)
		}

		// Multiple provenances: if JSON output requested, show all; otherwise list them
		if provenanceJSON {
			return fetchAndDisplayAllProvenances(cmd.OutOrStdout(), ctx, fullImage, provenances, provenanceJSON, token)
		}

		return listProvenances(cmd.OutOrStdout(), provenances, imageName)
	},
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
		return outputProvenanceJSON(w, content)
	}
	return outputProvenanceReadable(w, content, digest)
}

// fetchAndDisplayAllProvenances fetches and displays all provenances
func fetchAndDisplayAllProvenances(w io.Writer, ctx context.Context, image string, provenances []oras.ReferrerInfo, jsonOutput bool, token string) error {
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
func listProvenances(w io.Writer, provenances []oras.ReferrerInfo, imageName string) error {
	fmt.Fprintf(w, "Multiple provenance documents found for %s\n\n", imageName)
	fmt.Fprintf(w, "Use --digest <digest> to select one, or --all to show all:\n\n")

	for i, prov := range provenances {
		fmt.Fprintf(w, "  %d. %s\n", i+1, prov.Digest)
		if prov.MediaType != "" {
			fmt.Fprintf(w, "     Type: %s\n", prov.MediaType)
		}
	}

	fmt.Fprintf(w, "\nExample: ghcrctl provenance %s --digest %s\n", imageName, shortProvenanceDigest(provenances[0].Digest))

	return nil
}

// outputProvenanceJSON outputs provenance content as JSON
func outputProvenanceJSON(w io.Writer, content []map[string]interface{}) error {
	data, err := json.MarshalIndent(content, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}
	fmt.Fprintln(w, string(data))
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

func init() {
	rootCmd.AddCommand(provenanceCmd)
	provenanceCmd.Flags().StringVar(&provenanceTag, "tag", "latest", "Tag to resolve (default: latest)")
	provenanceCmd.Flags().StringVar(&provenanceDigest, "digest", "", "Specific provenance digest to fetch")
	provenanceCmd.Flags().BoolVar(&provenanceAll, "all", false, "Show all provenance documents")
	provenanceCmd.Flags().BoolVar(&provenanceJSON, "json", false, "Output in JSON format")
}
