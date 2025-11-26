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
	graphTag          string
	graphVersion      int64
	graphDigest       string
	graphJSONOutput   bool
	graphOutputFormat string
)

var graphCmd = &cobra.Command{
	Use:   "graph <image>",
	Short: "Display OCI artifact graph",
	Long: `Display the OCI artifact graph for a container image, including SBOM and provenance.

Examples:
  # Show graph for latest tag
  ghcrctl graph myimage

  # Show graph for specific tag
  ghcrctl graph myimage --tag v1.0.0

  # Show graph by digest
  ghcrctl graph myimage --digest sha256:abc123...

  # Show graph by version ID
  ghcrctl graph myimage --version 12345678

  # Output in JSON format
  ghcrctl graph myimage --json`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		imageName := args[0]

		// Handle output format flag (-o)
		if graphOutputFormat != "" {
			switch graphOutputFormat {
			case "json":
				graphJSONOutput = true
			case "table":
				graphJSONOutput = false
			default:
				cmd.SilenceUsage = true
				return fmt.Errorf("invalid output format %q. Supported formats: json, table", graphOutputFormat)
			}
		}

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

		ctx := cmd.Context()
		var digest string

		// Determine how to get the digest based on which flag was used
		if versionChanged {
			// Get digest from version ID
			ghClient, err := gh.NewClientWithContext(cmd.Context(), token)
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
		ghClient, err := gh.NewClientWithContext(cmd.Context(), token)
		if err != nil {
			cmd.SilenceUsage = true
			return fmt.Errorf("failed to create GitHub client: %w", err)
		}

		// Fetch all versions once and cache them for lookups
		// This avoids redundant API calls (5× → 1×)
		allVersions, err := ghClient.ListPackageVersions(ctx, owner, ownerType, imageName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not list package versions: %v\n", err)
			allVersions = []gh.PackageVersionInfo{} // Empty list, lookups will fail gracefully
		}

		// Create digest→version map for O(1) lookups
		versionCache := make(map[string]gh.PackageVersionInfo)
		for _, ver := range allVersions {
			versionCache[ver.Name] = ver
		}

		// Map digest to version ID using cache
		// Note: The cache already includes tags from ListPackageVersions,
		// so we don't need to make additional GetVersionTags API calls
		versionInfo, found := versionCache[digest]
		if !found {
			// Non-fatal: version ID is optional
			fmt.Fprintf(os.Stderr, "Warning: could not map digest %s to version ID\n", digest)
			// Fall back to just adding the queried tag
			g.Root.AddTag(graphTag)
		} else {
			g.Root.SetVersionID(versionInfo.ID)

			// Use tags from cached version data (already fetched by ListPackageVersions)
			// This eliminates the need for separate GetVersionTags API calls
			if len(versionInfo.Tags) > 0 {
				for _, tag := range versionInfo.Tags {
					g.Root.AddTag(tag)
				}
			} else {
				// If no tags in cache, fall back to the queried tag
				g.Root.AddTag(graphTag)
			}
		}

		// Get platform manifests (if multi-arch image)
		platformInfos, err := oras.GetPlatformManifests(ctx, fullImage, digest)
		if err != nil {
			// Non-fatal: platforms are optional
			fmt.Fprintf(os.Stderr, "Warning: could not get platform manifests: %v\n", err)
		}

		// Discover referrers to check if this digest has children
		// This helps us determine if we need to find a parent digest
		initialReferrers, err := oras.DiscoverReferrers(ctx, fullImage, digest)
		if err != nil {
			// Non-fatal: referrers are optional
			fmt.Fprintf(os.Stderr, "Warning: could not discover referrers: %v\n", err)
		}

		// Check if this digest has no children (no platforms, no referrers)
		// If so, it might be a child artifact itself, so search for its parent
		if len(platformInfos) == 0 && len(initialReferrers) == 0 {
			// Try to find the parent graph that contains this digest
			parentDigest, err := findParentDigest(ctx, allVersions, fullImage, digest)
			if err == nil && parentDigest != "" && parentDigest != digest {
				// Found a parent! Use the parent digest instead
				digest = parentDigest

				// Recreate graph with parent digest
				g, err = graph.NewGraph(digest)
				if err != nil {
					cmd.SilenceUsage = true
					return fmt.Errorf("failed to create graph with parent digest: %w", err)
				}

				// Map parent digest to version ID and use cached tags
				if parentVersionInfo, found := versionCache[digest]; found {
					g.Root.SetVersionID(parentVersionInfo.ID)
					// Use tags from cached version data (no API call needed)
					for _, tag := range parentVersionInfo.Tags {
						g.Root.AddTag(tag)
					}
				}

				// Get platforms for the parent
				platformInfos, _ = oras.GetPlatformManifests(ctx, fullImage, digest)
			}
		}

		// If multi-arch image, create platform objects and discover their referrers
		if len(platformInfos) > 0 {
			// Create platform objects and discover platform-specific referrers
			for _, pInfo := range platformInfos {
				// Create platform object
				platform := graph.NewPlatform(pInfo.Digest, pInfo.Platform, pInfo.Architecture, pInfo.OS, pInfo.Variant)
				platform.Size = pInfo.Size

				// Try to get version ID for platform manifest
				platformVersionID, err := ghClient.GetVersionIDByDigest(ctx, owner, ownerType, imageName, pInfo.Digest)
				if err == nil {
					platform.Manifest.SetVersionID(platformVersionID)
				}

				// Discover platform-specific referrers (attestations that directly reference this platform manifest)
				platformReferrers, err := oras.DiscoverReferrers(ctx, fullImage, pInfo.Digest)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Warning: could not discover referrers for platform %s: %v\n", pInfo.Platform, err)
				} else {
					// Process all referrers found for this specific platform
					for _, ref := range platformReferrers {
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

						platform.AddReferrer(artifact)
					}
				}

				g.AddPlatform(platform)
			}

			// Also discover index-level referrers (attestations that reference the image index itself)
			// These are shown at the root level, not under individual platforms
			indexReferrers, err := oras.DiscoverReferrers(ctx, fullImage, digest)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: could not discover index-level referrers: %v\n", err)
			} else {
				for _, ref := range indexReferrers {
					artifact, err := graph.NewArtifact(ref.Digest, ref.ArtifactType)
					if err != nil {
						fmt.Fprintf(os.Stderr, "Warning: skipping invalid index referrer: %v\n", err)
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
		} else {
			// Single-arch image: discover referrers for the root manifest
			referrers, err := oras.DiscoverReferrers(ctx, fullImage, digest)
			if err != nil {
				// Non-fatal: referrers are optional
				fmt.Fprintf(os.Stderr, "Warning: could not discover referrers: %v\n", err)
			} else {
				// Add referrers to graph root
				for _, ref := range referrers {
					artifact, err := graph.NewArtifact(ref.Digest, ref.ArtifactType)
					if err != nil {
						fmt.Fprintf(os.Stderr, "Warning: skipping invalid referrer: %v\n", err)
						continue
					}

					// Try to get version ID and all tags for this referrer using cache
					if refVersionInfo, found := versionCache[ref.Digest]; found {
						artifact.SetVersionID(refVersionInfo.ID)

						// Use tags from cached version data (no API call needed)
						for _, tag := range refVersionInfo.Tags {
							artifact.AddTag(tag)
						}
					}

					g.AddReferrer(artifact)
				}
			}
		}

		// Output results
		if graphJSONOutput {
			return outputGraphJSON(cmd.OutOrStdout(), g)
		}
		return outputGraphTree(cmd.OutOrStdout(), g, imageName)
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
	}

	// Add platforms with nested referrers for multi-arch images
	if len(g.Platforms) > 0 {
		platforms := []map[string]interface{}{}
		for _, p := range g.Platforms {
			platform := map[string]interface{}{
				"platform":     p.Platform,
				"digest":       p.Manifest.Digest,
				"size":         p.Size,
				"os":           p.OS,
				"architecture": p.Architecture,
				"variant":      p.Variant,
				"version_id":   p.Manifest.VersionID,
				"referrers":    []map[string]interface{}{},
			}

			// Add platform-specific referrers
			for _, ref := range p.Referrers {
				platform["referrers"] = append(platform["referrers"].([]map[string]interface{}), map[string]interface{}{
					"digest":     ref.Digest,
					"type":       ref.Type,
					"tags":       ref.Tags,
					"version_id": ref.VersionID,
				})
			}

			platforms = append(platforms, platform)
		}
		output["platforms"] = platforms
	}

	// Add root-level referrers for single-arch images
	if len(g.Referrers) > 0 {
		referrers := []map[string]interface{}{}
		for _, ref := range g.Referrers {
			referrers = append(referrers, map[string]interface{}{
				"digest":     ref.Digest,
				"type":       ref.Type,
				"tags":       ref.Tags,
				"version_id": ref.VersionID,
			})
		}
		output["referrers"] = referrers
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

func outputGraphTree(w io.Writer, g *graph.Graph, imageName string) error {
	fmt.Fprintf(w, "OCI Artifact Graph for %s\n\n", imageName)

	// Determine if multi-arch
	isMultiArch := len(g.Platforms) > 0

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

	// Display platforms with nested attestations (for multi-arch images)
	if len(g.Platforms) > 0 {
		hasIndexReferrers := len(g.Referrers) > 0
		fmt.Fprintf(w, "  │\n")
		for i, p := range g.Platforms {
			isLastPlatform := i == len(g.Platforms)-1 && !hasIndexReferrers
			platformConnector := "├"
			platformPrefix := "│  "
			if isLastPlatform {
				platformConnector = "└"
				platformPrefix = "   "
			}

			shortDigest := p.Manifest.Digest
			if len(shortDigest) > 19 {
				shortDigest = shortDigest[:19] + "..."
			}

			fmt.Fprintf(w, "  %s─ Platform: %s\n", platformConnector, p.Platform)
			fmt.Fprintf(w, "  %s   Digest: %s\n", platformPrefix, shortDigest)
			if p.Size > 0 {
				fmt.Fprintf(w, "  %s   Size: %d bytes\n", platformPrefix, p.Size)
			}
			if p.Manifest.VersionID != 0 {
				fmt.Fprintf(w, "  %s   Version ID: %d\n", platformPrefix, p.Manifest.VersionID)
			}

			// Display platform-specific attestations
			if len(p.Referrers) > 0 {
				fmt.Fprintf(w, "  %s   │\n", platformPrefix)
				fmt.Fprintf(w, "  %s   └─ Attestations (referrers):\n", platformPrefix)
				for j, ref := range p.Referrers {
					isLastRef := j == len(p.Referrers)-1
					refConnector := "├"
					refPrefix := "│"
					if isLastRef {
						refConnector = "└"
						refPrefix = " "
					}

					refShortDigest := ref.Digest
					if len(refShortDigest) > 19 {
						refShortDigest = refShortDigest[:19] + "..."
					}

					fmt.Fprintf(w, "  %s        %s─ %s\n", platformPrefix, refConnector, ref.Type)
					fmt.Fprintf(w, "  %s        %s    Digest: %s\n", platformPrefix, refPrefix, refShortDigest)
					if ref.VersionID != 0 {
						fmt.Fprintf(w, "  %s        %s    Version ID: %d\n", platformPrefix, refPrefix, ref.VersionID)
					}
				}
			}

			// Add spacing between platforms
			if !isLastPlatform || hasIndexReferrers {
				fmt.Fprintf(w, "  %s\n", platformPrefix)
			}
		}

		// Display index-level attestations (for multiarch images)
		if hasIndexReferrers {
			fmt.Fprintf(w, "  │\n")
			fmt.Fprintf(w, "  └─ Index-level Attestations (referrers):\n")
			for i, ref := range g.Referrers {
				isLast := i == len(g.Referrers)-1

				connector := "├"
				continuePrefix := "│"
				if isLast {
					connector = "└"
					continuePrefix = " "
				}

				shortDigest := ref.Digest
				if len(shortDigest) > 19 {
					shortDigest = shortDigest[:19] + "..."
				}

				fmt.Fprintf(w, "     %s─ %s\n", connector, ref.Type)
				fmt.Fprintf(w, "     %s  Digest: %s\n", continuePrefix, shortDigest)
				if ref.VersionID != 0 {
					fmt.Fprintf(w, "     %s  Version ID: %d\n", continuePrefix, ref.VersionID)
				}
			}
		}
	}

	// Display root-level attestations (for single-arch images only)
	// For multiarch images, these are already shown as "Index-level Attestations" above
	if len(g.Referrers) > 0 && len(g.Platforms) == 0 {
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
		fmt.Fprintf(w, "  Platforms: %d\n", len(g.Platforms))

		// Count total referrers: index-level + platform-specific
		totalReferrers := len(g.Referrers)
		for _, p := range g.Platforms {
			totalReferrers += len(p.Referrers)
		}

		totalVersions := 1 + len(g.Platforms) + totalReferrers
		untaggedVersions := len(g.Platforms) + totalReferrers
		fmt.Fprintf(w, "  SBOM: %v\n", g.HasSBOM())
		fmt.Fprintf(w, "  Provenance: %v\n", g.HasProvenance())
		fmt.Fprintf(w, "  Total versions: %d (%d tagged, %d untagged)\n", totalVersions, 1, untaggedVersions)
	} else {
		fmt.Fprintf(w, "  Type: Single-arch image\n")
		fmt.Fprintf(w, "  SBOM: %v\n", g.HasSBOM())
		fmt.Fprintf(w, "  Provenance: %v\n", g.HasProvenance())
		totalVersions := 1 + len(g.Referrers)
		untaggedVersions := len(g.Referrers)
		fmt.Fprintf(w, "  Total versions: %d (%d tagged, %d untagged)\n", totalVersions, 1, untaggedVersions)
	}

	return nil
}

// sortByIDProximity sorts versions by their ID proximity to a target ID
// Versions with IDs closest to targetID come first
func sortByIDProximity(versions []gh.PackageVersionInfo, targetID int64) []gh.PackageVersionInfo {
	// Create a copy to avoid modifying the original slice
	sorted := make([]gh.PackageVersionInfo, len(versions))
	copy(sorted, versions)

	// Sort by absolute distance from targetID
	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			distI := sorted[i].ID - targetID
			if distI < 0 {
				distI = -distI
			}
			distJ := sorted[j].ID - targetID
			if distJ < 0 {
				distJ = -distJ
			}
			if distJ < distI {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	return sorted
}

// findParentDigest searches for a parent digest that references the given child digest
// Returns the parent digest if found, or empty string if not found
// Uses pre-fetched versions list to avoid redundant API calls
func findParentDigest(ctx context.Context, versions []gh.PackageVersionInfo, fullImage, childDigest string) (string, error) {
	// Find the child version's ID to optimize search order
	var childID int64
	for _, ver := range versions {
		if ver.Name == childDigest {
			childID = ver.ID
			break
		}
	}

	// Sort versions by proximity to child ID - related artifacts are typically created together
	// and have IDs within a small range (typically ±4-20)
	if childID != 0 {
		versions = sortByIDProximity(versions, childID)
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
	graphCmd.Flags().StringVarP(&graphOutputFormat, "output", "o", "", "Output format (json, table)")
}
