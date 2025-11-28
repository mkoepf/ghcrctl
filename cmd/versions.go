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

// VersionGraph is an alias for discovery.VersionGraph for backward compatibility
type VersionGraph = discovery.VersionGraph

// VersionChild is an alias for discovery.VersionChild for backward compatibility
type VersionChild = discovery.VersionChild

func buildVersionGraphs(ctx context.Context, fullImage string, versionsToGraph []gh.PackageVersionInfo, allVersions []gh.PackageVersionInfo, client *gh.Client, owner, ownerType, imageName string) ([]VersionGraph, error) {
	// Create version cache for efficient lookups (use ALL versions for child discovery)
	cache := discovery.NewVersionCacheFromSlice(allVersions)

	// Track which versions are graph roots (to avoid treating them as standalone later)
	isGraphRoot := make(map[int64]bool)

	// Track reference counts for each version ID (how many graphs reference it)
	refCounts := make(map[int64]int)

	// Identify cosign children: versions whose tags indicate they are .sig or .att artifacts
	// Map from parent digest -> list of cosign child versions
	cosignChildrenByParent := make(map[string][]gh.PackageVersionInfo)
	isCosignChild := make(map[int64]bool)
	for _, ver := range allVersions {
		for _, tag := range ver.Tags {
			if parentDigest, ok := oras.ExtractParentDigestFromCosignTag(tag); ok {
				cosignChildrenByParent[parentDigest] = append(cosignChildrenByParent[parentDigest], ver)
				isCosignChild[ver.ID] = true
				break // Only need to check one cosign tag per version
			}
		}
	}

	// Identify orphan attestations: untagged versions that might be attestations
	// Try to detect their parent by extracting the subject digest from their content
	for _, ver := range allVersions {
		// Skip if already identified as a child or if it has tags
		if isCosignChild[ver.ID] || len(ver.Tags) > 0 {
			continue
		}

		// Check if this might be an attestation by trying to extract its subject
		parentDigest, err := oras.ExtractSubjectDigest(ctx, fullImage, ver.Name)
		if err == nil && parentDigest != "" {
			// Found parent - add as a child
			cosignChildrenByParent[parentDigest] = append(cosignChildrenByParent[parentDigest], ver)
			isCosignChild[ver.ID] = true
		}
	}

	var graphs []VersionGraph

	// Helper to build a graph for a version
	buildGraph := func(ver gh.PackageVersionInfo) *VersionGraph {
		relatedArtifacts := discoverRelatedVersionsByDigest(ctx, fullImage, ver.Name, ver.Name)

		// Skip if no children and not tagged (standalone untagged versions are handled separately)
		if len(relatedArtifacts) == 0 && len(ver.Tags) == 0 {
			return nil
		}

		// Use ResolveType for consistent type determination
		graphType := "manifest" // default
		if artType, err := oras.ResolveType(ctx, fullImage, ver.Name); err == nil {
			graphType = artType.DisplayType()
		}

		graph := &VersionGraph{
			RootVersion: ver,
			Children:    []VersionChild{},
			Type:        graphType,
		}

		// Find child versions by digest and consolidate artifact types
		childMap := make(map[int64]*VersionChild)
		for _, artifact := range relatedArtifacts {
			if childVer, found := cache.ByDigest[artifact.Digest]; found {
				if _, exists := childMap[childVer.ID]; exists {
					// Skip duplicates - the first type wins
				} else {
					// Only count reference once per unique child per graph
					refCounts[childVer.ID]++
					childMap[childVer.ID] = &VersionChild{
						Version: childVer,
						Type:    artifact.Type,
						Size:    artifact.Size,
					}
				}
			}
		}

		// Add children to graph
		for _, child := range childMap {
			graph.Children = append(graph.Children, *child)
		}

		// Add cosign children (versions with .sig or .att tags referencing this digest)
		if cosignChildren, found := cosignChildrenByParent[ver.Name]; found {
			for _, cosignVer := range cosignChildren {
				if _, exists := childMap[cosignVer.ID]; exists {
					continue // Already added via ORAS discovery
				}
				// Resolve type for the cosign artifact
				childType := oras.ArtifactType{Role: "attestation"} // default
				if artType, err := oras.ResolveType(ctx, fullImage, cosignVer.Name); err == nil {
					childType = artType
				}
				refCounts[cosignVer.ID]++
				graph.Children = append(graph.Children, VersionChild{
					Version: cosignVer,
					Type:    childType,
					Size:    0, // Size not easily available from version info
				})
			}
		}

		return graph
	}

	// Process tagged versions first (potential graph roots)
	// Skip versions that are cosign children - they'll be added to their parent's graph
	for _, ver := range versionsToGraph {
		if len(ver.Tags) > 0 && !isGraphRoot[ver.ID] && !isCosignChild[ver.ID] {
			isGraphRoot[ver.ID] = true
			if graph := buildGraph(ver); graph != nil {
				graphs = append(graphs, *graph)
			}
		}
	}

	// Process untagged versions as potential graph roots
	for _, ver := range versionsToGraph {
		if len(ver.Tags) == 0 && !isGraphRoot[ver.ID] && !isCosignChild[ver.ID] {
			if graph := buildGraph(ver); graph != nil {
				isGraphRoot[ver.ID] = true
				graphs = append(graphs, *graph)
			}
		}
	}

	// Add remaining unassigned versions as standalone (not a root and not a child of any graph)
	// This includes orphan attestations (untagged versions with attestation types)
	for _, ver := range versionsToGraph {
		if !isGraphRoot[ver.ID] && refCounts[ver.ID] == 0 && !isCosignChild[ver.ID] {
			// Use ResolveType for consistent type determination
			graphType := "manifest" // default
			if artType, err := oras.ResolveType(ctx, fullImage, ver.Name); err == nil {
				graphType = artType.DisplayType()
			}

			graph := VersionGraph{
				RootVersion: ver,
				Children:    []VersionChild{},
				Type:        graphType,
			}
			graphs = append(graphs, graph)
		}
	}

	// Second pass: update RefCount on all children based on total references
	for i := range graphs {
		for j := range graphs[i].Children {
			graphs[i].Children[j].RefCount = refCounts[graphs[i].Children[j].Version.ID]
		}
	}

	return graphs, nil
}

// discoverRelatedVersionsByDigest discovers children using a digest directly
func discoverRelatedVersionsByDigest(ctx context.Context, fullImage, digest, rootDigest string) []oras.ChildArtifact {
	// Use unified discovery
	children, err := oras.DiscoverChildren(ctx, fullImage, digest, nil)
	if err != nil {
		return nil
	}

	// Filter out the root digest
	var filtered []oras.ChildArtifact
	for _, child := range children {
		if child.Digest != rootDigest {
			filtered = append(filtered, child)
		}
	}

	return filtered
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
			relatedArtifacts := discoverRelatedVersionsByDigest(ctx, fullImage, ver.Name, ver.Name)

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

	// Calculate distinct versions and shared count
	seenVersions := make(map[int64]bool)
	sharedCount := 0
	for _, g := range graphs {
		seenVersions[g.RootVersion.ID] = true
		for _, child := range g.Children {
			if !seenVersions[child.Version.ID] {
				seenVersions[child.Version.ID] = true
				if child.RefCount > 1 {
					sharedCount++
				}
			}
		}
	}
	totalVersions := len(seenVersions)

	fmt.Fprintf(w, "Versions for %s:\n\n", imageName)

	// Find column widths and max RefCount for shared indicator
	maxIDLen := len("VERSION ID")
	maxTypeLen := len("TYPE")
	maxDigestLen := len("DIGEST")
	maxTagsLen := len("TAGS")
	maxCreatedLen := len("CREATED")
	maxRefCount := 0

	for _, g := range graphs {
		// Check root version
		ver := g.RootVersion
		if idLen := len(fmt.Sprintf("%d", ver.ID)); idLen > maxIDLen {
			maxIDLen = idLen
		}
		if typeLen := len(g.Type); typeLen > maxTypeLen {
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
			// Determine child type display using unified type
			childType := child.Type.DisplayType()
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
			// Track max RefCount for shared indicator width
			if child.RefCount > maxRefCount {
				maxRefCount = child.RefCount
			}
		}
	}

	// Calculate shared indicator width: " (N*)" where N is maxRefCount
	sharedIndicatorWidth := 0
	if maxRefCount > 1 {
		sharedIndicatorWidth = len(fmt.Sprintf(" (%d*)", maxRefCount))
	}

	// Total ID column width includes space for shared indicator
	idColWidth := maxIDLen + sharedIndicatorWidth

	// Print header with colors
	fmt.Fprintf(w, "  %s  %s  %s  %s  %s\n",
		display.ColorHeader(fmt.Sprintf("%-*s", idColWidth, "VERSION ID")),
		display.ColorHeader(fmt.Sprintf("%-*s", maxTypeLen, "TYPE")),
		display.ColorHeader(fmt.Sprintf("%-*s", maxDigestLen, "DIGEST")),
		display.ColorHeader(fmt.Sprintf("%-*s", maxTagsLen, "TAGS")),
		display.ColorHeader("CREATED"))
	fmt.Fprintf(w, "  %s  %s  %s  %s  %s\n",
		display.ColorSeparator(strings.Repeat("-", idColWidth)),
		display.ColorSeparator(strings.Repeat("-", maxTypeLen)),
		display.ColorSeparator(strings.Repeat("-", maxDigestLen)),
		display.ColorSeparator(strings.Repeat("-", maxTagsLen)),
		display.ColorSeparator(strings.Repeat("-", maxCreatedLen)))

	// Print graphs
	for i, g := range graphs {
		printVersionGraph(w, g, maxIDLen, sharedIndicatorWidth, maxTypeLen, maxDigestLen, maxTagsLen, maxCreatedLen)

		// Add blank line between graphs (but not after the last one)
		if i < len(graphs)-1 && len(g.Children) > 0 {
			fmt.Fprintln(w)
		}
	}

	// Build summary with proper plural/singular
	versionWord := "versions"
	if totalVersions == 1 {
		versionWord = "version"
	}
	graphWord := "graphs"
	if len(graphs) == 1 {
		graphWord = "graph"
	}

	if sharedCount > 0 {
		sharedWord := "versions appear"
		if sharedCount == 1 {
			sharedWord = "version appears"
		}
		fmt.Fprintf(w, "\nTotal: %s %s in %s %s. %s %s in multiple graphs.\n",
			display.ColorCount(totalVersions), versionWord,
			display.ColorCount(len(graphs)), graphWord,
			display.ColorShared(fmt.Sprintf("%d", sharedCount)), sharedWord)
	} else {
		fmt.Fprintf(w, "\nTotal: %s %s in %s %s.\n",
			display.ColorCount(totalVersions), versionWord,
			display.ColorCount(len(graphs)), graphWord)
	}
	return nil
}

func printVersionGraph(w io.Writer, g VersionGraph, maxIDLen, sharedIndicatorWidth, maxTypeLen, maxDigestLen, maxTagsLen, maxCreatedLen int) {
	// Determine tree indicators (apply color)
	var rootIndicator, midIndicator, lastIndicator string
	if len(g.Children) > 0 {
		rootIndicator = display.ColorTreeIndicator("┌")
		midIndicator = display.ColorTreeIndicator("├")
		lastIndicator = display.ColorTreeIndicator("└")
	} else {
		rootIndicator = " "
	}

	// Print root version (refCount=0 means not a child, so no sharing indicator)
	printVersion(w, rootIndicator, g.RootVersion, g.Type,
		maxIDLen, sharedIndicatorWidth, maxTypeLen, maxDigestLen, maxTagsLen, maxCreatedLen, 0)

	// Print children
	for i, child := range g.Children {
		indicator := midIndicator
		if i == len(g.Children)-1 {
			indicator = lastIndicator
		}

		// Determine child type display using unified type
		childType := child.Type.DisplayType()

		printVersion(w, indicator, child.Version, childType, maxIDLen, sharedIndicatorWidth, maxTypeLen, maxDigestLen, maxTagsLen, maxCreatedLen, child.RefCount)
	}
}

func printVersion(w io.Writer, indicator string, ver gh.PackageVersionInfo, vtype string, maxIDLen, sharedIndicatorWidth, maxTypeLen, maxDigestLen, maxTagsLen, maxCreatedLen int, refCount int) {
	// Format raw values for width calculation
	tagsStr := display.FormatTags(ver.Tags)
	digest := display.ShortDigest(ver.Name)

	// Apply colors (pad first, then color to preserve alignment)
	coloredType := display.ColorVersionType(fmt.Sprintf("%-*s", maxTypeLen, vtype))
	coloredDigest := display.ColorDigest(fmt.Sprintf("%-*s", maxDigestLen, digest))
	// Pad tags after coloring won't work for alignment, so we pad the raw string
	// and accept that colored output may shift slightly (acceptable tradeoff)
	var paddedTags string
	if len(ver.Tags) > 0 {
		paddedTags = display.ColorTags(ver.Tags) + strings.Repeat(" ", maxTagsLen-len(tagsStr))
	} else {
		paddedTags = display.ColorTags(ver.Tags) + strings.Repeat(" ", maxTagsLen-2) // "[]" is 2 chars
	}

	// Format VERSION ID with optional sharing indicator (directly after ID)
	// Pad the combined ID+indicator to maintain column alignment
	idStr := fmt.Sprintf("%d", ver.ID)
	sharedIndicator := ""
	sharedIndicatorLen := 0
	if refCount > 1 {
		sharedIndicator = fmt.Sprintf(" (%d*)", refCount)
		sharedIndicatorLen = len(sharedIndicator)
	}
	// Calculate padding: total column width - ID length - indicator length
	idColWidth := maxIDLen + sharedIndicatorWidth
	padding := idColWidth - len(idStr) - sharedIndicatorLen
	if padding < 0 {
		padding = 0
	}

	// Color the shared indicator
	coloredSharedIndicator := ""
	if sharedIndicator != "" {
		coloredSharedIndicator = display.ColorShared(sharedIndicator)
	}

	fmt.Fprintf(w, "%s %s%s%s  %s  %s  %s  %s\n",
		indicator,
		idStr,
		coloredSharedIndicator,
		strings.Repeat(" ", padding),
		coloredType,
		coloredDigest,
		paddedTags,
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

	// Print root version with colored tree indicators and type
	versionType := g.Type
	if hasChildren {
		fmt.Fprintf(w, "%s %d  %s\n",
			display.ColorTreeIndicator("┌─"),
			g.RootVersion.ID,
			display.ColorVersionType(versionType))
	} else {
		fmt.Fprintf(w, "%s %d  %s\n",
			display.ColorTreeIndicator("─"),
			g.RootVersion.ID,
			display.ColorVersionType(versionType))
	}
	printVerboseDetails(w, g.RootVersion.Name, g.RootVersion.Tags, g.RootVersion.CreatedAt, 0, hasChildren)

	// Print children
	for i, child := range g.Children {
		isLastChild := i == len(g.Children)-1
		indicator := display.ColorTreeIndicator("├─")
		if isLastChild {
			indicator = display.ColorTreeIndicator("└─")
		}

		// Determine type label
		typeLabel := formatVerboseType(child)

		fmt.Fprintf(w, "%s %d  %s\n", indicator, child.Version.ID, display.ColorVersionType(typeLabel))
		printVerboseChildDetails(w, child, isLastChild)
	}

	if !isLast {
		fmt.Fprintln(w)
	}
}

func formatVerboseType(child VersionChild) string {
	if child.Type.IsPlatform() {
		return "platform: " + child.Type.Platform
	}
	return "attestation: " + child.Type.Role
}

func printVerboseDetails(w io.Writer, digest string, tags []string, created string, size int64, hasChildren bool) {
	prefix := display.ColorTreeIndicator("│  ")
	if !hasChildren {
		prefix = "   "
	}

	fmt.Fprintf(w, "%sDigest:  %s\n", prefix, display.ColorDigest(digest))
	if len(tags) > 0 {
		fmt.Fprintf(w, "%sTags:    %s\n", prefix, display.ColorTags(tags))
	}
	if size > 0 {
		fmt.Fprintf(w, "%sSize:    %s\n", prefix, formatSize(size))
	}
	fmt.Fprintf(w, "%sCreated: %s\n", prefix, created)
	fmt.Fprintf(w, "%s\n", prefix)
}

func printVerboseChildDetails(w io.Writer, child VersionChild, isLast bool) {
	prefix := display.ColorTreeIndicator("│  ")
	if isLast {
		prefix = "   "
	}

	fmt.Fprintf(w, "%sDigest:  %s\n", prefix, display.ColorDigest(child.Version.Name))
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
