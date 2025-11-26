package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mhk/ghcrctl/internal/config"
	"github.com/mhk/ghcrctl/internal/filter"
	"github.com/mhk/ghcrctl/internal/gh"
	"github.com/mhk/ghcrctl/internal/graph"
	"github.com/mhk/ghcrctl/internal/oras"
	"github.com/mhk/ghcrctl/internal/prompts"
	"github.com/spf13/cobra"
)

var (
	deleteForce      bool
	deleteDryRun     bool
	deleteVersionDig string
	deleteGraphDig   string
	deleteGraphVer   int64

	// Filter flags for bulk deletion
	deleteTagPattern    string
	deleteOnlyTagged    bool
	deleteOnlyUntagged  bool
	deleteOlderThan     string
	deleteNewerThan     string
	deleteOlderThanDays int
	deleteNewerThanDays int
)

var deleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete package versions from GitHub Container Registry",
	Long: `Delete package versions from GitHub Container Registry.

Use subcommands to delete individual versions or complete OCI graphs.

Available Commands:
  version     Delete a single package version
  graph       Delete an entire OCI artifact graph (image + platforms + attestations)`,
}

var deleteVersionCmd = &cobra.Command{
	Use:   "version <image> [version-id]",
	Short: "Delete package version(s)",
	Long: `Delete package version(s) from GitHub Container Registry.

This command can delete a single version by ID/digest, or multiple versions using filters.
By default, it will prompt for confirmation before deleting.

IMPORTANT: Deletion is permanent and cannot be undone (except within 30 days
via the GitHub web UI if the package namespace is available).

Examples:
  # Delete by version ID
  ghcrctl delete version myimage 12345678

  # Delete by digest
  ghcrctl delete version myimage --digest sha256:abc123...

  # Delete all untagged versions (bulk deletion)
  ghcrctl delete version myimage --untagged

  # Delete untagged versions older than 30 days
  ghcrctl delete version myimage --untagged --older-than-days 30

  # Delete versions matching tag pattern older than a date
  ghcrctl delete version myimage --tag-pattern ".*-rc.*" --older-than 2025-01-01

  # Preview what would be deleted (dry-run)
  ghcrctl delete version myimage --untagged --dry-run

  # Skip confirmation for bulk deletion
  ghcrctl delete version myimage --untagged --older-than-days 30 --force`,
	Args: func(cmd *cobra.Command, args []string) error {
		// Check if any filter flags are set (indicates bulk deletion)
		filterFlagsSet := deleteOnlyTagged || deleteOnlyUntagged || deleteTagPattern != "" ||
			deleteOlderThan != "" || deleteNewerThan != "" ||
			deleteOlderThanDays > 0 || deleteNewerThanDays > 0

		// If --digest is provided, we need exactly 1 arg (image name)
		if deleteVersionDig != "" {
			if len(args) != 1 {
				return fmt.Errorf("accepts 1 arg(s) when using --digest, received %d", len(args))
			}
			return nil
		}

		// If filter flags are set (bulk deletion), we need exactly 1 arg (image name)
		if filterFlagsSet {
			if len(args) != 1 {
				return fmt.Errorf("accepts 1 arg (image name) when using filters, received %d", len(args))
			}
			return nil
		}

		// Otherwise, we need exactly 2 args (image name and version-id)
		return cobra.ExactArgs(2)(cmd, args)
	},
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

		// Check if this is bulk deletion mode
		filterFlagsSet := deleteOnlyTagged || deleteOnlyUntagged || deleteTagPattern != "" ||
			deleteOlderThan != "" || deleteNewerThan != "" ||
			deleteOlderThanDays > 0 || deleteNewerThanDays > 0

		if filterFlagsSet {
			// Bulk deletion mode
			return runBulkDelete(ctx, cmd, client, owner, ownerType, imageName)
		}

		// Single deletion mode
		return runSingleDelete(ctx, cmd, args, client, owner, ownerType, imageName)
	},
}

var deleteGraphCmd = &cobra.Command{
	Use:   "graph <image> <tag>",
	Short: "Delete an entire OCI artifact graph",
	Long: `Delete an entire OCI artifact graph from GitHub Container Registry.

This command discovers and deletes all versions that make up an OCI artifact graph,
including the root image/index, platform manifests, and attestations (SBOM, provenance).

The graph can be identified by:
  - Tag (positional argument) - most common
  - Digest (--digest flag)
  - Version ID (--version flag) - finds the graph containing this version

IMPORTANT: Deletion is permanent and cannot be undone (except within 30 days
via the GitHub web UI if the package namespace is available).

Examples:
  # Delete graph by tag (most common)
  ghcrctl delete graph myimage v1.0.0

  # Delete graph by digest
  ghcrctl delete graph myimage --digest sha256:abc123...

  # Delete graph containing a specific version
  ghcrctl delete graph myimage --version 12345678

  # Skip confirmation
  ghcrctl delete graph myimage v1.0.0 --force

  # Preview what would be deleted
  ghcrctl delete graph myimage v1.0.0 --dry-run`,
	Args: func(cmd *cobra.Command, args []string) error {
		// Count how many lookup methods are provided
		flagsSet := 0
		if cmd.Flags().Changed("digest") {
			flagsSet++
		}
		if cmd.Flags().Changed("version") {
			flagsSet++
		}

		// If flags are provided, we need exactly 1 arg (image name)
		if flagsSet > 0 {
			if flagsSet > 1 {
				return fmt.Errorf("flags --digest and --version are mutually exclusive")
			}
			if len(args) != 1 {
				return fmt.Errorf("accepts 1 arg when using --digest or --version, received %d", len(args))
			}
			return nil
		}

		// Otherwise, we need exactly 2 args (image name and tag)
		return cobra.ExactArgs(2)(cmd, args)
	},
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

		// Create clients
		ghClient, err := gh.NewClientWithContext(cmd.Context(), token)
		if err != nil {
			cmd.SilenceUsage = true
			return fmt.Errorf("failed to create GitHub client: %w", err)
		}

		ctx := cmd.Context()
		fullImage := fmt.Sprintf("ghcr.io/%s/%s", owner, imageName)

		// Determine the root digest based on which flag/argument was used
		var rootDigest string
		var tag string

		if deleteGraphDig != "" {
			// Lookup by digest
			rootDigest = deleteGraphDig
			if !strings.HasPrefix(rootDigest, "sha256:") {
				rootDigest = "sha256:" + rootDigest
			}
		} else if deleteGraphVer != 0 {
			// Lookup by version ID - need to find the digest first
			allVersions, err := ghClient.ListPackageVersions(ctx, owner, ownerType, imageName)
			if err != nil {
				cmd.SilenceUsage = true
				return fmt.Errorf("failed to list package versions: %w", err)
			}

			var found bool
			for _, ver := range allVersions {
				if ver.ID == deleteGraphVer {
					rootDigest = ver.Name
					found = true
					break
				}
			}

			if !found {
				cmd.SilenceUsage = true
				return fmt.Errorf("version ID %d not found", deleteGraphVer)
			}
		} else {
			// Lookup by tag (positional argument)
			tag = args[1]
			rootDigest, err = oras.ResolveTag(ctx, fullImage, tag)
			if err != nil {
				cmd.SilenceUsage = true
				return fmt.Errorf("failed to resolve tag '%s': %w", tag, err)
			}
		}

		// Build the graph (reuse logic from graph command)
		g, err := buildGraph(ctx, ghClient, fullImage, owner, ownerType, imageName, rootDigest, tag)
		if err != nil {
			cmd.SilenceUsage = true
			return fmt.Errorf("failed to build graph: %w", err)
		}

		// Collect all version IDs to delete
		versionIDs := collectVersionIDs(g)

		// Display what will be deleted
		fmt.Fprintf(cmd.OutOrStdout(), "Preparing to delete complete OCI graph:\n")
		fmt.Fprintf(cmd.OutOrStdout(), "  Image: %s\n", imageName)
		if tag != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "  Tag:   %s\n", tag)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "\n")
		displayGraphSummary(cmd.OutOrStdout(), g)
		fmt.Fprintf(cmd.OutOrStdout(), "\nTotal: %d version(s) will be deleted\n\n", len(versionIDs))

		// Handle dry-run
		if deleteDryRun {
			fmt.Fprintln(cmd.OutOrStdout(), "DRY RUN: No changes made")
			return nil
		}

		// Confirm deletion unless --force is used
		if !deleteForce {
			confirmed, err := prompts.Confirm(os.Stdin, cmd.OutOrStdout(), "Are you sure you want to delete this graph?")
			if err != nil {
				return fmt.Errorf("failed to read confirmation: %w", err)
			}

			if !confirmed {
				fmt.Fprintln(cmd.OutOrStdout(), "Deletion cancelled")
				return nil
			}
		}

		// Perform deletions (children first, then root)
		err = deleteGraph(ctx, ghClient, owner, ownerType, imageName, versionIDs, cmd.OutOrStdout())
		if err != nil {
			cmd.SilenceUsage = true
			return err
		}

		fmt.Fprintf(cmd.OutOrStdout(), "\nSuccessfully deleted %d version(s) of %s\n", len(versionIDs), imageName)
		return nil
	},
}

// runSingleDelete handles deletion of a single version
func runSingleDelete(ctx context.Context, cmd *cobra.Command, args []string, client *gh.Client, owner, ownerType, imageName string) error {
	var versionID int64
	var err error

	// Determine version ID from either positional arg or --digest flag
	if deleteVersionDig != "" {
		// Normalize digest format
		digest := deleteVersionDig
		if !strings.HasPrefix(digest, "sha256:") {
			digest = "sha256:" + digest
		}

		// Look up version ID by digest
		versionID, err = client.GetVersionIDByDigest(ctx, owner, ownerType, imageName, digest)
		if err != nil {
			cmd.SilenceUsage = true
			return fmt.Errorf("failed to find version by digest: %w", err)
		}
	} else {
		// Parse version ID from positional argument
		versionIDStr := args[1]
		versionID, err = strconv.ParseInt(versionIDStr, 10, 64)
		if err != nil {
			cmd.SilenceUsage = true
			return fmt.Errorf("invalid version-id: must be a number, got %q", versionIDStr)
		}
	}

	// Get version tags to show what we're deleting
	tags, err := client.GetVersionTags(ctx, owner, ownerType, imageName, versionID)
	if err != nil {
		cmd.SilenceUsage = true
		return fmt.Errorf("failed to get version details: %w", err)
	}

	// Show what will be deleted
	fmt.Fprintf(cmd.OutOrStdout(), "Preparing to delete package version:\n")
	fmt.Fprintf(cmd.OutOrStdout(), "  Image:      %s\n", imageName)
	fmt.Fprintf(cmd.OutOrStdout(), "  Owner:      %s (%s)\n", owner, ownerType)
	fmt.Fprintf(cmd.OutOrStdout(), "  Version ID: %d\n", versionID)
	if len(tags) > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "  Tags:       %v\n", tags)
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "  Tags:       (untagged)\n")
	}
	fmt.Fprintln(cmd.OutOrStdout())

	// Handle dry-run
	if deleteDryRun {
		fmt.Fprintln(cmd.OutOrStdout(), "DRY RUN: No changes made")
		return nil
	}

	// Confirm deletion unless --force is used
	if !deleteForce {
		confirmed, err := prompts.Confirm(os.Stdin, cmd.OutOrStdout(), "Are you sure you want to delete this version?")
		if err != nil {
			return fmt.Errorf("failed to read confirmation: %w", err)
		}

		if !confirmed {
			fmt.Fprintln(cmd.OutOrStdout(), "Deletion cancelled")
			return nil
		}
	}

	// Perform deletion
	err = client.DeletePackageVersion(ctx, owner, ownerType, imageName, versionID)
	if err != nil {
		cmd.SilenceUsage = true
		return fmt.Errorf("failed to delete package version: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Successfully deleted version %d of %s\n", versionID, imageName)
	return nil
}

// runBulkDelete handles deletion of multiple versions using filters
func runBulkDelete(ctx context.Context, cmd *cobra.Command, client *gh.Client, owner, ownerType, imageName string) error {
	// Build filter from flags
	filter, err := buildDeleteFilter()
	if err != nil {
		cmd.SilenceUsage = true
		return fmt.Errorf("invalid filter options: %w", err)
	}

	// List all package versions
	allVersions, err := client.ListPackageVersions(ctx, owner, ownerType, imageName)
	if err != nil {
		cmd.SilenceUsage = true
		return fmt.Errorf("failed to list package versions: %w", err)
	}

	// Apply filters
	matchingVersions := filter.Apply(allVersions)

	// Check if any versions match
	if len(matchingVersions) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No versions match the specified filters")
		return nil
	}

	// Display summary of what will be deleted
	fmt.Fprintf(cmd.OutOrStdout(), "Preparing to delete %d package version(s):\n", len(matchingVersions))
	fmt.Fprintf(cmd.OutOrStdout(), "  Image: %s\n", imageName)
	fmt.Fprintf(cmd.OutOrStdout(), "  Owner: %s (%s)\n\n", owner, ownerType)

	// Show details of matching versions (limit to first 10 for readability)
	displayLimit := 10
	for i, ver := range matchingVersions {
		if i >= displayLimit {
			fmt.Fprintf(cmd.OutOrStdout(), "  ... and %d more\n", len(matchingVersions)-displayLimit)
			break
		}

		tagsStr := "(untagged)"
		if len(ver.Tags) > 0 {
			tagsStr = strings.Join(ver.Tags, ", ")
		}
		fmt.Fprintf(cmd.OutOrStdout(), "  - ID: %d, Tags: %s, Created: %s\n", ver.ID, tagsStr, ver.CreatedAt)
	}
	fmt.Fprintln(cmd.OutOrStdout())

	// Handle dry-run
	if deleteDryRun {
		fmt.Fprintln(cmd.OutOrStdout(), "DRY RUN: No changes made")
		return nil
	}

	// Confirm deletion unless --force is used
	if !deleteForce {
		confirmed, err := prompts.Confirm(os.Stdin, cmd.OutOrStdout(), fmt.Sprintf("Are you sure you want to delete %d version(s)?", len(matchingVersions)))
		if err != nil {
			return fmt.Errorf("failed to read confirmation: %w", err)
		}

		if !confirmed {
			fmt.Fprintln(cmd.OutOrStdout(), "Deletion cancelled")
			return nil
		}
	}

	// Perform bulk deletion
	successCount := 0
	failCount := 0
	for i, ver := range matchingVersions {
		fmt.Fprintf(cmd.OutOrStdout(), "Deleting version %d/%d (ID: %d)...\n", i+1, len(matchingVersions), ver.ID)
		err := client.DeletePackageVersion(ctx, owner, ownerType, imageName, ver.ID)
		if err != nil {
			fmt.Fprintf(cmd.OutOrStdout(), "  Failed: %v\n", err)
			failCount++
		} else {
			successCount++
		}
	}

	// Summary
	fmt.Fprintln(cmd.OutOrStdout())
	fmt.Fprintf(cmd.OutOrStdout(), "Deletion complete: %d succeeded, %d failed\n", successCount, failCount)

	if failCount > 0 {
		return fmt.Errorf("failed to delete %d version(s)", failCount)
	}

	return nil
}

func init() {
	rootCmd.AddCommand(deleteCmd)

	// Add subcommands
	deleteCmd.AddCommand(deleteVersionCmd)
	deleteCmd.AddCommand(deleteGraphCmd)

	// Flags for delete version
	deleteVersionCmd.Flags().BoolVar(&deleteForce, "force", false, "Skip confirmation prompt")
	deleteVersionCmd.Flags().BoolVar(&deleteDryRun, "dry-run", false, "Show what would be deleted without deleting")
	deleteVersionCmd.Flags().StringVar(&deleteVersionDig, "digest", "", "Delete version by digest")

	// Filter flags for bulk deletion
	deleteVersionCmd.Flags().StringVar(&deleteTagPattern, "tag-pattern", "", "Delete versions matching regex pattern")
	deleteVersionCmd.Flags().BoolVar(&deleteOnlyTagged, "tagged", false, "Delete only tagged versions")
	deleteVersionCmd.Flags().BoolVar(&deleteOnlyUntagged, "untagged", false, "Delete only untagged versions")
	deleteVersionCmd.Flags().StringVar(&deleteOlderThan, "older-than", "", "Delete versions older than date (YYYY-MM-DD or RFC3339)")
	deleteVersionCmd.Flags().StringVar(&deleteNewerThan, "newer-than", "", "Delete versions newer than date (YYYY-MM-DD or RFC3339)")
	deleteVersionCmd.Flags().IntVar(&deleteOlderThanDays, "older-than-days", 0, "Delete versions older than N days")
	deleteVersionCmd.Flags().IntVar(&deleteNewerThanDays, "newer-than-days", 0, "Delete versions newer than N days")

	// Flags for delete graph
	deleteGraphCmd.Flags().BoolVar(&deleteForce, "force", false, "Skip confirmation prompt")
	deleteGraphCmd.Flags().BoolVar(&deleteDryRun, "dry-run", false, "Show what would be deleted without deleting")
	deleteGraphCmd.Flags().StringVar(&deleteGraphDig, "digest", "", "Find graph by digest")
	deleteGraphCmd.Flags().Int64Var(&deleteGraphVer, "version", 0, "Find graph containing this version ID")
}

// Helper functions for delete version bulk operations

// parseDeleteDate attempts to parse a date string in multiple user-friendly formats
func parseDeleteDate(dateStr string) (time.Time, error) {
	if dateStr == "" {
		return time.Time{}, nil
	}

	// Try multiple formats in order of likelihood
	formats := []string{
		"2006-01-02",          // Date only (most convenient)
		time.RFC3339,          // Full datetime with timezone
		"2006-01-02T15:04:05", // Datetime without timezone
		time.RFC3339Nano,      // With fractional seconds
	}

	var lastErr error
	for _, format := range formats {
		t, err := time.Parse(format, dateStr)
		if err == nil {
			return t, nil
		}
		lastErr = err
	}

	return time.Time{}, lastErr
}

// buildDeleteFilter constructs a VersionFilter from delete command flags
func buildDeleteFilter() (*filter.VersionFilter, error) {
	// Check for conflicting flags
	if deleteOnlyTagged && deleteOnlyUntagged {
		return nil, fmt.Errorf("cannot use --tagged and --untagged together")
	}

	vf := &filter.VersionFilter{
		OnlyTagged:    deleteOnlyTagged,
		OnlyUntagged:  deleteOnlyUntagged,
		TagPattern:    deleteTagPattern,
		OlderThanDays: deleteOlderThanDays,
		NewerThanDays: deleteNewerThanDays,
	}

	// Parse older-than date
	if deleteOlderThan != "" {
		t, err := parseDeleteDate(deleteOlderThan)
		if err != nil {
			return nil, fmt.Errorf("invalid --older-than date format (expected YYYY-MM-DD or RFC3339): %w", err)
		}
		vf.OlderThan = t
	}

	// Parse newer-than date
	if deleteNewerThan != "" {
		t, err := parseDeleteDate(deleteNewerThan)
		if err != nil {
			return nil, fmt.Errorf("invalid --newer-than date format (expected YYYY-MM-DD or RFC3339): %w", err)
		}
		vf.NewerThan = t
	}

	return vf, nil
}

// Helper functions for delete graph

func buildGraph(ctx context.Context, ghClient *gh.Client, fullImage, owner, ownerType, imageName, rootDigest, tag string) (*graph.Graph, error) {
	// Create graph with root
	g, err := graph.NewGraph(rootDigest)
	if err != nil {
		return nil, err
	}

	// Fetch all versions once for efficient lookups
	allVersions, err := ghClient.ListPackageVersions(ctx, owner, ownerType, imageName)
	if err != nil {
		return nil, fmt.Errorf("failed to list package versions: %w", err)
	}

	// Create digest→version map
	versionCache := make(map[string]gh.PackageVersionInfo)
	for _, ver := range allVersions {
		versionCache[ver.Name] = ver
	}

	// Set root version info
	if rootInfo, found := versionCache[rootDigest]; found {
		g.Root.SetVersionID(rootInfo.ID)
		for _, t := range rootInfo.Tags {
			g.Root.AddTag(t)
		}
	} else if tag != "" {
		g.Root.AddTag(tag)
	}

	// Get platform manifests
	platformInfos, _ := oras.GetPlatformManifests(ctx, fullImage, rootDigest)

	// Check if we need to find parent graph
	initialReferrers, _ := oras.DiscoverReferrers(ctx, fullImage, rootDigest)
	if len(platformInfos) == 0 && len(initialReferrers) == 0 {
		// This might be a child artifact, try to find parent
		parentDigest, err := findParentDigest(ctx, allVersions, fullImage, rootDigest)
		if err == nil && parentDigest != "" && parentDigest != rootDigest {
			// Found parent, rebuild graph with parent
			return buildGraph(ctx, ghClient, fullImage, owner, ownerType, imageName, parentDigest, tag)
		}
	}

	// Build multi-arch or single-arch graph
	if len(platformInfos) > 0 {
		// Multi-arch image
		for _, pInfo := range platformInfos {
			platform := graph.NewPlatform(pInfo.Digest, pInfo.Platform, pInfo.Architecture, pInfo.OS, pInfo.Variant)
			platform.Size = pInfo.Size

			if pInfo, found := versionCache[pInfo.Digest]; found {
				platform.Manifest.SetVersionID(pInfo.ID)
			}

			// Get platform-specific referrers
			platformReferrers, _ := oras.DiscoverReferrers(ctx, fullImage, pInfo.Digest)
			for _, ref := range platformReferrers {
				artifact, err := graph.NewArtifact(ref.Digest, ref.ArtifactType)
				if err != nil {
					continue
				}
				if refInfo, found := versionCache[ref.Digest]; found {
					artifact.SetVersionID(refInfo.ID)
				}
				platform.AddReferrer(artifact)
			}

			g.AddPlatform(platform)
		}

		// Get index-level referrers
		indexReferrers, _ := oras.DiscoverReferrers(ctx, fullImage, rootDigest)
		for _, ref := range indexReferrers {
			artifact, err := graph.NewArtifact(ref.Digest, ref.ArtifactType)
			if err != nil {
				continue
			}
			if refInfo, found := versionCache[ref.Digest]; found {
				artifact.SetVersionID(refInfo.ID)
			}
			g.AddReferrer(artifact)
		}
	} else {
		// Single-arch image
		referrers, _ := oras.DiscoverReferrers(ctx, fullImage, rootDigest)
		for _, ref := range referrers {
			artifact, err := graph.NewArtifact(ref.Digest, ref.ArtifactType)
			if err != nil {
				continue
			}
			if refInfo, found := versionCache[ref.Digest]; found {
				artifact.SetVersionID(refInfo.ID)
			}
			g.AddReferrer(artifact)
		}
	}

	return g, nil
}

func collectVersionIDs(g *graph.Graph) []int64 {
	ids := []int64{}

	// Collect IDs in deletion order: children first, then root
	// This prevents orphaning artifacts

	// Platform-specific referrers (attestations)
	for _, p := range g.Platforms {
		for _, ref := range p.Referrers {
			if ref.VersionID != 0 {
				ids = append(ids, ref.VersionID)
			}
		}
	}

	// Root-level referrers (attestations)
	for _, ref := range g.Referrers {
		if ref.VersionID != 0 {
			ids = append(ids, ref.VersionID)
		}
	}

	// Platform manifests
	for _, p := range g.Platforms {
		if p.Manifest.VersionID != 0 {
			ids = append(ids, p.Manifest.VersionID)
		}
	}

	// Root (last)
	if g.Root.VersionID != 0 {
		ids = append(ids, g.Root.VersionID)
	}

	return ids
}

func displayGraphSummary(w io.Writer, g *graph.Graph) {
	fmt.Fprintf(w, "Root (Image): %s\n", g.Root.Digest)
	if len(g.Root.Tags) > 0 {
		fmt.Fprintf(w, "  Tags: %v\n", g.Root.Tags)
	}
	if g.Root.VersionID != 0 {
		fmt.Fprintf(w, "  Version ID: %d\n", g.Root.VersionID)
	}

	if len(g.Platforms) > 0 {
		fmt.Fprintf(w, "\nPlatforms (%d):\n", len(g.Platforms))
		for _, p := range g.Platforms {
			fmt.Fprintf(w, "  - %s (version %d)\n", p.Platform, p.Manifest.VersionID)
		}
	}

	totalReferrers := len(g.Referrers)
	for _, p := range g.Platforms {
		totalReferrers += len(p.Referrers)
	}

	if totalReferrers > 0 {
		fmt.Fprintf(w, "\nAttestations (%d):\n", totalReferrers)
		for _, ref := range g.Referrers {
			fmt.Fprintf(w, "  - %s (version %d)\n", ref.Type, ref.VersionID)
		}
		for _, p := range g.Platforms {
			for _, ref := range p.Referrers {
				fmt.Fprintf(w, "  - %s for %s (version %d)\n", ref.Type, p.Platform, ref.VersionID)
			}
		}
	}
}

func deleteGraph(ctx context.Context, client *gh.Client, owner, ownerType, imageName string, versionIDs []int64, w io.Writer) error {
	for i, versionID := range versionIDs {
		fmt.Fprintf(w, "Deleting version %d/%d (ID: %d)...\n", i+1, len(versionIDs), versionID)
		err := client.DeletePackageVersion(ctx, owner, ownerType, imageName, versionID)
		if err != nil {
			return fmt.Errorf("failed to delete version %d: %w", versionID, err)
		}
	}
	return nil
}
