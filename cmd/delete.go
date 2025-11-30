package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mkoepf/ghcrctl/internal/discover"
	"github.com/mkoepf/ghcrctl/internal/display"
	"github.com/mkoepf/ghcrctl/internal/filter"
	"github.com/mkoepf/ghcrctl/internal/gh"
	"github.com/mkoepf/ghcrctl/internal/prompts"
	"github.com/spf13/cobra"
)

// newDeleteCmd creates the delete command and its subcommands with isolated flag state.
func newDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete package versions from GitHub Container Registry",
		Long: `Delete package versions from GitHub Container Registry.

Use subcommands to delete individual versions or complete OCI images.

Available Commands:
  version     Delete a single package version
  image       Delete an entire OCI image (index + platforms + attestations)`,
	}

	// Add subcommands via their factories
	cmd.AddCommand(newDeleteVersionCmd())
	cmd.AddCommand(newDeleteImageCmd())

	return cmd
}

// newDeleteVersionCmd creates the delete version subcommand with isolated flag state.
func newDeleteVersionCmd() *cobra.Command {
	var (
		force         bool
		dryRun        bool
		digest        string
		tagPattern    string
		onlyTagged    bool
		onlyUntagged  bool
		olderThan     string
		newerThan     string
		olderThanDays int
		newerThanDays int
	)

	cmd := &cobra.Command{
		Use:   "version <owner/package> [version-id]",
		Short: "Delete package version(s)",
		Long: `Delete package version(s) from GitHub Container Registry.

This command can delete a single version by ID/digest, or multiple versions using filters.
By default, it will prompt for confirmation before deleting.

IMPORTANT: Deletion is permanent and cannot be undone (except within 30 days
via the GitHub web UI if the package namespace is available).

Examples:
  # Delete by version ID
  ghcrctl delete version mkoepf/myimage 12345678

  # Delete by digest
  ghcrctl delete version mkoepf/myimage --digest sha256:abc123...

  # Delete all untagged versions (bulk deletion)
  ghcrctl delete version mkoepf/myimage --untagged

  # Delete untagged versions older than 30 days
  ghcrctl delete version mkoepf/myimage --untagged --older-than-days 30

  # Delete versions matching tag pattern older than a date
  ghcrctl delete version mkoepf/myimage --tag-pattern ".*-rc.*" --older-than 2025-01-01

  # Preview what would be deleted (dry-run)
  ghcrctl delete version mkoepf/myimage --untagged --dry-run

  # Skip confirmation for bulk deletion
  ghcrctl delete version mkoepf/myimage --untagged --older-than-days 30 --force`,
		Args: func(cmd *cobra.Command, args []string) error {
			// Check if any filter flags are set (indicates bulk deletion)
			filterFlagsSet := onlyTagged || onlyUntagged || tagPattern != "" ||
				olderThan != "" || newerThan != "" ||
				olderThanDays > 0 || newerThanDays > 0

			// If --digest is provided, we need exactly 1 arg (owner/image)
			if digest != "" {
				if len(args) != 1 {
					return fmt.Errorf("accepts 1 arg(s) when using --digest, received %d", len(args))
				}
				return nil
			}

			// If filter flags are set (bulk deletion), we need exactly 1 arg (owner/package)
			if filterFlagsSet {
				if len(args) != 1 {
					return fmt.Errorf("accepts 1 arg (owner/package) when using filters, received %d", len(args))
				}
				return nil
			}

			// Otherwise, we need exactly 2 args (owner/image and version-id)
			return cobra.ExactArgs(2)(cmd, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// Parse owner/image reference
			owner, imageName, _, err := parseImageRef(args[0])
			if err != nil {
				cmd.SilenceUsage = true
				return err
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

			// Auto-detect owner type
			ownerType, err := client.GetOwnerType(ctx, owner)
			if err != nil {
				cmd.SilenceUsage = true
				return fmt.Errorf("failed to determine owner type: %w", err)
			}

			// Check if this is bulk deletion mode
			filterFlagsSet := onlyTagged || onlyUntagged || tagPattern != "" ||
				olderThan != "" || newerThan != "" ||
				olderThanDays > 0 || newerThanDays > 0

			if filterFlagsSet {
				// Bulk deletion mode
				return runBulkDeleteWithFlags(ctx, cmd, client, owner, ownerType, imageName,
					tagPattern, onlyTagged, onlyUntagged, olderThan, newerThan,
					olderThanDays, newerThanDays, force, dryRun)
			}

			// Single deletion mode
			return runSingleDeleteWithFlags(ctx, cmd, args, client, owner, ownerType, imageName,
				digest, force, dryRun)
		},
	}

	// Flags for delete version
	cmd.Flags().BoolVar(&force, "force", false, "Skip confirmation prompt")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be deleted without deleting")
	cmd.Flags().StringVar(&digest, "digest", "", "Delete version by digest")

	// Filter flags for bulk deletion
	cmd.Flags().StringVar(&tagPattern, "tag-pattern", "", "Delete versions matching regex pattern")
	cmd.Flags().BoolVar(&onlyTagged, "tagged", false, "Delete only tagged versions")
	cmd.Flags().BoolVar(&onlyUntagged, "untagged", false, "Delete only untagged versions")
	cmd.Flags().StringVar(&olderThan, "older-than", "", "Delete versions older than date (YYYY-MM-DD or RFC3339)")
	cmd.Flags().StringVar(&newerThan, "newer-than", "", "Delete versions newer than date (YYYY-MM-DD or RFC3339)")
	cmd.Flags().IntVar(&olderThanDays, "older-than-days", 0, "Delete versions older than N days")
	cmd.Flags().IntVar(&newerThanDays, "newer-than-days", 0, "Delete versions newer than N days")

	return cmd
}

// newDeleteImageCmd creates the delete image subcommand with isolated flag state.
func newDeleteImageCmd() *cobra.Command {
	var (
		force     bool
		dryRun    bool
		digest    string
		versionID int64
	)

	cmd := &cobra.Command{
		Use:   "image <owner/package[:tag]>",
		Short: "Delete an entire OCI image",
		Long: `Delete an entire OCI image from GitHub Container Registry.

This command discovers and deletes all versions that make up an OCI image,
including the root index, platform manifests, and attestations (SBOM, provenance).

The image can be identified by:
  - Tag (in the image reference) - most common
  - Digest (--digest flag)
  - Version ID (--version flag) - finds the image containing this version

IMPORTANT: Deletion is permanent and cannot be undone (except within 30 days
via the GitHub web UI if the package namespace is available).

Examples:
  # Delete image by tag (most common)
  ghcrctl delete image mkoepf/myimage:v1.0.0

  # Delete image by digest
  ghcrctl delete image mkoepf/myimage --digest sha256:abc123...

  # Delete image containing a specific version
  ghcrctl delete image mkoepf/myimage --version 12345678

  # Skip confirmation
  ghcrctl delete image mkoepf/myimage:v1.0.0 --force

  # Preview what would be deleted
  ghcrctl delete image mkoepf/myimage:v1.0.0 --dry-run`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Parse owner/image[:tag] reference
			owner, imageName, tag, err := parseImageRef(args[0])
			if err != nil {
				cmd.SilenceUsage = true
				return err
			}

			// Validate: need either tag in reference, or --digest, or --version
			hasDigestFlag := cmd.Flags().Changed("digest")
			hasVersionFlag := cmd.Flags().Changed("version")

			if hasDigestFlag && hasVersionFlag {
				cmd.SilenceUsage = true
				return fmt.Errorf("flags --digest and --version are mutually exclusive")
			}

			if tag == "" && !hasDigestFlag && !hasVersionFlag {
				cmd.SilenceUsage = true
				return fmt.Errorf("tag required: use format owner/image:tag, or use --digest or --version flag")
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

			// Auto-detect owner type
			ownerType, err := ghClient.GetOwnerType(ctx, owner)
			if err != nil {
				cmd.SilenceUsage = true
				return fmt.Errorf("failed to determine owner type: %w", err)
			}

			fullImage := fmt.Sprintf("ghcr.io/%s/%s", owner, imageName)

			// Determine the root digest based on which flag/argument was used
			var rootDigest string

			if digest != "" {
				// Lookup by digest - support both full and short (prefix) format
				digestInput := digest
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
						return fmt.Errorf("no version found matching digest prefix %s", digest)
					}
				} else {
					rootDigest = digestInput
				}
			} else if versionID != 0 {
				// Lookup by version ID - need to find the digest first
				allVersions, err := ghClient.ListPackageVersions(ctx, owner, ownerType, imageName)
				if err != nil {
					cmd.SilenceUsage = true
					return fmt.Errorf("failed to list package versions: %w", err)
				}

				var found bool
				for _, ver := range allVersions {
					if ver.ID == versionID {
						rootDigest = ver.Name
						found = true
						break
					}
				}

				if !found {
					cmd.SilenceUsage = true
					return fmt.Errorf("version ID %d not found", versionID)
				}
			} else {
				// Lookup by tag (from image reference)
				rootDigest, err = discover.ResolveTag(ctx, fullImage, tag)
				if err != nil {
					cmd.SilenceUsage = true
					return fmt.Errorf("failed to resolve tag '%s': %w", tag, err)
				}
			}

			// Use discover package to get all versions and their relationships
			allVersions, err := ghClient.ListPackageVersions(ctx, owner, ownerType, imageName)
			if err != nil {
				cmd.SilenceUsage = true
				return fmt.Errorf("failed to list package versions: %w", err)
			}

			discoverer := discover.NewPackageDiscoverer()
			versions, err := discoverer.DiscoverPackage(ctx, fullImage, allVersions, nil)
			if err != nil {
				cmd.SilenceUsage = true
				return fmt.Errorf("failed to discover package: %w", err)
			}

			versionMap := discover.ToMap(versions)

			// Find image by root digest
			imageVersions := discover.FindImageByDigest(versionMap, rootDigest)
			if len(imageVersions) == 0 {
				cmd.SilenceUsage = true
				return fmt.Errorf("no image found for digest %s", rootDigest)
			}

			// Classify versions into exclusive (to delete) and shared (to preserve)
			toDelete, shared := discover.ClassifyImageVersions(imageVersions, versionMap)

			// Extract version IDs from toDelete list
			versionIDs := make([]int64, len(toDelete))
			for i, v := range toDelete {
				versionIDs[i] = v.ID
			}

			// Display what will be deleted
			fmt.Fprintf(cmd.OutOrStdout(), "Preparing to delete complete OCI image:\n")
			fmt.Fprintf(cmd.OutOrStdout(), "  Image: %s\n", imageName)
			if tag != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "  Tag:   %s\n", tag)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\n")
			displayImageVersions(cmd.OutOrStdout(), toDelete, shared, versionMap)
			fmt.Fprintf(cmd.OutOrStdout(), "\nTotal: %s version(s) will be deleted\n\n",
				display.ColorWarning(fmt.Sprintf("%d", len(versionIDs))))

			// Handle dry-run
			if dryRun {
				fmt.Fprintln(cmd.OutOrStdout(), display.ColorDryRun("DRY RUN: No changes made"))
				return nil
			}

			// Confirm deletion unless --force is used
			if !force {
				confirmed, err := prompts.Confirm(os.Stdin, cmd.OutOrStdout(), display.ColorWarning("Are you sure you want to delete this image?"))
				if err != nil {
					return fmt.Errorf("failed to read confirmation: %w", err)
				}

				if !confirmed {
					fmt.Fprintln(cmd.OutOrStdout(), "Deletion cancelled")
					return nil
				}
			}

			// Perform deletions (children first, then root)
			err = deleteGraphVersions(ctx, ghClient, owner, ownerType, imageName, versionIDs, cmd.OutOrStdout())
			if err != nil {
				cmd.SilenceUsage = true
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "\n%s\n",
				display.ColorSuccess(fmt.Sprintf("Successfully deleted %d version(s) of %s", len(versionIDs), imageName)))
			return nil
		},
	}

	// Flags for delete image
	cmd.Flags().BoolVar(&force, "force", false, "Skip confirmation prompt")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be deleted without deleting")
	cmd.Flags().StringVar(&digest, "digest", "", "Find image by digest")
	cmd.Flags().Int64Var(&versionID, "version", 0, "Find image containing this version ID")

	return cmd
}

// runSingleDeleteWithFlags handles deletion of a single version with explicit flag values
func runSingleDeleteWithFlags(ctx context.Context, cmd *cobra.Command, args []string, client *gh.Client, owner, ownerType, imageName string,
	digestFlag string, force, dryRun bool) error {
	var versionID int64
	var err error

	// Determine version ID from either positional arg or --digest flag
	if digestFlag != "" {
		// Normalize digest format
		digest := digestFlag
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

	// Count how many images this version belongs to
	imageCount := countImageMembership(ctx, client, owner, ownerType, imageName, versionID)

	// Build params and delegate to testable function
	params := deleteVersionParams{
		owner:      owner,
		ownerType:  ownerType,
		imageName:  imageName,
		versionID:  versionID,
		tags:       tags,
		imageCount: imageCount,
		force:      force,
		dryRun:     dryRun,
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

// runBulkDeleteWithFlags handles deletion of multiple versions using filters with explicit flag values
func runBulkDeleteWithFlags(ctx context.Context, cmd *cobra.Command, client *gh.Client, owner, ownerType, imageName string,
	tagPattern string, onlyTagged, onlyUntagged bool, olderThan, newerThan string,
	olderThanDays, newerThanDays int, force, dryRun bool) error {
	// Build filter from flags
	versionFilter, err := buildDeleteFilterWithFlags(tagPattern, onlyTagged, onlyUntagged, olderThan, newerThan, olderThanDays, newerThanDays)
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
	matchingVersions := versionFilter.Apply(allVersions)

	// Check if any versions match
	if len(matchingVersions) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No versions match the specified filters")
		return nil
	}

	// Build all images to identify shared children that should be protected
	fullImage := fmt.Sprintf("ghcr.io/%s/%s", owner, imageName)
	discoverer := discover.NewPackageDiscoverer()
	versions, err := discoverer.DiscoverPackage(ctx, fullImage, allVersions, nil)

	// Track which version IDs are shared children (belong to multiple images)
	sharedChildren := make(map[int64]bool)
	if err == nil {
		versionMap := discover.ToMap(versions)
		for _, ver := range versions {
			if discover.CountImageMembershipByID(versionMap, ver.ID) > 1 {
				sharedChildren[ver.ID] = true
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
		fmt.Fprintf(cmd.OutOrStdout(), "%s %d version(s) are shared by multiple images and will be preserved.\n\n",
			display.ColorWarning("Note:"), len(protectedVersions))
	}

	matchingVersions = safeToDelete

	// Re-check if any versions remain after filtering
	if len(matchingVersions) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No versions to delete (all matching versions are shared with other images)")
		return nil
	}

	// Build params and delegate to testable function
	params := bulkDeleteParams{
		owner:     owner,
		ownerType: ownerType,
		imageName: imageName,
		versions:  matchingVersions,
		force:     force,
		dryRun:    dryRun,
	}

	confirmFn := func(count int) (bool, error) {
		return prompts.Confirm(os.Stdin, cmd.OutOrStdout(),
			display.ColorWarning(fmt.Sprintf("Are you sure you want to delete %d version(s)?", count)))
	}

	return executeBulkDelete(ctx, client, params, cmd.OutOrStdout(), confirmFn)
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
	imageCount int
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
	if params.imageCount > 0 {
		imageWord := "image"
		if params.imageCount > 1 {
			imageWord = "images"
		}
		fmt.Fprintf(w, "  Images:     %s\n", display.ColorShared(fmt.Sprintf("%d %s", params.imageCount, imageWord)))
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

// buildDeleteFilterWithFlags constructs a VersionFilter from explicit flag values
func buildDeleteFilterWithFlags(tagPattern string, onlyTagged, onlyUntagged bool,
	olderThan, newerThan string, olderThanDays, newerThanDays int) (*filter.VersionFilter, error) {
	// Check for conflicting flags
	if onlyTagged && onlyUntagged {
		return nil, fmt.Errorf("cannot use --tagged and --untagged together")
	}

	vf := &filter.VersionFilter{
		OnlyTagged:    onlyTagged,
		OnlyUntagged:  onlyUntagged,
		TagPattern:    tagPattern,
		OlderThanDays: olderThanDays,
		NewerThanDays: newerThanDays,
	}

	// Parse older-than date
	if olderThan != "" {
		t, err := parseDeleteDate(olderThan)
		if err != nil {
			return nil, fmt.Errorf("invalid --older-than date format (expected YYYY-MM-DD or RFC3339): %w", err)
		}
		vf.OlderThan = t
	}

	// Parse newer-than date
	if newerThan != "" {
		t, err := parseDeleteDate(newerThan)
		if err != nil {
			return nil, fmt.Errorf("invalid --newer-than date format (expected YYYY-MM-DD or RFC3339): %w", err)
		}
		vf.NewerThan = t
	}

	return vf, nil
}

func deleteGraphVersions(ctx context.Context, client *gh.Client, owner, ownerType, imageName string, versionIDs []int64, w io.Writer) error {
	return deleteGraphWithDeleter(ctx, client, owner, ownerType, imageName, versionIDs, w)
}

// displayImageVersions displays versions to delete and shared versions.
func displayImageVersions(w io.Writer, toDelete, shared []discover.VersionInfo, versionMap map[string]discover.VersionInfo) {
	// Show what will be deleted
	if len(toDelete) > 0 {
		fmt.Fprintf(w, "Versions to delete (%d):\n", len(toDelete))
		for _, v := range toDelete {
			fmt.Fprintf(w, "  - %s (version %d)%s\n", formatType(v.Types), v.ID, formatTags(v.Tags))
		}
	}

	// Show what will be preserved (shared with other images)
	if len(shared) > 0 {
		fmt.Fprintf(w, "\n%s\n", display.ColorWarning("Shared versions (preserved, used by other images):"))
		for _, v := range shared {
			refCount := discover.CountImageMembership(versionMap, v.Digest)
			fmt.Fprintf(w, "  - %s (version %d, shared by %d images)%s\n", formatType(v.Types), v.ID, refCount, formatTags(v.Tags))
		}
	}
}

// formatType returns the first type or "unknown" if none.
func formatType(types []string) string {
	if len(types) > 0 {
		return types[0]
	}
	return "unknown"
}

// formatTags returns formatted tags or empty string if none.
func formatTags(tags []string) string {
	if len(tags) > 0 {
		return fmt.Sprintf(" [%s]", strings.Join(tags, ", "))
	}
	return ""
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

// countImageMembership returns how many images the given version ID belongs to
// (either as root or as a child). Returns 0 if unable to determine.
func countImageMembership(ctx context.Context, client *gh.Client, owner, ownerType, imageName string, versionID int64) int {
	// Get all versions for this package
	allVersions, err := client.ListPackageVersions(ctx, owner, ownerType, imageName)
	if err != nil {
		return 0
	}

	fullImage := fmt.Sprintf("ghcr.io/%s/%s", owner, imageName)

	// Use discover package to get version relationships
	discoverer := discover.NewPackageDiscoverer()
	versions, err := discoverer.DiscoverPackage(ctx, fullImage, allVersions, nil)
	if err != nil {
		return 0
	}

	// Convert to map and count membership
	versionMap := discover.ToMap(versions)
	return discover.CountImageMembershipByID(versionMap, versionID)
}
