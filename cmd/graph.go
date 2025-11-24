package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/mhk/ghcrctl/internal/config"
	"github.com/mhk/ghcrctl/internal/gh"
	"github.com/mhk/ghcrctl/internal/graph"
	"github.com/mhk/ghcrctl/internal/oras"
	"github.com/spf13/cobra"
)

var (
	graphTag        string
	graphJSONOutput bool
)

var graphCmd = &cobra.Command{
	Use:   "graph <image>",
	Short: "Display OCI artifact graph",
	Long:  `Display the OCI artifact graph for a container image, including SBOM and provenance.`,
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
		ctx := context.Background()
		digest, err := oras.ResolveTag(ctx, fullImage, graphTag)
		if err != nil {
			cmd.SilenceUsage = true
			return fmt.Errorf("failed to resolve tag '%s': %w", graphTag, err)
		}

		// Create graph with root image
		g, err := graph.NewGraph(digest)
		if err != nil {
			cmd.SilenceUsage = true
			return fmt.Errorf("failed to create graph: %w", err)
		}

		// Add the tag to the root
		g.Root.AddTag(graphTag)

		// Get GitHub client to map digest to version ID
		ghClient, err := gh.NewClient(token)
		if err != nil {
			cmd.SilenceUsage = true
			return fmt.Errorf("failed to create GitHub client: %w", err)
		}

		// Map digest to version ID
		versionID, err := ghClient.GetVersionIDByDigest(ctx, owner, ownerType, imageName, digest)
		if err != nil {
			// Non-fatal: version ID is optional
			fmt.Fprintf(os.Stderr, "Warning: could not map digest to version ID: %v\n", err)
		} else {
			g.Root.SetVersionID(versionID)
		}

		// Get platform manifests (if multi-arch image)
		platforms, err := oras.GetPlatformManifests(ctx, fullImage, digest)
		if err != nil {
			// Non-fatal: platforms are optional
			fmt.Fprintf(os.Stderr, "Warning: could not get platform manifests: %v\n", err)
		}

		// Discover referrers (SBOM, provenance, etc.)
		referrers, err := oras.DiscoverReferrers(ctx, fullImage, digest)
		if err != nil {
			// Non-fatal: referrers are optional
			fmt.Fprintf(os.Stderr, "Warning: could not discover referrers: %v\n", err)
		} else {
			// Add referrers to graph
			for _, ref := range referrers {
				artifact, err := graph.NewArtifact(ref.Digest, ref.ArtifactType)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Warning: skipping invalid referrer: %v\n", err)
					continue
				}
				g.AddReferrer(artifact)
			}
		}

		// Output results
		if graphJSONOutput {
			return outputGraphJSON(cmd.OutOrStdout(), g, platforms)
		}
		return outputGraphTree(cmd.OutOrStdout(), g, imageName, platforms)
	},
}

func outputGraphJSON(w io.Writer, g *graph.Graph, platforms []oras.PlatformInfo) error {
	// Create JSON-friendly structure
	output := map[string]interface{}{
		"root": map[string]interface{}{
			"digest":     g.Root.Digest,
			"type":       g.Root.Type,
			"tags":       g.Root.Tags,
			"version_id": g.Root.VersionID,
		},
		"platforms": []map[string]interface{}{},
		"referrers": []map[string]interface{}{},
	}

	// Add platform manifests
	for _, p := range platforms {
		output["platforms"] = append(output["platforms"].([]map[string]interface{}), map[string]interface{}{
			"platform":     p.Platform,
			"digest":       p.Digest,
			"size":         p.Size,
			"os":           p.OS,
			"architecture": p.Architecture,
			"variant":      p.Variant,
		})
	}

	// Add referrers
	for _, ref := range g.Referrers {
		output["referrers"] = append(output["referrers"].([]map[string]interface{}), map[string]interface{}{
			"digest":     ref.Digest,
			"type":       ref.Type,
			"tags":       ref.Tags,
			"version_id": ref.VersionID,
		})
	}

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}
	fmt.Fprintln(w, string(data))
	return nil
}

func outputGraphTree(w io.Writer, g *graph.Graph, imageName string, platforms []oras.PlatformInfo) error {
	fmt.Fprintf(w, "OCI Artifact Graph for %s\n\n", imageName)

	// Determine if multi-arch
	isMultiArch := len(platforms) > 0

	// Display root (manifest list or single manifest)
	rootType := "Manifest"
	if isMultiArch {
		rootType = "Image Index"
	}

	fmt.Fprintf(w, "%s: %s\n", rootType, g.Root.Digest)
	if len(g.Root.Tags) > 0 {
		fmt.Fprintf(w, "  Tags: %v\n", g.Root.Tags)
	}
	if g.Root.VersionID != 0 {
		fmt.Fprintf(w, "  Version ID: %d\n", g.Root.VersionID)
	}

	// Display platform manifests (references)
	if len(platforms) > 0 {
		fmt.Fprintf(w, "  │\n")
		fmt.Fprintf(w, "  ├─ Platform Manifests (references):\n")
		for i, p := range platforms {
			isLast := i == len(platforms)-1
			prefix := "│  "
			if isLast && len(g.Referrers) == 0 {
				prefix = "   "
			}

			connector := "├"
			if isLast {
				connector = "└"
			}

			shortDigest := p.Digest
			if len(shortDigest) > 19 {
				shortDigest = shortDigest[:19] + "..."
			}

			fmt.Fprintf(w, "  %s  %s─ %s\n", prefix, connector, p.Platform)
			fmt.Fprintf(w, "  %s     Digest: %s\n", prefix, shortDigest)
			if p.Size > 0 {
				fmt.Fprintf(w, "  %s     Size: %d bytes\n", prefix, p.Size)
			}
		}
	}

	// Display referrers (attestations)
	if len(g.Referrers) > 0 {
		fmt.Fprintf(w, "  │\n")
		fmt.Fprintf(w, "  └─ Attestations (referrers):\n")
		for i, ref := range g.Referrers {
			isLast := i == len(g.Referrers)-1
			prefix := "     "

			connector := "├"
			if isLast {
				connector = "└"
			}

			shortDigest := ref.Digest
			if len(shortDigest) > 19 {
				shortDigest = shortDigest[:19] + "..."
			}

			fmt.Fprintf(w, "  %s  %s─ %s\n", prefix, connector, ref.Type)
			fmt.Fprintf(w, "  %s     Digest: %s\n", prefix, shortDigest)
			if ref.VersionID != 0 {
				fmt.Fprintf(w, "  %s     Version ID: %d\n", prefix, ref.VersionID)
			}
		}
	}

	// Display summary
	fmt.Fprintf(w, "\nSummary:\n")
	if isMultiArch {
		fmt.Fprintf(w, "  Platforms: %d\n", len(platforms))
	} else {
		fmt.Fprintf(w, "  Type: Single-arch image\n")
	}
	fmt.Fprintf(w, "  SBOM: %v\n", g.HasSBOM())
	fmt.Fprintf(w, "  Provenance: %v\n", g.HasProvenance())
	totalVersions := 1 + len(platforms) + len(g.Referrers)
	untaggedVersions := len(platforms) + len(g.Referrers)
	fmt.Fprintf(w, "  Total versions: %d (%d tagged, %d untagged)\n", totalVersions, 1, untaggedVersions)

	return nil
}

func init() {
	rootCmd.AddCommand(graphCmd)
	graphCmd.Flags().StringVar(&graphTag, "tag", "latest", "Tag to resolve (default: latest)")
	graphCmd.Flags().BoolVar(&graphJSONOutput, "json", false, "Output in JSON format")
}
