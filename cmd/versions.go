package cmd

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/mhk/ghcrctl/internal/config"
	"github.com/mhk/ghcrctl/internal/discovery"
	"github.com/mhk/ghcrctl/internal/display"
	"github.com/mhk/ghcrctl/internal/filter"
	"github.com/mhk/ghcrctl/internal/gh"
	"github.com/mhk/ghcrctl/internal/oras"
	"github.com/spf13/cobra"
)

var (
	versionsJSON          bool
	versionsVerbose       bool
	versionsTag           string
	versionsTagPattern    string
	versionsOnlyTagged    bool
	versionsOnlyUntagged  bool
	versionsOlderThan     string
	versionsNewerThan     string
	versionsOlderThanDays int
	versionsNewerThanDays int
	versionsOutputFormat  string
	versionsVersionID     int64
	versionsDigest        string
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

  # List only tagged versions
  ghcrctl versions myimage --tagged

  # List only untagged versions
  ghcrctl versions myimage --untagged

  # List versions matching a tag pattern (regex)
  ghcrctl versions myimage --tag-pattern "^v1\\..*"

  # List versions older than a specific date
  ghcrctl versions myimage --older-than 2025-01-01

  # List versions newer than a specific date
  ghcrctl versions myimage --newer-than 2025-11-01

  # List versions older than 30 days
  ghcrctl versions myimage --older-than-days 30

  # Combine filters: untagged versions older than 7 days
  ghcrctl versions myimage --untagged --older-than-days 7

  # Filter by specific version ID
  ghcrctl versions myimage --version 12345678

  # Filter by digest (supports prefix matching)
  ghcrctl versions myimage --digest sha256:abc123

  # List versions in JSON format
  ghcrctl versions myimage --json`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		imageName := args[0]

		// Handle output format flag (-o)
		if versionsOutputFormat != "" {
			switch versionsOutputFormat {
			case "json":
				versionsJSON = true
			case "table":
				versionsJSON = false
			default:
				cmd.SilenceUsage = true
				return fmt.Errorf("invalid output format %q. Supported formats: json, table", versionsOutputFormat)
			}
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

		// Create GitHub client
		client, err := gh.NewClientWithContext(cmd.Context(), token)
		if err != nil {
			cmd.SilenceUsage = true
			return fmt.Errorf("failed to create GitHub client: %w", err)
		}

		ctx := cmd.Context()

		// List package versions
		allVersions, err := client.ListPackageVersions(ctx, owner, ownerType, imageName)
		if err != nil {
			cmd.SilenceUsage = true
			return fmt.Errorf("failed to list versions: %w", err)
		}

		// Build filter from command-line flags
		versionFilter, err := buildVersionFilter()
		if err != nil {
			cmd.SilenceUsage = true
			return fmt.Errorf("invalid filter options: %w", err)
		}

		// Build full image reference
		fullImage := fmt.Sprintf("ghcr.io/%s/%s", owner, imageName)

		// For --untagged filter, discover tagged graph members first
		// This allows the filter to exclude children of tagged versions
		if versionFilter.OnlyUntagged {
			versionFilter.TaggedGraphMembers = identifyTaggedGraphMembers(ctx, fullImage, allVersions)
		}

		// Apply filters to determine which versions to graph
		// Build graphs only for filtered roots, but provide all versions for child lookup
		versionsToGraph := versionFilter.Apply(allVersions)
		if len(versionsToGraph) == 0 {
			cmd.SilenceUsage = true
			return fmt.Errorf("no versions found matching filter criteria")
		}

		// Build graph relationships
		// Pass versionsToGraph (filtered roots) and allVersions (for child lookup)
		versionGraphs, err := buildVersionGraphs(ctx, fullImage, versionsToGraph, allVersions, client, owner, ownerType, imageName)
		if err != nil {
			cmd.SilenceUsage = true
			return fmt.Errorf("failed to build version graphs: %w", err)
		}

		// Output results
		if versionsJSON {
			return display.OutputJSON(cmd.OutOrStdout(), allVersions)
		}
		if versionsVerbose {
			return outputVersionsVerbose(cmd.OutOrStdout(), versionGraphs, imageName)
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
	Size         int64  // Size in bytes (for verbose output)
}

func buildVersionGraphs(ctx context.Context, fullImage string, versionsToGraph []gh.PackageVersionInfo, allVersions []gh.PackageVersionInfo, client *gh.Client, owner, ownerType, imageName string) ([]VersionGraph, error) {
	// Create version cache for efficient lookups (use ALL versions for child discovery)
	cache := discovery.NewVersionCacheFromSlice(allVersions)

	// Track which versions have been assigned to a graph
	assigned := make(map[int64]bool)
	var graphs []VersionGraph

	// Process tagged versions first (potential graph roots)
	for _, ver := range versionsToGraph {
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
				if childVer, found := cache.ByDigest[artifact.Digest]; found && !assigned[childVer.ID] {
					if existing, exists := childMap[childVer.ID]; exists {
						// Consolidate artifact types for the same version
						existing.ArtifactType = existing.ArtifactType + ", " + artifact.ArtifactType
					} else {
						childMap[childVer.ID] = &VersionChild{
							Version:      childVer,
							ArtifactType: artifact.ArtifactType,
							Platform:     artifact.Platform,
							Size:         artifact.Size,
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
	for _, ver := range versionsToGraph {
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
					if childVer, found := cache.ByDigest[artifact.Digest]; found && !assigned[childVer.ID] {
						if existing, exists := childMap[childVer.ID]; exists {
							existing.ArtifactType = existing.ArtifactType + ", " + artifact.ArtifactType
						} else {
							childMap[childVer.ID] = &VersionChild{
								Version:      childVer,
								ArtifactType: artifact.ArtifactType,
								Platform:     artifact.Platform,
								Size:         artifact.Size,
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
	for _, ver := range versionsToGraph {
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
	Size         int64  // Size in bytes
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
					Size:         platform.Size,
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
					Size:         ref.Size,
				})
			}
		}
	}

	return artifacts, graphType
}

// identifyTaggedGraphMembers discovers all versions that belong to tagged graphs.
// This includes tagged versions themselves and all their children (platform manifests,
// attestations, etc.). Used by --untagged filter to exclude children of tagged versions.
func identifyTaggedGraphMembers(ctx context.Context, fullImage string, allVersions []gh.PackageVersionInfo) map[int64]bool {
	members := make(map[int64]bool)
	cache := discovery.NewVersionCacheFromSlice(allVersions)

	for _, ver := range allVersions {
		if len(ver.Tags) > 0 {
			// This is a tagged version - add it and discover its children
			members[ver.ID] = true

			// Discover children using ORAS
			relatedArtifacts, _ := discoverRelatedVersionsByDigest(ctx, fullImage, ver.Name, ver.Name)

			// Add all children to the members set
			for _, artifact := range relatedArtifacts {
				if childVer, found := cache.ByDigest[artifact.Digest]; found {
					members[childVer.ID] = true
				}
			}
		}
	}

	return members
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
		digest := display.ShortDigest(ver.Name)
		if len(digest) > maxDigestLen {
			maxDigestLen = len(digest)
		}
		if tagsStr := display.FormatTags(ver.Tags); len(tagsStr) > maxTagsLen {
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
			digest := display.ShortDigest(ver.Name)
			if len(digest) > maxDigestLen {
				maxDigestLen = len(digest)
			}
			if tagsStr := display.FormatTags(ver.Tags); len(tagsStr) > maxTagsLen {
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
	tagsStr := display.FormatTags(ver.Tags)
	digest := display.ShortDigest(ver.Name)
	fmt.Fprintf(w, "%s %-*d  %-*s  %-*s  %-*s  %s\n",
		indicator,
		maxIDLen, ver.ID,
		maxTypeLen, vtype,
		maxDigestLen, digest,
		maxTagsLen, tagsStr,
		ver.CreatedAt)
}

// outputVersionsVerbose outputs detailed version information
func outputVersionsVerbose(w io.Writer, graphs []VersionGraph, imageName string) error {
	if len(graphs) == 0 {
		fmt.Fprintf(w, "No versions found for %s\n", imageName)
		return nil
	}

	fmt.Fprintf(w, "Versions for %s:\n\n", imageName)

	for i, g := range graphs {
		printVerboseGraph(w, g, i == len(graphs)-1)
	}

	return nil
}

func printVerboseGraph(w io.Writer, g VersionGraph, isLast bool) {
	hasChildren := len(g.Children) > 0

	// Print root version
	if hasChildren {
		fmt.Fprintf(w, "┌─ %d  %s\n", g.RootVersion.ID, g.Type)
	} else {
		fmt.Fprintf(w, "─  %d  %s\n", g.RootVersion.ID, g.Type)
	}
	printVerboseDetails(w, g.RootVersion.Name, g.RootVersion.Tags, g.RootVersion.CreatedAt, 0, hasChildren)

	// Print children
	for i, child := range g.Children {
		isLastChild := i == len(g.Children)-1
		indicator := "├─"
		if isLastChild {
			indicator = "└─"
		}

		// Determine type label
		typeLabel := formatVerboseType(child)

		fmt.Fprintf(w, "%s %d  %s\n", indicator, child.Version.ID, typeLabel)
		printVerboseChildDetails(w, child, isLastChild)
	}

	if !isLast {
		fmt.Fprintln(w)
	}
}

func formatVerboseType(child VersionChild) string {
	if child.Platform != "" {
		return "platform: " + child.Platform
	}
	return "attestation: " + child.ArtifactType
}

func printVerboseDetails(w io.Writer, digest string, tags []string, created string, size int64, hasChildren bool) {
	prefix := "│  "
	if !hasChildren {
		prefix = "   "
	}

	fmt.Fprintf(w, "%sDigest:  %s\n", prefix, digest)
	if len(tags) > 0 {
		fmt.Fprintf(w, "%sTags:    %s\n", prefix, display.FormatTags(tags))
	}
	if size > 0 {
		fmt.Fprintf(w, "%sSize:    %s\n", prefix, formatSize(size))
	}
	fmt.Fprintf(w, "%sCreated: %s\n", prefix, created)
	fmt.Fprintf(w, "%s\n", prefix)
}

func printVerboseChildDetails(w io.Writer, child VersionChild, isLast bool) {
	prefix := "│  "
	if isLast {
		prefix = "   "
	}

	fmt.Fprintf(w, "%sDigest:  %s\n", prefix, child.Version.Name)
	if child.Size > 0 {
		fmt.Fprintf(w, "%sSize:    %s\n", prefix, formatSize(child.Size))
	}
	fmt.Fprintf(w, "%sCreated: %s\n", prefix, child.Version.CreatedAt)
	fmt.Fprintf(w, "%s\n", prefix)
}

func formatSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
	)
	switch {
	case bytes >= MB:
		return fmt.Sprintf("%.1f MB", float64(bytes)/MB)
	case bytes >= KB:
		return fmt.Sprintf("%.1f KB", float64(bytes)/KB)
	default:
		return fmt.Sprintf("%d bytes", bytes)
	}
}

func determineVersionType(ver gh.PackageVersionInfo, graphType string) string {
	// Check graphType first - it tells us what the version actually is
	// regardless of whether it has tags
	if graphType == "index" {
		return "index"
	}

	// For all other cases (manifest, standalone), return "manifest"
	// This is more accurate than "untagged" which describes tag status, not type
	return "manifest"
}

// parseUserDate parses a date string in multiple user-friendly formats
// Supports: "2025-11-01", "2025-11-01T10:30:00Z", etc.
func parseUserDate(dateStr string) (time.Time, error) {
	// Try common formats in order of likelihood
	formats := []string{
		"2006-01-02",          // Date only (most convenient)
		time.RFC3339,          // 2006-01-02T15:04:05Z07:00
		time.RFC3339Nano,      // With fractional seconds
		"2006-01-02 15:04:05", // Space-separated (similar to GitHub format)
		"2006-01-02T15:04:05", // Without timezone
		time.DateTime,         // 2006-01-02 15:04:05 (Go 1.20+)
		time.DateOnly,         // 2006-01-02 (Go 1.20+)
	}

	for _, format := range formats {
		t, err := time.Parse(format, dateStr)
		if err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse date %q (supported formats: YYYY-MM-DD, RFC3339)", dateStr)
}

// buildVersionFilter creates a VersionFilter from command-line flags
func buildVersionFilter() (*filter.VersionFilter, error) {
	vf := &filter.VersionFilter{
		OnlyTagged:    versionsOnlyTagged,
		OnlyUntagged:  versionsOnlyUntagged,
		TagPattern:    versionsTagPattern,
		OlderThanDays: versionsOlderThanDays,
		NewerThanDays: versionsNewerThanDays,
		VersionID:     versionsVersionID,
		Digest:        versionsDigest,
	}

	// Handle exact tag match (backward compatibility with --tag flag)
	if versionsTag != "" {
		vf.Tags = []string{versionsTag}
	}

	// Parse absolute date filters
	if versionsOlderThan != "" {
		t, err := parseUserDate(versionsOlderThan)
		if err != nil {
			return nil, fmt.Errorf("invalid --older-than date format: %w", err)
		}
		vf.OlderThan = t
	}

	if versionsNewerThan != "" {
		t, err := parseUserDate(versionsNewerThan)
		if err != nil {
			return nil, fmt.Errorf("invalid --newer-than date format: %w", err)
		}
		vf.NewerThan = t
	}

	// Validate conflicting flags
	if versionsOnlyTagged && versionsOnlyUntagged {
		return nil, fmt.Errorf("cannot use --tagged and --untagged together")
	}

	return vf, nil
}

func init() {
	rootCmd.AddCommand(versionsCmd)
	versionsCmd.Flags().BoolVarP(&versionsVerbose, "verbose", "v", false, "Show detailed version information")
	versionsCmd.Flags().BoolVar(&versionsJSON, "json", false, "Output in JSON format")
	versionsCmd.Flags().StringVar(&versionsTag, "tag", "", "Filter versions by exact tag match")
	versionsCmd.Flags().StringVar(&versionsTagPattern, "tag-pattern", "", "Filter versions by tag regex pattern")
	versionsCmd.Flags().BoolVar(&versionsOnlyTagged, "tagged", false, "Show only tagged versions")
	versionsCmd.Flags().BoolVar(&versionsOnlyUntagged, "untagged", false, "Show only untagged versions")
	versionsCmd.Flags().StringVar(&versionsOlderThan, "older-than", "", "Show versions older than date (YYYY-MM-DD or RFC3339)")
	versionsCmd.Flags().StringVar(&versionsNewerThan, "newer-than", "", "Show versions newer than date (YYYY-MM-DD or RFC3339)")
	versionsCmd.Flags().IntVar(&versionsOlderThanDays, "older-than-days", 0, "Show versions older than N days")
	versionsCmd.Flags().IntVar(&versionsNewerThanDays, "newer-than-days", 0, "Show versions newer than N days")
	versionsCmd.Flags().StringVarP(&versionsOutputFormat, "output", "o", "", "Output format (json, table)")
	versionsCmd.Flags().Int64Var(&versionsVersionID, "version", 0, "Filter by exact version ID")
	versionsCmd.Flags().StringVar(&versionsDigest, "digest", "", "Filter by digest (supports prefix matching)")
}
