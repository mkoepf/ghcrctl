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
	"github.com/mhk/ghcrctl/internal/graph"
	"github.com/mhk/ghcrctl/internal/oras"
	"github.com/spf13/cobra"
)

var (
	graphTag        string
	graphVersion    int64
	graphDigest     string
	graphJSONOutput bool
)

var graphCmd = &cobra.Command{
	Use:   "graph <image>",
	Short: "Display OCI artifact graph",
	Long:  `Display the OCI artifact graph for a container image, including SBOM and provenance.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		imageName := args[0]

		// Validate flag exclusivity - count how many flags were explicitly set
		flagsSet := 0
		tagChanged := cmd.Flags().Changed("tag")
		versionChanged := cmd.Flags().Changed("version")
		digestChanged := cmd.Flags().Changed("digest")

		if tagChanged {
			flagsSet++
		}
		if versionChanged {
			flagsSet++
		}
		if digestChanged {
			flagsSet++
		}

		if flagsSet > 1 {
			cmd.SilenceUsage = true
			return fmt.Errorf("flags --tag, --version, and --digest are mutually exclusive; only one can be used at a time")
		}

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

		ctx := context.Background()
		var digest string

		// Determine how to get the digest based on which flag was used
		if versionChanged {
			// Get digest from version ID
			ghClient, err := gh.NewClient(token)
			if err != nil {
				cmd.SilenceUsage = true
				return fmt.Errorf("failed to create GitHub client: %w", err)
			}

			// Get all versions for the image
			versions, err := ghClient.ListPackageVersions(ctx, owner, ownerType, imageName)
			if err != nil {
				cmd.SilenceUsage = true
				return fmt.Errorf("failed to get package versions: %w", err)
			}

			// Find the version with matching ID
			var foundVersion *gh.PackageVersionInfo
			for i := range versions {
				if versions[i].ID == graphVersion {
					foundVersion = &versions[i]
					break
				}
			}

			if foundVersion == nil {
				cmd.SilenceUsage = true
				return fmt.Errorf("version ID %d not found for image %s", graphVersion, imageName)
			}

			digest = foundVersion.Name
		} else if digestChanged {
			// Use provided digest directly
			digest = graphDigest
			// Normalize digest format (add sha256: prefix if missing)
			if !strings.HasPrefix(digest, "sha256:") {
				digest = "sha256:" + digest
			}
		} else {
			// Default: resolve tag to digest
			digest, err = oras.ResolveTag(ctx, fullImage, graphTag)
			if err != nil {
				cmd.SilenceUsage = true
				return fmt.Errorf("failed to resolve tag '%s': %w", graphTag, err)
			}
		}

		// Create graph with root image
		g, err := graph.NewGraph(digest)
		if err != nil {
			cmd.SilenceUsage = true
			return fmt.Errorf("failed to create graph: %w", err)
		}

		// Get GitHub client to map digest to version ID and fetch all tags
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
			// Fall back to just adding the queried tag
			g.Root.AddTag(graphTag)
		} else {
			g.Root.SetVersionID(versionID)

			// Fetch all tags for this version
			allTags, err := ghClient.GetVersionTags(ctx, owner, ownerType, imageName, versionID)
			if err != nil {
				// Non-fatal: if we can't get all tags, fall back to just the queried tag
				fmt.Fprintf(os.Stderr, "Warning: could not fetch all tags for version: %v\n", err)
				g.Root.AddTag(graphTag)
			} else {
				// Add all tags to the graph
				for _, tag := range allTags {
					g.Root.AddTag(tag)
				}
			}
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
		}

		// Check if this digest has no children (no platforms, no referrers)
		// If so, it might be a child artifact itself, so search for its parent
		if len(platforms) == 0 && len(referrers) == 0 {
			// Try to find the parent graph that contains this digest
			parentDigest, err := findParentDigest(ctx, ghClient, owner, ownerType, imageName, fullImage, digest)
			if err == nil && parentDigest != "" && parentDigest != digest {
				// Found a parent! Use the parent digest instead
				digest = parentDigest

				// Recreate graph with parent digest
				g, err = graph.NewGraph(digest)
				if err != nil {
					cmd.SilenceUsage = true
					return fmt.Errorf("failed to create graph with parent digest: %w", err)
				}

				// Map parent digest to version ID and fetch tags
				versionID, err := ghClient.GetVersionIDByDigest(ctx, owner, ownerType, imageName, digest)
				if err == nil {
					g.Root.SetVersionID(versionID)
					allTags, err := ghClient.GetVersionTags(ctx, owner, ownerType, imageName, versionID)
					if err == nil {
						for _, tag := range allTags {
							g.Root.AddTag(tag)
						}
					}
				}

				// Get platforms and referrers for the parent
				platforms, _ = oras.GetPlatformManifests(ctx, fullImage, digest)
				referrers, _ = oras.DiscoverReferrers(ctx, fullImage, digest)
			}
		}

		if len(referrers) > 0 {
			// Add referrers to graph
			for _, ref := range referrers {
				artifact, err := graph.NewArtifact(ref.Digest, ref.ArtifactType)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Warning: skipping invalid referrer: %v\n", err)
					continue
				}

				// Try to get version ID and all tags for this referrer
				refVersionID, err := ghClient.GetVersionIDByDigest(ctx, owner, ownerType, imageName, ref.Digest)
				if err == nil {
					artifact.SetVersionID(refVersionID)

					// Fetch all tags for this referrer version
					refTags, err := ghClient.GetVersionTags(ctx, owner, ownerType, imageName, refVersionID)
					if err == nil {
						for _, tag := range refTags {
							artifact.AddTag(tag)
						}
					}
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

func formatTags(tags []string) string {
	if len(tags) == 0 {
		return "[]"
	}
	result := "["
	for i, tag := range tags {
		if i > 0 {
			result += ", "
		}
		result += tag
	}
	result += "]"
	return result
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
		fmt.Fprintf(w, "  Tags: %s\n", formatTags(g.Root.Tags))
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

// findParentDigest searches for a parent digest that references the given child digest
// Returns the parent digest if found, or empty string if not found
func findParentDigest(ctx context.Context, ghClient *gh.Client, owner, ownerType, imageName, fullImage, childDigest string) (string, error) {
	// Get all versions for this image
	versions, err := ghClient.ListPackageVersions(ctx, owner, ownerType, imageName)
	if err != nil {
		return "", fmt.Errorf("failed to list package versions: %w", err)
	}

	// For each version, check if it references the child digest
	for _, ver := range versions {
		candidateDigest := ver.Name

		// Skip if this is the same as the child digest
		if candidateDigest == childDigest {
			continue
		}

		// Check if this candidate has the child digest as a platform manifest
		platforms, err := oras.GetPlatformManifests(ctx, fullImage, candidateDigest)
		if err == nil {
			for _, p := range platforms {
				if p.Digest == childDigest {
					// Found a parent! This candidate has the child as a platform manifest
					return candidateDigest, nil
				}
			}
		}

		// Check if this candidate has the child digest as a referrer (attestation)
		referrers, err := oras.DiscoverReferrers(ctx, fullImage, candidateDigest)
		if err == nil {
			for _, r := range referrers {
				if r.Digest == childDigest {
					// Found a parent! This candidate has the child as a referrer
					return candidateDigest, nil
				}
			}
		}
	}

	// No parent found
	return "", nil
}

func init() {
	rootCmd.AddCommand(graphCmd)
	graphCmd.Flags().StringVar(&graphTag, "tag", "latest", "Tag to resolve (default: latest)")
	graphCmd.Flags().Int64Var(&graphVersion, "version", 0, "Version ID to find graph for")
	graphCmd.Flags().StringVar(&graphDigest, "digest", "", "Digest to find graph for (accepts sha256:... or just the hash)")
	graphCmd.Flags().BoolVar(&graphJSONOutput, "json", false, "Output in JSON format")
}
