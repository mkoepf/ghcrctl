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
	"github.com/mhk/ghcrctl/internal/discovery"
	"github.com/mhk/ghcrctl/internal/display"
	"github.com/mhk/ghcrctl/internal/filter"
	"github.com/mhk/ghcrctl/internal/gh"
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
			// Lookup by digest - support both full and short (prefix) format
			digestInput := deleteGraphDig
			if !strings.HasPrefix(digestInput, "sha256:") {
				digestInput = "sha256:" + digestInput
			}

			// If it looks like a short digest, resolve to full digest
			if len(digestInput) < 71 { // sha256: (7) + 64 hex chars = 71
				allVersions, err := ghClient.ListPackageVersions(ctx, owner, ownerType, imageName)
				if err != nil {
					cmd.SilenceUsage = true
					return fmt.Errorf("failed to list package versions: %w", err)
				}

				var found bool
				for _, ver := range allVersions {
					if strings.HasPrefix(ver.Name, digestInput) {
						rootDigest = ver.Name
						found = true
						break
					}
				}

				if !found {
					cmd.SilenceUsage = true
					return fmt.Errorf("no version found matching digest prefix %s", deleteGraphDig)
				}
			} else {
				rootDigest = digestInput
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
		if g == nil {
			cmd.SilenceUsage = true
			return fmt.Errorf("no graph found for digest %s", rootDigest)
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
		fmt.Fprintf(cmd.OutOrStdout(), "\nTotal: %s version(s) will be deleted\n\n",
			display.ColorWarning(fmt.Sprintf("%d", len(versionIDs))))

		// Handle dry-run
		if deleteDryRun {
			fmt.Fprintln(cmd.OutOrStdout(), display.ColorDryRun("DRY RUN: No changes made"))
			return nil
		}

		// Confirm deletion unless --force is used
		if !deleteForce {
			confirmed, err := prompts.Confirm(os.Stdin, cmd.OutOrStdout(), display.ColorWarning("Are you sure you want to delete this graph?"))
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

		fmt.Fprintf(cmd.OutOrStdout(), "\n%s\n",
			display.ColorSuccess(fmt.Sprintf("Successfully deleted %d version(s) of %s", len(versionIDs), imageName)))
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

	// Count how many graphs this version belongs to
	graphCount := countGraphMembership(ctx, client, owner, ownerType, imageName, versionID)

	// Build params and delegate to testable function
	params := deleteVersionParams{
		owner:      owner,
		ownerType:  ownerType,
		imageName:  imageName,
		versionID:  versionID,
		tags:       tags,
		graphCount: graphCount,
		force:      deleteForce,
		dryRun:     deleteDryRun,
	}

	confirmFn := func() (bool, error) {
		return prompts.Confirm(os.Stdin, cmd.OutOrStdout(), display.ColorWarning("Are you sure you want to delete this version?"))
	}

	err = executeSingleDelete(ctx, client, params, cmd.OutOrStdout(), confirmFn)
	if err != nil {
		cmd.SilenceUsage = true
		return err
	}
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

	// Build all graphs to identify shared children that should be protected
	fullImage := fmt.Sprintf("ghcr.io/%s/%s", owner, imageName)
	allGraphs, err := buildVersionGraphs(ctx, fullImage, allVersions, allVersions, client, owner, ownerType, imageName)

	// Track which version IDs are shared children (RefCount > 1)
	sharedChildren := make(map[int64]bool)
	if err == nil {
		for _, g := range allGraphs {
			for _, child := range g.Children {
				if child.RefCount > 1 {
					sharedChildren[child.Version.ID] = true
				}
			}
		}
	}

	// Filter out shared children from deletion list
	var safeToDelete []gh.PackageVersionInfo
	var protectedVersions []gh.PackageVersionInfo
	for _, ver := range matchingVersions {
		if sharedChildren[ver.ID] {
			protectedVersions = append(protectedVersions, ver)
		} else {
			safeToDelete = append(safeToDelete, ver)
		}
	}

	// Update matching versions to only include safe ones
	if len(protectedVersions) > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "%s %d version(s) are shared by multiple graphs and will be preserved.\n\n",
			display.ColorWarning("Note:"), len(protectedVersions))
	}

	matchingVersions = safeToDelete

	// Re-check if any versions remain after filtering
	if len(matchingVersions) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No versions to delete (all matching versions are shared with other graphs)")
		return nil
	}

	// Build params and delegate to testable function
	params := bulkDeleteParams{
		owner:     owner,
		ownerType: ownerType,
		imageName: imageName,
		versions:  matchingVersions,
		force:     deleteForce,
		dryRun:    deleteDryRun,
	}

	confirmFn := func(count int) (bool, error) {
		return prompts.Confirm(os.Stdin, cmd.OutOrStdout(),
			display.ColorWarning(fmt.Sprintf("Are you sure you want to delete %d version(s)?", count)))
	}

	return executeBulkDelete(ctx, client, params, cmd.OutOrStdout(), confirmFn)
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

// deleteVersionParams holds the parameters for single version deletion.
// This struct allows for dependency injection and testing.
type deleteVersionParams struct {
	owner      string
	ownerType  string
	imageName  string
	versionID  int64
	tags       []string
	graphCount int
	force      bool
	dryRun     bool
}

// executeSingleDelete performs the actual deletion after all parameters are resolved.
// This function is extracted for testability.
func executeSingleDelete(ctx context.Context, deleter gh.PackageDeleter, params deleteVersionParams, w io.Writer, confirmFn func() (bool, error)) error {
	// Show what will be deleted
	fmt.Fprintf(w, "Preparing to delete package version:\n")
	fmt.Fprintf(w, "  Image:      %s\n", params.imageName)
	fmt.Fprintf(w, "  Owner:      %s (%s)\n", params.owner, params.ownerType)
	fmt.Fprintf(w, "  Version ID: %d\n", params.versionID)
	fmt.Fprintf(w, "  Tags:       %s\n", FormatTagsForDisplay(params.tags))
	if params.graphCount > 0 {
		graphWord := "graph"
		if params.graphCount > 1 {
			graphWord = "graphs"
		}
		fmt.Fprintf(w, "  Graphs:     %s\n", display.ColorShared(fmt.Sprintf("%d %s", params.graphCount, graphWord)))
	}
	fmt.Fprintln(w)

	// Handle dry-run
	if params.dryRun {
		fmt.Fprintln(w, display.ColorDryRun("DRY RUN: No changes made"))
		return nil
	}

	// Confirm deletion unless --force is used
	if !params.force {
		confirmed, err := confirmFn()
		if err != nil {
			return fmt.Errorf("failed to read confirmation: %w", err)
		}

		if !confirmed {
			fmt.Fprintln(w, "Deletion cancelled")
			return nil
		}
	}

	// Perform deletion
	err := deleter.DeletePackageVersion(ctx, params.owner, params.ownerType, params.imageName, params.versionID)
	if err != nil {
		return fmt.Errorf("failed to delete package version: %w", err)
	}

	fmt.Fprintln(w, display.ColorSuccess(fmt.Sprintf("Successfully deleted version %d of %s", params.versionID, params.imageName)))
	return nil
}

// bulkDeleteParams holds the parameters for bulk deletion.
type bulkDeleteParams struct {
	owner     string
	ownerType string
	imageName string
	versions  []gh.PackageVersionInfo
	force     bool
	dryRun    bool
}

// executeBulkDelete performs bulk deletion of versions.
// This function is extracted for testability.
func executeBulkDelete(ctx context.Context, deleter gh.PackageDeleter, params bulkDeleteParams, w io.Writer, confirmFn func(count int) (bool, error)) error {
	// Display summary of what will be deleted
	fmt.Fprintf(w, "Preparing to delete %s package version(s):\n",
		display.ColorWarning(fmt.Sprintf("%d", len(params.versions))))
	fmt.Fprintf(w, "  Image: %s\n", params.imageName)
	fmt.Fprintf(w, "  Owner: %s (%s)\n\n", params.owner, params.ownerType)

	// Show details of matching versions (limit to first 10 for readability)
	displayLimit := 10
	for i, ver := range params.versions {
		if i >= displayLimit {
			fmt.Fprintf(w, "  ... and %d more\n", len(params.versions)-displayLimit)
			break
		}

		tagsStr := FormatTagsForDisplay(ver.Tags)
		fmt.Fprintf(w, "  - ID: %d, Tags: %s, Created: %s\n", ver.ID, tagsStr, ver.CreatedAt)
	}
	fmt.Fprintln(w)

	// Handle dry-run
	if params.dryRun {
		fmt.Fprintln(w, display.ColorDryRun("DRY RUN: No changes made"))
		return nil
	}

	// Confirm deletion unless --force is used
	if !params.force {
		confirmed, err := confirmFn(len(params.versions))
		if err != nil {
			return fmt.Errorf("failed to read confirmation: %w", err)
		}

		if !confirmed {
			fmt.Fprintln(w, "Deletion cancelled")
			return nil
		}
	}

	// Perform bulk deletion
	successCount := 0
	failCount := 0
	for i, ver := range params.versions {
		fmt.Fprintf(w, "Deleting version %d/%d (ID: %d)...\n", i+1, len(params.versions), ver.ID)
		err := deleter.DeletePackageVersion(ctx, params.owner, params.ownerType, params.imageName, ver.ID)
		if err != nil {
			fmt.Fprintf(w, "  %s\n", display.ColorError(fmt.Sprintf("Failed: %v", err)))
			failCount++
		} else {
			successCount++
		}
	}

	// Summary
	fmt.Fprintln(w)
	if failCount > 0 {
		fmt.Fprintf(w, "Deletion complete: %s succeeded, %s failed\n",
			display.ColorSuccess(fmt.Sprintf("%d", successCount)),
			display.ColorError(fmt.Sprintf("%d", failCount)))
	} else {
		fmt.Fprintf(w, "Deletion complete: %s succeeded\n",
			display.ColorSuccess(fmt.Sprintf("%d", successCount)))
	}

	if failCount > 0 {
		return fmt.Errorf("failed to delete %d version(s)", failCount)
	}

	return nil
}

// FormatTagsForDisplay formats a tag slice for display
// Returns "[]" for empty/nil slices, or comma-separated tags otherwise
func FormatTagsForDisplay(tags []string) string {
	if len(tags) == 0 {
		return "[]"
	}
	return strings.Join(tags, ", ")
}

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

func buildGraph(ctx context.Context, ghClient *gh.Client, fullImage, owner, ownerType, imageName, rootDigest, tag string) (*discovery.VersionGraph, error) {
	// Create GraphBuilder
	builder := discovery.NewGraphBuilder(ctx, ghClient, fullImage, owner, ownerType, imageName)

	// Fetch all versions once for efficient lookups
	cache, err := builder.GetVersionCache()
	if err != nil {
		return nil, fmt.Errorf("failed to get version cache: %w", err)
	}

	// Check if we need to find parent graph (if rootDigest has no children)
	platformInfos, _ := oras.GetPlatformManifests(ctx, fullImage, rootDigest)
	initialReferrers, _ := oras.DiscoverReferrers(ctx, fullImage, rootDigest)
	if len(platformInfos) == 0 && len(initialReferrers) == 0 {
		// This might be a child artifact, try to find parent
		parentDigest, err := builder.FindParentDigest(rootDigest, cache)
		if err == nil && parentDigest != "" && parentDigest != rootDigest {
			// Found parent, rebuild graph with parent
			return buildGraph(ctx, ghClient, fullImage, owner, ownerType, imageName, parentDigest, tag)
		}
	}

	// Build the target graph
	targetGraph, err := builder.BuildGraph(rootDigest, cache)
	if err != nil || targetGraph == nil {
		return targetGraph, err
	}

	// To calculate RefCount correctly, we need to check how many OTHER graphs
	// reference the same children. Build all graphs and count references.
	allVersions, err := ghClient.ListPackageVersions(ctx, owner, ownerType, imageName)
	if err != nil {
		// If we can't list versions, return graph without RefCount (conservative: no deletion of children)
		for i := range targetGraph.Children {
			targetGraph.Children[i].RefCount = 2 // Assume shared to be safe
		}
		return targetGraph, nil
	}

	// Build all graphs to calculate reference counts
	allGraphs, err := buildVersionGraphs(ctx, fullImage, allVersions, allVersions, ghClient, owner, ownerType, imageName)
	if err != nil {
		// Fallback: assume shared
		for i := range targetGraph.Children {
			targetGraph.Children[i].RefCount = 2
		}
		return targetGraph, nil
	}

	// Find the target graph in allGraphs and use its RefCount values
	return findGraphByDigest(allGraphs, rootDigest, targetGraph)
}

// findGraphByDigest finds a graph by its root digest from a list of graphs.
// If found, returns the matching graph. If not found, applies conservative
// RefCount (2) to the fallback graph and returns it.
// This is extracted for testability.
func findGraphByDigest(graphs []discovery.VersionGraph, rootDigest string, fallback *discovery.VersionGraph) (*discovery.VersionGraph, error) {
	for _, g := range graphs {
		if g.RootVersion.Name == rootDigest {
			return &g, nil
		}
	}

	// If not found, return fallback with conservative RefCount
	if fallback != nil {
		for i := range fallback.Children {
			fallback.Children[i].RefCount = 2
		}
	}
	return fallback, nil
}

func collectVersionIDs(g *discovery.VersionGraph) []int64 {
	ids := []int64{}

	// Collect IDs in deletion order: children first, then root
	// This prevents orphaning artifacts
	//
	// IMPORTANT: Skip shared children (RefCount > 1) - they belong to other graphs too
	// and should only be deleted when their last parent graph is deleted.

	// First: attestations (sbom, provenance, etc.) - only exclusive ones
	for _, child := range g.Children {
		if !child.Type.IsPlatform() && child.Version.ID != 0 && child.RefCount <= 1 {
			ids = append(ids, child.Version.ID)
		}
	}

	// Second: platform manifests - only exclusive ones
	for _, child := range g.Children {
		if child.Type.IsPlatform() && child.Version.ID != 0 && child.RefCount <= 1 {
			ids = append(ids, child.Version.ID)
		}
	}

	// Last: root
	if g.RootVersion.ID != 0 {
		ids = append(ids, g.RootVersion.ID)
	}

	return ids
}

func displayGraphSummary(w io.Writer, g *discovery.VersionGraph) {
	fmt.Fprintf(w, "Root (Image): %s\n", g.RootVersion.Name)
	if len(g.RootVersion.Tags) > 0 {
		fmt.Fprintf(w, "  Tags: %v\n", g.RootVersion.Tags)
	}
	if g.RootVersion.ID != 0 {
		fmt.Fprintf(w, "  Version ID: %d\n", g.RootVersion.ID)
	}

	// Separate children into exclusive (will delete) and shared (will preserve)
	var exclusivePlatforms, sharedPlatforms []discovery.VersionChild
	var exclusiveAttestations, sharedAttestations []discovery.VersionChild

	for _, child := range g.Children {
		if child.Type.IsPlatform() {
			if child.RefCount > 1 {
				sharedPlatforms = append(sharedPlatforms, child)
			} else {
				exclusivePlatforms = append(exclusivePlatforms, child)
			}
		} else {
			if child.RefCount > 1 {
				sharedAttestations = append(sharedAttestations, child)
			} else {
				exclusiveAttestations = append(exclusiveAttestations, child)
			}
		}
	}

	// Show what will be deleted
	if len(exclusivePlatforms) > 0 {
		fmt.Fprintf(w, "\nPlatforms to delete (%d):\n", len(exclusivePlatforms))
		for _, p := range exclusivePlatforms {
			fmt.Fprintf(w, "  - %s (version %d)\n", p.Type.Platform, p.Version.ID)
		}
	}

	if len(exclusiveAttestations) > 0 {
		fmt.Fprintf(w, "\nAttestations to delete (%d):\n", len(exclusiveAttestations))
		for _, att := range exclusiveAttestations {
			fmt.Fprintf(w, "  - %s (version %d)\n", att.Type.Role, att.Version.ID)
		}
	}

	// Show what will be preserved (shared with other graphs)
	if len(sharedPlatforms) > 0 || len(sharedAttestations) > 0 {
		fmt.Fprintf(w, "\n%s\n", display.ColorWarning("Shared artifacts (preserved, used by other graphs):"))
		for _, p := range sharedPlatforms {
			fmt.Fprintf(w, "  - %s (version %d, shared by %d graphs)\n", p.Type.Platform, p.Version.ID, p.RefCount)
		}
		for _, att := range sharedAttestations {
			fmt.Fprintf(w, "  - %s (version %d, shared by %d graphs)\n", att.Type.Role, att.Version.ID, att.RefCount)
		}
	}
}

func deleteGraph(ctx context.Context, client *gh.Client, owner, ownerType, imageName string, versionIDs []int64, w io.Writer) error {
	return deleteGraphWithDeleter(ctx, client, owner, ownerType, imageName, versionIDs, w)
}

// deleteGraphWithDeleter is the internal implementation that accepts a PackageDeleter interface.
// This allows for dependency injection and testing with mocks.
func deleteGraphWithDeleter(ctx context.Context, deleter gh.PackageDeleter, owner, ownerType, imageName string, versionIDs []int64, w io.Writer) error {
	for i, versionID := range versionIDs {
		fmt.Fprintf(w, "Deleting version %d/%d (ID: %d)...\n", i+1, len(versionIDs), versionID)
		err := deleter.DeletePackageVersion(ctx, owner, ownerType, imageName, versionID)
		if err != nil {
			return fmt.Errorf("failed to delete version %d: %w", versionID, err)
		}
	}
	return nil
}

// countGraphMembership returns how many graphs the given version ID belongs to
// (either as root or as a child). Returns 0 if unable to determine.
func countGraphMembership(ctx context.Context, client *gh.Client, owner, ownerType, imageName string, versionID int64) int {
	// Get all versions for this image
	allVersions, err := client.ListPackageVersions(ctx, owner, ownerType, imageName)
	if err != nil {
		return 0
	}

	fullImage := fmt.Sprintf("ghcr.io/%s/%s", owner, imageName)

	// Build all graphs
	graphs, err := buildVersionGraphs(ctx, fullImage, allVersions, allVersions, client, owner, ownerType, imageName)
	if err != nil {
		return 0
	}

	return countVersionInGraphs(graphs, versionID)
}

// countVersionInGraphs counts how many graphs contain the given version ID
// (either as root or as a child). This is extracted for testability.
func countVersionInGraphs(graphs []discovery.VersionGraph, versionID int64) int {
	count := 0
	for _, g := range graphs {
		// Check if it's the root
		if g.RootVersion.ID == versionID {
			count++
			continue
		}
		// Check if it's a child
		for _, child := range g.Children {
			if child.Version.ID == versionID {
				count++
				break
			}
		}
	}

	return count
}
