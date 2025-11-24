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
			return outputGraphJSON(cmd.OutOrStdout(), g)
		}
		return outputGraphTable(cmd.OutOrStdout(), g, imageName)
	},
}

func outputGraphJSON(w io.Writer, g *graph.Graph) error {
	// Create JSON-friendly structure
	output := map[string]interface{}{
		"root": map[string]interface{}{
			"digest":     g.Root.Digest,
			"type":       g.Root.Type,
			"tags":       g.Root.Tags,
			"version_id": g.Root.VersionID,
		},
		"referrers": []map[string]interface{}{},
	}

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

func outputGraphTable(w io.Writer, g *graph.Graph, imageName string) error {
	fmt.Fprintf(w, "OCI Artifact Graph for %s\n\n", imageName)

	// Display root image
	fmt.Fprintf(w, "Image:\n")
	fmt.Fprintf(w, "  Digest: %s\n", g.Root.Digest)
	if len(g.Root.Tags) > 0 {
		fmt.Fprintf(w, "  Tags: %v\n", g.Root.Tags)
	}
	if g.Root.VersionID != 0 {
		fmt.Fprintf(w, "  Version ID: %d\n", g.Root.VersionID)
	}

	// Display referrers
	if len(g.Referrers) == 0 {
		fmt.Fprintf(w, "\nNo referrers found (SBOM, provenance, etc.)\n")
	} else {
		fmt.Fprintf(w, "\nReferrers:\n")
		for _, ref := range g.Referrers {
			fmt.Fprintf(w, "\n  %s:\n", ref.Type)
			fmt.Fprintf(w, "    Digest: %s\n", ref.Digest)
			if ref.VersionID != 0 {
				fmt.Fprintf(w, "    Version ID: %d\n", ref.VersionID)
			}
		}
	}

	// Display summary
	fmt.Fprintf(w, "\nSummary:\n")
	fmt.Fprintf(w, "  SBOM: %v\n", g.HasSBOM())
	fmt.Fprintf(w, "  Provenance: %v\n", g.HasProvenance())
	fmt.Fprintf(w, "  Total artifacts: %d\n", g.UniqueArtifactCount())

	return nil
}

func init() {
	rootCmd.AddCommand(graphCmd)
	graphCmd.Flags().StringVar(&graphTag, "tag", "latest", "Tag to resolve (default: latest)")
	graphCmd.Flags().BoolVar(&graphJSONOutput, "json", false, "Output in JSON format")
}
