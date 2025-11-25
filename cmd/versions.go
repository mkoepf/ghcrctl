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
		client, err := gh.NewClient(token)
		if err != nil {
			cmd.SilenceUsage = true
			return fmt.Errorf("failed to create GitHub client: %w", err)
		}

		ctx := context.Background()

		// List package versions
		versions, err := client.ListPackageVersions(ctx, owner, ownerType, imageName)
		if err != nil {
			cmd.SilenceUsage = true
			return fmt.Errorf("failed to list versions: %w", err)
		}

		// Build graph relationships
		fullImage := fmt.Sprintf("ghcr.io/%s/%s", owner, imageName)
		versionGraphs, err := buildVersionGraphs(ctx, fullImage, versions, client, owner, ownerType, imageName)
		if err != nil {
			cmd.SilenceUsage = true
			return fmt.Errorf("failed to build version graphs: %w", err)
		}

		// Filter graphs by tag if specified
		if versionsTag != "" {
			filtered := []VersionGraph{}
			for _, graph := range versionGraphs {
				for _, tag := range graph.RootVersion.Tags {
					if tag == versionsTag {
						filtered = append(filtered, graph)
						break
					}
				}
			}
			versionGraphs = filtered

			if len(versionGraphs) == 0 {
				cmd.SilenceUsage = true
				return fmt.Errorf("no versions found with tag %q", versionsTag)
			}

			// Also filter versions list for JSON output
			if versionsJSON {
				versions = []gh.PackageVersionInfo{}
				for _, graph := range versionGraphs {
					versions = append(versions, graph.RootVersion)
					for _, child := range graph.Children {
						versions = append(versions, child.Version)
					}
				}
			}
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

			// Discover related versions by resolving the digest
			relatedArtifacts, graphType := discoverRelatedVersions(ctx, fullImage, ver.Tags[0], ver.Name)
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

func discoverRelatedVersions(ctx context.Context, fullImage, tag, rootDigest string) ([]DiscoveredArtifact, string) {
	var artifacts []DiscoveredArtifact
	graphType := "manifest"

	// Resolve tag to get the digest
	digest, err := oras.ResolveTag(ctx, fullImage, tag)
	if err != nil {
		return artifacts, graphType
	}

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
			if tagsStr := formatTags(ver.Tags); len(tagsStr) > maxTagsLen {
				maxTagsLen = len(tagsStr)
			}
			if len(ver.CreatedAt) > maxCreatedLen {
				maxCreatedLen = len(ver.CreatedAt)
			}
		}
	}

	// Print header
	fmt.Fprintf(w, "  %-*s  %-*s  %-*s  %s\n",
		maxIDLen, "VERSION ID",
		maxTypeLen, "TYPE",
		maxTagsLen, "TAGS",
		"CREATED")
	fmt.Fprintf(w, "  %s  %s  %s  %s\n",
		strings.Repeat("-", maxIDLen),
		strings.Repeat("-", maxTypeLen),
		strings.Repeat("-", maxTagsLen),
		strings.Repeat("-", maxCreatedLen))

	// Print graphs
	for i, g := range graphs {
		printVersionGraph(w, g, maxIDLen, maxTypeLen, maxTagsLen, maxCreatedLen)

		// Add blank line between graphs (but not after the last one)
		if i < len(graphs)-1 && len(g.Children) > 0 {
			fmt.Fprintln(w)
		}
	}

	fmt.Fprintf(w, "\nTotal: %d version(s) in %d graph(s)\n", totalVersions, len(graphs))
	return nil
}

func printVersionGraph(w io.Writer, g VersionGraph, maxIDLen, maxTypeLen, maxTagsLen, maxCreatedLen int) {
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
		maxIDLen, maxTypeLen, maxTagsLen, maxCreatedLen)

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

		printVersion(w, indicator, child.Version, childType, maxIDLen, maxTypeLen, maxTagsLen, maxCreatedLen)
	}
}

func printVersion(w io.Writer, indicator string, ver gh.PackageVersionInfo, vtype string, maxIDLen, maxTypeLen, maxTagsLen, maxCreatedLen int) {
	tagsStr := formatTags(ver.Tags)
	fmt.Fprintf(w, "%s %-*d  %-*s  %-*s  %s\n",
		indicator,
		maxIDLen, ver.ID,
		maxTypeLen, vtype,
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
