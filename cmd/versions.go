package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/mhk/ghcrctl/internal/config"
	"github.com/mhk/ghcrctl/internal/gh"
	"github.com/mhk/ghcrctl/internal/oras"
	"github.com/spf13/cobra"
)

var (
	versionsJSON bool
	versionsTag  string
)

var versionsCmd = &cobra.Command{
	Use:   "versions <image>",
	Short: "List all versions of a package",
	Long: `List all versions of a container image package with graph relationships.

This command displays all versions of a package, showing how they relate to each
other in OCI artifact graphs. Versions belonging to the same graph (image index,
platform manifests, attestations) are visually grouped together.

The version ID can be used with the delete command.

Examples:
  # List all versions with graph relationships
  ghcrctl versions myimage

  # List versions for a specific tag
  ghcrctl versions myimage --tag v1.0

  # List versions in JSON format
  ghcrctl versions myimage --json`,
	Args: cobra.ExactArgs(1),
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

		// Create GitHub client
		client, err := gh.NewClientWithContext(cmd.Context(), token)
		if err != nil {
			cmd.SilenceUsage = true
			return fmt.Errorf("failed to create GitHub client: %w", err)
		}

		ctx := cmd.Context()

		// List package versions
		versions, err := client.ListPackageVersions(ctx, owner, ownerType, imageName)
		if err != nil {
			cmd.SilenceUsage = true
			return fmt.Errorf("failed to list versions: %w", err)
		}

		// Optimization: Filter versions by tag BEFORE building graphs
		// This avoids making ORAS API calls for versions we'll filter out anyway
		if versionsTag != "" {
			versions = filterVersionsByTag(versions, versionsTag)
			if len(versions) == 0 {
				cmd.SilenceUsage = true
				return fmt.Errorf("no versions found with tag %q", versionsTag)
			}
		}

		// Build graph relationships (only for filtered versions if tag specified)
		fullImage := fmt.Sprintf("ghcr.io/%s/%s", owner, imageName)
		versionGraphs, err := buildVersionGraphs(ctx, fullImage, versions, client, owner, ownerType, imageName)
		if err != nil {
			cmd.SilenceUsage = true
			return fmt.Errorf("failed to build version graphs: %w", err)
		}

		// Output results
		if versionsJSON {
			return outputVersionsJSON(cmd.OutOrStdout(), versions)
		}
		return outputVersionsTableWithGraphs(cmd.OutOrStdout(), versionGraphs, imageName)
	},
}

// VersionGraph represents a group of related versions
type VersionGraph struct {
	RootVersion gh.PackageVersionInfo
	Children    []VersionChild
	Type        string // "index", "manifest", or "standalone"
}

// VersionChild represents a child version with its artifact type
type VersionChild struct {
	Version      gh.PackageVersionInfo
	ArtifactType string // "platform", "sbom", "provenance", or "attestation"
	Platform     string // e.g., "linux/amd64" for platform manifests
}

// filterVersionsByTag filters versions to only those with the specified tag
// Returns all versions if tagFilter is empty
func filterVersionsByTag(versions []gh.PackageVersionInfo, tagFilter string) []gh.PackageVersionInfo {
	if tagFilter == "" {
		return versions
	}

	filtered := []gh.PackageVersionInfo{}
	for _, ver := range versions {
		for _, tag := range ver.Tags {
			if tag == tagFilter {
				filtered = append(filtered, ver)
				break
			}
		}
	}
	return filtered
}

func buildVersionGraphs(ctx context.Context, fullImage string, versions []gh.PackageVersionInfo, client *gh.Client, owner, ownerType, imageName string) ([]VersionGraph, error) {
	// Map digest to version info for quick lookup
	digestToVersion := make(map[string]gh.PackageVersionInfo)
	for _, ver := range versions {
		digestToVersion[ver.Name] = ver
	}

	// Track which versions have been assigned to a graph
	assigned := make(map[int64]bool)
	var graphs []VersionGraph

	// Process tagged versions first (potential graph roots)
	for _, ver := range versions {
		if len(ver.Tags) > 0 && !assigned[ver.ID] {
			// This is a tagged version - potential graph root
			graph := VersionGraph{
				RootVersion: ver,
				Children:    []VersionChild{},
			}

			assigned[ver.ID] = true

			// Optimization: Use digest directly from ver.Name instead of resolving tag
			// This eliminates redundant ORAS ResolveTag calls (2 OCI calls per version)
			// since GitHub API already provides the digest
			relatedArtifacts, graphType := discoverRelatedVersionsByDigest(ctx, fullImage, ver.Name, ver.Name)
			graph.Type = graphType

			// Find child versions by digest and consolidate artifact types
			childMap := make(map[int64]*VersionChild) // Map version ID to child
			for _, artifact := range relatedArtifacts {
				if childVer, found := digestToVersion[artifact.Digest]; found && !assigned[childVer.ID] {
					if existing, exists := childMap[childVer.ID]; exists {
						// Consolidate artifact types for the same version
						existing.ArtifactType = existing.ArtifactType + ", " + artifact.ArtifactType
					} else {
						childMap[childVer.ID] = &VersionChild{
							Version:      childVer,
							ArtifactType: artifact.ArtifactType,
							Platform:     artifact.Platform,
						}
					}
				}
			}

			// Add children to graph and mark as assigned
			for _, child := range childMap {
				graph.Children = append(graph.Children, *child)
				assigned[child.Version.ID] = true
			}

			graphs = append(graphs, graph)
		}
	}

	// Process untagged versions as potential graph roots
	for _, ver := range versions {
		if len(ver.Tags) == 0 && !assigned[ver.ID] {
			// Try to discover children using the digest directly
			relatedArtifacts, graphType := discoverRelatedVersionsByDigest(ctx, fullImage, ver.Name, ver.Name)

			// Only create a graph if this version has children
			if len(relatedArtifacts) > 0 {
				graph := VersionGraph{
					RootVersion: ver,
					Children:    []VersionChild{},
					Type:        graphType,
				}
				assigned[ver.ID] = true

				// Find child versions by digest
				childMap := make(map[int64]*VersionChild)
				for _, artifact := range relatedArtifacts {
					if childVer, found := digestToVersion[artifact.Digest]; found && !assigned[childVer.ID] {
						if existing, exists := childMap[childVer.ID]; exists {
							existing.ArtifactType = existing.ArtifactType + ", " + artifact.ArtifactType
						} else {
							childMap[childVer.ID] = &VersionChild{
								Version:      childVer,
								ArtifactType: artifact.ArtifactType,
								Platform:     artifact.Platform,
							}
						}
					}
				}

				for _, child := range childMap {
					graph.Children = append(graph.Children, *child)
					assigned[child.Version.ID] = true
				}

				graphs = append(graphs, graph)
			}
		}
	}

	// Add remaining unassigned versions as standalone
	for _, ver := range versions {
		if !assigned[ver.ID] {
			graph := VersionGraph{
				RootVersion: ver,
				Children:    []VersionChild{},
				Type:        "standalone",
			}
			graphs = append(graphs, graph)
		}
	}

	return graphs, nil
}

// DiscoveredArtifact represents a discovered related artifact
type DiscoveredArtifact struct {
	Digest       string
	ArtifactType string // "platform", "sbom", "provenance", "attestation"
	Platform     string // e.g., "linux/amd64" for platform manifests
}

// discoverRelatedVersionsByDigest discovers children using a digest directly
func discoverRelatedVersionsByDigest(ctx context.Context, fullImage, digest, rootDigest string) ([]DiscoveredArtifact, string) {
	var artifacts []DiscoveredArtifact
	graphType := "manifest"

	// Get platform manifests
	platforms, err := oras.GetPlatformManifests(ctx, fullImage, digest)
	if err == nil && len(platforms) > 0 {
		graphType = "index"
		for _, platform := range platforms {
			if platform.Digest != rootDigest {
				artifacts = append(artifacts, DiscoveredArtifact{
					Digest:       platform.Digest,
					ArtifactType: "platform",
					Platform:     platform.Platform,
				})
			}
		}
	}

	// Discover referrers (SBOM, provenance, etc.)
	referrers, err := oras.DiscoverReferrers(ctx, fullImage, digest)
	if err == nil {
		for _, ref := range referrers {
			if ref.Digest != rootDigest {
				artType := "attestation" // default
				if ref.ArtifactType == "sbom" {
					artType = "sbom"
				} else if ref.ArtifactType == "provenance" {
					artType = "provenance"
				}
				artifacts = append(artifacts, DiscoveredArtifact{
					Digest:       ref.Digest,
					ArtifactType: artType,
					Platform:     "",
				})
			}
		}
	}

	return artifacts, graphType
}

func outputVersionsJSON(w io.Writer, versions []gh.PackageVersionInfo) error {
	data, err := json.MarshalIndent(versions, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}
	fmt.Fprintln(w, string(data))
	return nil
}

func outputVersionsTableWithGraphs(w io.Writer, graphs []VersionGraph, imageName string) error {
	if len(graphs) == 0 {
		fmt.Fprintf(w, "No versions found for %s\n", imageName)
		return nil
	}

	// Calculate total versions and graph count
	totalVersions := 0
	for _, g := range graphs {
		totalVersions += 1 + len(g.Children)
	}

	fmt.Fprintf(w, "Versions for %s:\n\n", imageName)

	// Find column widths
	maxIDLen := len("VERSION ID")
	maxTypeLen := len("TYPE")
	maxDigestLen := len("DIGEST")
	maxTagsLen := len("TAGS")
	maxCreatedLen := len("CREATED")

	for _, g := range graphs {
		// Check root version
		ver := g.RootVersion
		if idLen := len(fmt.Sprintf("%d", ver.ID)); idLen > maxIDLen {
			maxIDLen = idLen
		}
		if typeLen := len(determineVersionType(ver, g.Type)); typeLen > maxTypeLen {
			maxTypeLen = typeLen
		}
		digest := shortDigest(ver.Name)
		if len(digest) > maxDigestLen {
			maxDigestLen = len(digest)
		}
		if tagsStr := formatTags(ver.Tags); len(tagsStr) > maxTagsLen {
			maxTagsLen = len(tagsStr)
		}
		if len(ver.CreatedAt) > maxCreatedLen {
			maxCreatedLen = len(ver.CreatedAt)
		}

		// Check children
		for _, child := range g.Children {
			ver := child.Version
			if idLen := len(fmt.Sprintf("%d", ver.ID)); idLen > maxIDLen {
				maxIDLen = idLen
			}
			// Determine child type display
			childType := child.ArtifactType
			if child.Platform != "" {
				childType = child.Platform
			}
			if typeLen := len(childType); typeLen > maxTypeLen {
				maxTypeLen = typeLen
			}
			digest := shortDigest(ver.Name)
			if len(digest) > maxDigestLen {
				maxDigestLen = len(digest)
			}
			if tagsStr := formatTags(ver.Tags); len(tagsStr) > maxTagsLen {
				maxTagsLen = len(tagsStr)
			}
			if len(ver.CreatedAt) > maxCreatedLen {
				maxCreatedLen = len(ver.CreatedAt)
			}
		}
	}

	// Print header
	fmt.Fprintf(w, "  %-*s  %-*s  %-*s  %-*s  %s\n",
		maxIDLen, "VERSION ID",
		maxTypeLen, "TYPE",
		maxDigestLen, "DIGEST",
		maxTagsLen, "TAGS",
		"CREATED")
	fmt.Fprintf(w, "  %s  %s  %s  %s  %s\n",
		strings.Repeat("-", maxIDLen),
		strings.Repeat("-", maxTypeLen),
		strings.Repeat("-", maxDigestLen),
		strings.Repeat("-", maxTagsLen),
		strings.Repeat("-", maxCreatedLen))

	// Print graphs
	for i, g := range graphs {
		printVersionGraph(w, g, maxIDLen, maxTypeLen, maxDigestLen, maxTagsLen, maxCreatedLen)

		// Add blank line between graphs (but not after the last one)
		if i < len(graphs)-1 && len(g.Children) > 0 {
			fmt.Fprintln(w)
		}
	}

	fmt.Fprintf(w, "\nTotal: %d version(s) in %d graph(s)\n", totalVersions, len(graphs))
	return nil
}

func printVersionGraph(w io.Writer, g VersionGraph, maxIDLen, maxTypeLen, maxDigestLen, maxTagsLen, maxCreatedLen int) {
	// Determine tree indicators
	var rootIndicator, midIndicator, lastIndicator string
	if len(g.Children) > 0 {
		rootIndicator = "┌"
		midIndicator = "├"
		lastIndicator = "└"
	} else {
		rootIndicator = " "
	}

	// Print root version
	printVersion(w, rootIndicator, g.RootVersion, determineVersionType(g.RootVersion, g.Type),
		maxIDLen, maxTypeLen, maxDigestLen, maxTagsLen, maxCreatedLen)

	// Print children
	for i, child := range g.Children {
		indicator := midIndicator
		if i == len(g.Children)-1 {
			indicator = lastIndicator
		}

		// Determine child type display
		childType := child.ArtifactType
		if child.Platform != "" {
			childType = child.Platform
		}

		printVersion(w, indicator, child.Version, childType, maxIDLen, maxTypeLen, maxDigestLen, maxTagsLen, maxCreatedLen)
	}
}

func printVersion(w io.Writer, indicator string, ver gh.PackageVersionInfo, vtype string, maxIDLen, maxTypeLen, maxDigestLen, maxTagsLen, maxCreatedLen int) {
	tagsStr := formatTags(ver.Tags)
	digest := shortDigest(ver.Name)
	fmt.Fprintf(w, "%s %-*d  %-*s  %-*s  %-*s  %s\n",
		indicator,
		maxIDLen, ver.ID,
		maxTypeLen, vtype,
		maxDigestLen, digest,
		maxTagsLen, tagsStr,
		ver.CreatedAt)
}

func determineVersionType(ver gh.PackageVersionInfo, graphType string) string {
	if len(ver.Tags) > 0 {
		if graphType == "index" {
			return "index"
		}
		return "manifest"
	}
	return "untagged"
}

func init() {
	rootCmd.AddCommand(versionsCmd)
	versionsCmd.Flags().BoolVar(&versionsJSON, "json", false, "Output in JSON format")
	versionsCmd.Flags().StringVar(&versionsTag, "tag", "", "Filter versions by tag")
}
