package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

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
		Short: "Delete resources (versions, images, packages)",
		Long: `Delete resources from GitHub Container Registry.

Use subcommands to delete individual versions, complete OCI images, or entire packages.

Available Commands:
  version     Delete a single package version or bulk delete with filters
  image       Delete an entire OCI image (index + platforms + attestations)
  package     Delete an entire package (all versions)`,
	}

	// Add subcommands via their factories
	cmd.AddCommand(newDeleteVersionCmd())
	cmd.AddCommand(newDeleteImageCmd())
	cmd.AddCommand(newDeletePackageCmd())

	return cmd
}

// newDeleteVersionCmd creates the delete version subcommand with isolated flag state.
func newDeleteVersionCmd() *cobra.Command {
	var (
		force         bool
		yes           bool
		dryRun        bool
		versionID     int64
		digest        string
		tag           string
		tagPattern    string
		onlyTagged    bool
		onlyUntagged  bool
		olderThan     string
		newerThan     string
		olderThanDays int
		newerThanDays int
	)

	cmd := &cobra.Command{
		Use:   "version <owner/package>",
		Short: "Delete package version(s)",
		Long: `Delete package version(s) from GitHub Container Registry.

This command can delete a single version by ID/digest/tag, or multiple versions using filters.
By default, it will prompt for confirmation before deleting.

IMPORTANT: Deletion is permanent and cannot be undone (except within 30 days
via the GitHub web UI if the package namespace is available).

Requires a selector: --version, --digest, --tag, or filter flags for bulk deletion.

Examples:
  # Delete by version ID
  ghcrctl delete version mkoepf/myimage --version 12345678

  # Delete by digest
  ghcrctl delete version mkoepf/myimage --digest sha256:abc123...

  # Delete by tag
  ghcrctl delete version mkoepf/myimage --tag v1.0.0

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
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Parse owner/package reference (reject inline tags)
			owner, packageName, err := parsePackageRef(args[0])
			if err != nil {
				cmd.SilenceUsage = true
				return err
			}

			// Check if any selector is provided
			hasSingleSelector := versionID != 0 || digest != "" || tag != ""
			hasFilterSelector := onlyTagged || onlyUntagged || tagPattern != "" ||
				olderThan != "" || newerThan != "" ||
				olderThanDays > 0 || newerThanDays > 0

			if !hasSingleSelector && !hasFilterSelector {
				cmd.SilenceUsage = true
				return fmt.Errorf("selector required: use --version, --digest, --tag, or filter flags (--untagged, --older-than-days, etc.)")
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

			// Route to appropriate handler
			skipConfirm := force || yes
			if hasFilterSelector && !hasSingleSelector {
				// Bulk deletion mode
				return runBulkDeleteVersion(ctx, cmd, client, owner, ownerType, packageName,
					tagPattern, onlyTagged, onlyUntagged, olderThan, newerThan,
					olderThanDays, newerThanDays, skipConfirm, dryRun)
			}

			// Single deletion mode
			return runSingleDeleteVersion(ctx, cmd, client, owner, ownerType, packageName,
				versionID, digest, tag, skipConfirm, dryRun)
		},
	}

	// Single version selectors
	cmd.Flags().Int64Var(&versionID, "version", 0, "Delete version by ID")
	cmd.Flags().StringVar(&digest, "digest", "", "Delete version by digest")
	cmd.Flags().StringVar(&tag, "tag", "", "Delete version by tag")

	// Filter flags for bulk deletion
	cmd.Flags().StringVar(&tagPattern, "tag-pattern", "", "Delete versions matching regex pattern")
	cmd.Flags().BoolVar(&onlyTagged, "tagged", false, "Delete only tagged versions")
	cmd.Flags().BoolVar(&onlyUntagged, "untagged", false, "Delete only untagged versions")
	cmd.Flags().StringVar(&olderThan, "older-than", "", "Delete versions older than date (YYYY-MM-DD or RFC3339)")
	cmd.Flags().StringVar(&newerThan, "newer-than", "", "Delete versions newer than date (YYYY-MM-DD or RFC3339)")
	cmd.Flags().IntVar(&olderThanDays, "older-than-days", 0, "Delete versions older than N days")
	cmd.Flags().IntVar(&newerThanDays, "newer-than-days", 0, "Delete versions newer than N days")

	// Common flags
	cmd.Flags().BoolVar(&force, "force", false, "Skip confirmation prompt")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip confirmation prompt (alias for --force)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be deleted without deleting")

	// Mark single selectors as mutually exclusive
	cmd.MarkFlagsMutuallyExclusive("version", "digest", "tag")
	cmd.MarkFlagsMutuallyExclusive("tagged", "untagged")

	return cmd
}

// newDeleteImageCmd creates the delete image subcommand with isolated flag state.
func newDeleteImageCmd() *cobra.Command {
	var (
		force     bool
		yes       bool
		dryRun    bool
		tag       string
		digest    string
		versionID int64
	)

	cmd := &cobra.Command{
		Use:   "image <owner/package>",
		Short: "Delete an entire OCI image",
		Long: `Delete an entire OCI image from GitHub Container Registry.

This command discovers and deletes all versions that make up an OCI image,
including the root index, platform manifests, and attestations (SBOM, provenance).

Requires a selector: --tag, --digest, or --version.

IMPORTANT: Deletion is permanent and cannot be undone (except within 30 days
via the GitHub web UI if the package namespace is available).

Examples:
  # Delete image by tag (most common)
  ghcrctl delete image mkoepf/myimage --tag v1.0.0

  # Delete image by digest
  ghcrctl delete image mkoepf/myimage --digest sha256:abc123...

  # Delete image containing a specific version
  ghcrctl delete image mkoepf/myimage --version 12345678

  # Skip confirmation
  ghcrctl delete image mkoepf/myimage --tag v1.0.0 --force

  # Preview what would be deleted
  ghcrctl delete image mkoepf/myimage --tag v1.0.0 --dry-run`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Parse owner/package reference (reject inline tags)
			owner, packageName, err := parsePackageRef(args[0])
			if err != nil {
				cmd.SilenceUsage = true
				return err
			}

			// Require at least one selector
			if tag == "" && digest == "" && versionID == 0 {
				cmd.SilenceUsage = true
				return fmt.Errorf("selector required: use --tag, --digest, or --version")
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

			fullImage := fmt.Sprintf("ghcr.io/%s/%s", owner, packageName)

			// Determine the root digest based on which selector was used
			var rootDigest string

			if tag != "" {
				// Resolve tag to digest
				rootDigest, err = discover.ResolveTag(ctx, fullImage, tag)
				if err != nil {
					cmd.SilenceUsage = true
					return fmt.Errorf("failed to resolve tag '%s': %w", tag, err)
				}
			} else if digest != "" {
				// Lookup by digest - support both full and short (prefix) format
				digestInput := digest
				if !strings.HasPrefix(digestInput, "sha256:") {
					digestInput = "sha256:" + digestInput
				}

				// If it looks like a short digest, resolve to full digest
				if len(digestInput) < 71 { // sha256: (7) + 64 hex chars = 71
					allVersions, err := ghClient.ListPackageVersions(ctx, owner, ownerType, packageName)
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
				allVersions, err := ghClient.ListPackageVersions(ctx, owner, ownerType, packageName)
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
			}

			// Use discover package to get all versions and their relationships
			allVersions, err := ghClient.ListPackageVersions(ctx, owner, ownerType, packageName)
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
			toDelete, shared := discover.ClassifyImageVersions(imageVersions)

			// Extract version IDs from toDelete list
			versionIDs := make([]int64, len(toDelete))
			for i, v := range toDelete {
				versionIDs[i] = v.ID
			}

			// Display what will be deleted
			fmt.Fprintf(cmd.OutOrStdout(), "Preparing to delete complete OCI image:\n")
			fmt.Fprintf(cmd.OutOrStdout(), "  Package: %s\n", packageName)
			if tag != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "  Tag:     %s\n", tag)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\n")
			displayDeleteImageVersions(cmd.OutOrStdout(), toDelete, shared, imageVersions)
			fmt.Fprintf(cmd.OutOrStdout(), "\nTotal: %s version(s) will be deleted\n\n",
				display.ColorWarning(fmt.Sprintf("%d", len(versionIDs))))

			// Handle dry-run
			if dryRun {
				fmt.Fprintln(cmd.OutOrStdout(), display.ColorDryRun("DRY RUN: No changes made"))
				return nil
			}

			// Confirm deletion unless --force or --yes is used
			skipConfirm := force || yes
			if !skipConfirm {
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
			err = deleteVersionsInOrder(ctx, ghClient, owner, ownerType, packageName, versionIDs, cmd.OutOrStdout())
			if err != nil {
				cmd.SilenceUsage = true
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "\n%s\n",
				display.ColorSuccess(fmt.Sprintf("Successfully deleted %d version(s) of %s", len(versionIDs), packageName)))
			return nil
		},
	}

	// Selector flags
	cmd.Flags().StringVar(&tag, "tag", "", "Delete image by tag")
	cmd.Flags().StringVar(&digest, "digest", "", "Delete image by digest")
	cmd.Flags().Int64Var(&versionID, "version", 0, "Delete image containing this version ID")

	// Common flags
	cmd.Flags().BoolVar(&force, "force", false, "Skip confirmation prompt")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip confirmation prompt (alias for --force)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be deleted without deleting")

	cmd.MarkFlagsMutuallyExclusive("tag", "digest", "version")

	return cmd
}

// newDeletePackageCmd creates the delete package subcommand with isolated flag state.
func newDeletePackageCmd() *cobra.Command {
	var (
		force bool
		yes   bool
	)

	cmd := &cobra.Command{
		Use:   "package <owner/package>",
		Short: "Delete an entire package",
		Long: `Delete an entire package from GitHub Container Registry.

This command deletes the package and ALL its versions permanently.
Use this when you cannot delete the last tagged version of a package.

IMPORTANT: Deletion is permanent and cannot be undone (except within 30 days
via the GitHub web UI if the package namespace is available).

Examples:
  # Delete a package
  ghcrctl delete package mkoepf/myimage

  # Delete without confirmation
  ghcrctl delete package mkoepf/myimage --force`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Parse owner/package reference
			owner, packageName, err := parsePackageRef(args[0])
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

			// Fetch version count to show the user what they're deleting
			versions, err := client.ListPackageVersions(ctx, owner, ownerType, packageName)
			if err != nil {
				cmd.SilenceUsage = true
				return fmt.Errorf("failed to list package versions: %w", err)
			}

			// Count tagged vs untagged
			taggedCount := 0
			for _, v := range versions {
				if len(v.Tags) > 0 {
					taggedCount++
				}
			}
			untaggedCount := len(versions) - taggedCount

			// Show what will be deleted
			fmt.Fprintf(cmd.OutOrStdout(), "Preparing to delete entire package:\n")
			fmt.Fprintf(cmd.OutOrStdout(), "  Package:  %s/%s\n", owner, packageName)
			fmt.Fprintf(cmd.OutOrStdout(), "  Owner:    %s (%s)\n", owner, ownerType)
			fmt.Fprintf(cmd.OutOrStdout(), "  Versions: %s (%d tagged, %d untagged)\n\n",
				display.ColorWarning(fmt.Sprintf("%d total", len(versions))), taggedCount, untaggedCount)
			fmt.Fprintf(cmd.OutOrStdout(), "%s\n\n", display.ColorError("WARNING: This will permanently delete ALL versions of this package!"))

			// Confirm deletion unless --force or --yes is used
			skipConfirm := force || yes
			if !skipConfirm {
				confirmed, err := prompts.ConfirmWithInput(os.Stdin, cmd.OutOrStdout(),
					"To confirm, type the package name", packageName)
				if err != nil {
					return fmt.Errorf("failed to read confirmation: %w", err)
				}

				if !confirmed {
					fmt.Fprintln(cmd.OutOrStdout(), "Deletion cancelled (input did not match package name)")
					return nil
				}
			}

			// Perform deletion
			err = client.DeletePackage(ctx, owner, ownerType, packageName)
			if err != nil {
				cmd.SilenceUsage = true
				return fmt.Errorf("failed to delete package: %w", err)
			}

			fmt.Fprintln(cmd.OutOrStdout(), display.ColorSuccess(fmt.Sprintf("Successfully deleted package %s/%s", owner, packageName)))
			return nil
		},
	}

	// Common flags
	cmd.Flags().BoolVar(&force, "force", false, "Skip confirmation prompt")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip confirmation prompt (alias for --force)")

	return cmd
}

// runSingleDeleteVersion handles deletion of a single version
func runSingleDeleteVersion(ctx context.Context, cmd *cobra.Command, client *gh.Client, owner, ownerType, packageName string,
	versionID int64, digest, tag string, force, dryRun bool) error {

	var targetVersionID int64
	var err error

	if versionID != 0 {
		targetVersionID = versionID
	} else if digest != "" {
		// Normalize digest format
		if !strings.HasPrefix(digest, "sha256:") {
			digest = "sha256:" + digest
		}
		// Look up version ID by digest
		targetVersionID, err = client.GetVersionIDByDigest(ctx, owner, ownerType, packageName, digest)
		if err != nil {
			cmd.SilenceUsage = true
			return fmt.Errorf("failed to find version by digest: %w", err)
		}
	} else if tag != "" {
		// Resolve tag to digest first, then get version ID
		fullImage := fmt.Sprintf("ghcr.io/%s/%s", owner, packageName)
		resolvedDigest, err := discover.ResolveTag(ctx, fullImage, tag)
		if err != nil {
			cmd.SilenceUsage = true
			return fmt.Errorf("failed to resolve tag '%s': %w", tag, err)
		}
		targetVersionID, err = client.GetVersionIDByDigest(ctx, owner, ownerType, packageName, resolvedDigest)
		if err != nil {
			cmd.SilenceUsage = true
			return fmt.Errorf("failed to find version for tag '%s': %w", tag, err)
		}
	}

	// Get version tags to show what we're deleting
	tags, err := client.GetVersionTags(ctx, owner, ownerType, packageName, targetVersionID)
	if err != nil {
		cmd.SilenceUsage = true
		return fmt.Errorf("failed to get version details: %w", err)
	}

	// Count how many other versions reference this one
	imageCount := countIncomingRefs(ctx, client, owner, ownerType, packageName, targetVersionID)

	// Show what will be deleted
	fmt.Fprintf(cmd.OutOrStdout(), "Preparing to delete package version:\n")
	fmt.Fprintf(cmd.OutOrStdout(), "  Package:    %s\n", packageName)
	fmt.Fprintf(cmd.OutOrStdout(), "  Owner:      %s (%s)\n", owner, ownerType)
	fmt.Fprintf(cmd.OutOrStdout(), "  Version ID: %d\n", targetVersionID)
	fmt.Fprintf(cmd.OutOrStdout(), "  Tags:       %s\n", formatTagsForDisplay(tags))
	if imageCount > 0 {
		versionWord := "version"
		if imageCount > 1 {
			versionWord = "versions"
		}
		fmt.Fprintf(cmd.OutOrStdout(), "  Referenced: %s\n", display.ColorShared(fmt.Sprintf("by %d other %s", imageCount, versionWord)))
	}
	fmt.Fprintln(cmd.OutOrStdout())

	// Handle dry-run
	if dryRun {
		fmt.Fprintln(cmd.OutOrStdout(), display.ColorDryRun("DRY RUN: No changes made"))
		return nil
	}

	// Confirm deletion unless --force is used
	if !force {
		confirmed, err := prompts.Confirm(os.Stdin, cmd.OutOrStdout(), display.ColorWarning("Are you sure you want to delete this version?"))
		if err != nil {
			return fmt.Errorf("failed to read confirmation: %w", err)
		}

		if !confirmed {
			fmt.Fprintln(cmd.OutOrStdout(), "Deletion cancelled")
			return nil
		}
	}

	// Perform deletion
	err = client.DeletePackageVersion(ctx, owner, ownerType, packageName, targetVersionID)
	if err != nil {
		cmd.SilenceUsage = true
		if gh.IsLastTaggedVersionError(err) {
			fmt.Fprintf(cmd.OutOrStdout(), "\n%s\n", display.ColorWarning("GHCR does not allow to delete the last tagged version of a package."))
			fmt.Fprintf(cmd.OutOrStdout(), "You can delete the package instead:\n")
			fmt.Fprintf(cmd.OutOrStdout(), "  ghcrctl delete package %s/%s\n", owner, packageName)
			return fmt.Errorf("GHCR does not allow to delete the last tagged version of a package. You can delete the package instead.")
		}
		return fmt.Errorf("failed to delete package version: %w", err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), display.ColorSuccess(fmt.Sprintf("Successfully deleted version %d of %s", targetVersionID, packageName)))
	return nil
}

// runBulkDeleteVersion handles deletion of multiple versions using filters
func runBulkDeleteVersion(ctx context.Context, cmd *cobra.Command, client *gh.Client, owner, ownerType, packageName string,
	tagPattern string, onlyTagged, onlyUntagged bool, olderThan, newerThan string,
	olderThanDays, newerThanDays int, force, dryRun bool) error {

	// Build filter from flags
	versionFilter, err := buildDeleteVersionFilter(tagPattern, onlyTagged, onlyUntagged, olderThan, newerThan, olderThanDays, newerThanDays)
	if err != nil {
		cmd.SilenceUsage = true
		return fmt.Errorf("invalid filter options: %w", err)
	}

	// List all package versions
	allVersions, err := client.ListPackageVersions(ctx, owner, ownerType, packageName)
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
	fullImage := fmt.Sprintf("ghcr.io/%s/%s", owner, packageName)
	discoverer := discover.NewPackageDiscoverer()
	versions, err := discoverer.DiscoverPackage(ctx, fullImage, allVersions, nil)

	// Track which version IDs are shared (have incoming refs from outside deletion set)
	sharedChildren := make(map[int64]bool)
	if err == nil {
		// Build set of version IDs being deleted
		deletingIDs := make(map[int64]bool)
		for _, ver := range matchingVersions {
			deletingIDs[ver.ID] = true
		}

		// Build digest to ID map for checking incoming refs
		digestToID := make(map[string]int64)
		for _, ver := range versions {
			digestToID[ver.Digest] = ver.ID
		}

		// A version is shared if it has incoming refs from versions NOT being deleted
		for _, ver := range versions {
			for _, inRef := range ver.IncomingRefs {
				if refID, ok := digestToID[inRef]; ok {
					if !deletingIDs[refID] {
						// This version has a ref from something not being deleted
						sharedChildren[ver.ID] = true
						break
					}
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
		fmt.Fprintf(cmd.OutOrStdout(), "%s %d version(s) are shared by multiple images and will be preserved.\n\n",
			display.ColorWarning("Note:"), len(protectedVersions))
	}

	matchingVersions = safeToDelete

	// Re-check if any versions remain after filtering
	if len(matchingVersions) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No versions to delete (all matching versions are shared with other images)")
		return nil
	}

	// Display summary of what will be deleted
	fmt.Fprintf(cmd.OutOrStdout(), "Preparing to delete %s package version(s):\n",
		display.ColorWarning(fmt.Sprintf("%d", len(matchingVersions))))
	fmt.Fprintf(cmd.OutOrStdout(), "  Package: %s\n", packageName)
	fmt.Fprintf(cmd.OutOrStdout(), "  Owner:   %s (%s)\n\n", owner, ownerType)

	// Show details of matching versions (limit to first 10 for readability)
	displayLimit := 10
	for i, ver := range matchingVersions {
		if i >= displayLimit {
			fmt.Fprintf(cmd.OutOrStdout(), "  ... and %d more\n", len(matchingVersions)-displayLimit)
			break
		}

		tagsStr := formatTagsForDisplay(ver.Tags)
		fmt.Fprintf(cmd.OutOrStdout(), "  - ID: %d, Tags: %s, Created: %s\n", ver.ID, tagsStr, ver.CreatedAt)
	}
	fmt.Fprintln(cmd.OutOrStdout())

	// Handle dry-run
	if dryRun {
		fmt.Fprintln(cmd.OutOrStdout(), display.ColorDryRun("DRY RUN: No changes made"))
		return nil
	}

	// Confirm deletion unless --force is used
	if !force {
		confirmed, err := prompts.Confirm(os.Stdin, cmd.OutOrStdout(),
			display.ColorWarning(fmt.Sprintf("Are you sure you want to delete %d version(s)?", len(matchingVersions))))
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
	lastTaggedHit := false
	for i, ver := range matchingVersions {
		fmt.Fprintf(cmd.OutOrStdout(), "Deleting version %d/%d (ID: %d)...\n", i+1, len(matchingVersions), ver.ID)
		err := client.DeletePackageVersion(ctx, owner, ownerType, packageName, ver.ID)
		if err != nil {
			if gh.IsLastTaggedVersionError(err) {
				lastTaggedHit = true
			}
			fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", display.ColorError(fmt.Sprintf("Failed: %v", err)))
			failCount++
		} else {
			successCount++
		}
	}

	// Summary
	fmt.Fprintln(cmd.OutOrStdout())
	if failCount > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "Deletion complete: %s succeeded, %s failed\n",
			display.ColorSuccess(fmt.Sprintf("%d", successCount)),
			display.ColorError(fmt.Sprintf("%d", failCount)))
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "Deletion complete: %s succeeded\n",
			display.ColorSuccess(fmt.Sprintf("%d", successCount)))
	}

	if lastTaggedHit {
		fmt.Fprintf(cmd.OutOrStdout(), "\n%s\n", display.ColorWarning("Note: GHCR does not allow to delete the last tagged version of a package."))
		fmt.Fprintf(cmd.OutOrStdout(), "You can delete the package instead:\n")
		fmt.Fprintf(cmd.OutOrStdout(), "  ghcrctl delete package %s/%s\n", owner, packageName)
	}

	if failCount > 0 {
		return fmt.Errorf("failed to delete %d version(s)", failCount)
	}

	return nil
}

// buildDeleteVersionFilter creates a VersionFilter from command-line flags
func buildDeleteVersionFilter(tagPattern string, onlyTagged, onlyUntagged bool,
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
		t, err := filter.ParseDate(olderThan)
		if err != nil {
			return nil, fmt.Errorf("invalid --older-than date format (expected YYYY-MM-DD or RFC3339): %w", err)
		}
		vf.OlderThan = t
	}

	// Parse newer-than date
	if newerThan != "" {
		t, err := filter.ParseDate(newerThan)
		if err != nil {
			return nil, fmt.Errorf("invalid --newer-than date format (expected YYYY-MM-DD or RFC3339): %w", err)
		}
		vf.NewerThan = t
	}

	return vf, nil
}

// displayDeleteImageVersions displays versions to delete and shared versions.
func displayDeleteImageVersions(w io.Writer, toDelete, shared, imageVersions []discover.VersionInfo) {
	// Build set of digests in this image for counting external refs
	imageDigests := make(map[string]bool)
	for _, v := range imageVersions {
		imageDigests[v.Digest] = true
	}

	// Show what will be deleted
	if len(toDelete) > 0 {
		fmt.Fprintf(w, "Versions to delete (%d):\n", len(toDelete))
		for _, v := range toDelete {
			fmt.Fprintf(w, "  - %s (version %d)%s\n", formatVersionType(v.Types), v.ID, formatVersionTags(v.Tags))
		}
	}

	// Show what will be preserved (shared with other images)
	if len(shared) > 0 {
		fmt.Fprintf(w, "\n%s\n", display.ColorWarning("Shared versions (preserved):"))
		for _, v := range shared {
			// Count external references
			externalRefs := 0
			for _, inRef := range v.IncomingRefs {
				if !imageDigests[inRef] {
					externalRefs++
				}
			}
			fmt.Fprintf(w, "  - %s (version %d, referenced by %d versions outside this delete)%s\n",
				formatVersionType(v.Types), v.ID, externalRefs, formatVersionTags(v.Tags))
		}
	}
}

// formatVersionType returns the first type or "unknown" if none.
func formatVersionType(types []string) string {
	if len(types) > 0 {
		return types[0]
	}
	return "unknown"
}

// formatVersionTags returns formatted tags or empty string if none.
func formatVersionTags(tags []string) string {
	if len(tags) > 0 {
		return fmt.Sprintf(" [%s]", strings.Join(tags, ", "))
	}
	return ""
}

// formatTagsForDisplay formats a tag slice for display
func formatTagsForDisplay(tags []string) string {
	if len(tags) == 0 {
		return "[]"
	}
	return strings.Join(tags, ", ")
}

// =============================================================================
// Exported types and functions for testing
// =============================================================================

// PackageDeleter interface for mocking in tests
type PackageDeleter interface {
	DeletePackageVersion(ctx context.Context, owner, ownerType, packageName string, versionID int64) error
}

// DeleteVersionParams contains parameters for single version deletion
type DeleteVersionParams struct {
	Owner      string
	OwnerType  string
	ImageName  string
	VersionID  int64
	Tags       []string
	ImageCount int
	Force      bool
	DryRun     bool
}

// BulkDeleteParams contains parameters for bulk version deletion
type BulkDeleteParams struct {
	Owner     string
	OwnerType string
	ImageName string
	Versions  []gh.PackageVersionInfo
	Force     bool
	DryRun    bool
}

// DeleteGraphWithDeleter deletes versions using a deleter interface (for testing)
func DeleteGraphWithDeleter(ctx context.Context, deleter PackageDeleter, owner, ownerType, packageName string, versionIDs []int64, w io.Writer) error {
	for i, versionID := range versionIDs {
		fmt.Fprintf(w, "Deleting version %d/%d (ID: %d)...\n", i+1, len(versionIDs), versionID)
		err := deleter.DeletePackageVersion(ctx, owner, ownerType, packageName, versionID)
		if err != nil {
			return fmt.Errorf("failed to delete version %d: %w", versionID, err)
		}
	}
	return nil
}

// ExecuteSingleDelete executes a single version deletion with confirmation (for testing)
func ExecuteSingleDelete(ctx context.Context, deleter PackageDeleter, params DeleteVersionParams, w io.Writer, confirmFn func() (bool, error)) error {
	// Show what will be deleted
	fmt.Fprintf(w, "Preparing to delete package version:\n")
	fmt.Fprintf(w, "  Image:      %s\n", params.ImageName)
	fmt.Fprintf(w, "  Owner:      %s (%s)\n", params.Owner, params.OwnerType)
	fmt.Fprintf(w, "  Version ID: %d\n", params.VersionID)
	fmt.Fprintf(w, "  Tags:       %s\n", formatTagsForDisplay(params.Tags))
	if params.ImageCount > 0 {
		versionWord := "version"
		if params.ImageCount > 1 {
			versionWord = "versions"
		}
		fmt.Fprintf(w, "  Referenced: %s\n", display.ColorShared(fmt.Sprintf("by %d other %s", params.ImageCount, versionWord)))
	}
	fmt.Fprintln(w)

	// Handle dry-run
	if params.DryRun {
		fmt.Fprintln(w, display.ColorDryRun("DRY RUN: No changes made"))
		return nil
	}

	// Confirm deletion unless --force is used
	if !params.Force {
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
	err := deleter.DeletePackageVersion(ctx, params.Owner, params.OwnerType, params.ImageName, params.VersionID)
	if err != nil {
		if gh.IsLastTaggedVersionError(err) {
			fmt.Fprintf(w, "\n%s\n", display.ColorWarning("GHCR does not allow to delete the last tagged version of a package."))
			fmt.Fprintf(w, "You can delete the package instead:\n")
			fmt.Fprintf(w, "  ghcrctl delete package %s/%s\n", params.Owner, params.ImageName)
			return fmt.Errorf("GHCR does not allow to delete the last tagged version of a package. You can delete the package instead.")
		}
		return fmt.Errorf("failed to delete package version: %w", err)
	}

	fmt.Fprintln(w, display.ColorSuccess(fmt.Sprintf("Successfully deleted version %d of %s", params.VersionID, params.ImageName)))
	return nil
}

// ExecuteBulkDelete executes bulk version deletion with confirmation (for testing)
func ExecuteBulkDelete(ctx context.Context, deleter PackageDeleter, params BulkDeleteParams, w io.Writer, confirmFn func(count int) (bool, error)) error {
	// Display summary of what will be deleted
	fmt.Fprintf(w, "Preparing to delete %s package version(s):\n",
		display.ColorWarning(fmt.Sprintf("%d", len(params.Versions))))
	fmt.Fprintf(w, "  Image: %s\n", params.ImageName)
	fmt.Fprintf(w, "  Owner: %s (%s)\n\n", params.Owner, params.OwnerType)

	// Show details of matching versions (limit to first 10 for readability)
	displayLimit := 10
	for i, ver := range params.Versions {
		if i >= displayLimit {
			fmt.Fprintf(w, "  ... and %d more\n", len(params.Versions)-displayLimit)
			break
		}
		tagsStr := formatTagsForDisplay(ver.Tags)
		fmt.Fprintf(w, "  - ID: %d, Tags: %s, Created: %s\n", ver.ID, tagsStr, ver.CreatedAt)
	}
	fmt.Fprintln(w)

	// Handle dry-run
	if params.DryRun {
		fmt.Fprintln(w, display.ColorDryRun("DRY RUN: No changes made"))
		return nil
	}

	// Confirm deletion unless --force is used
	if !params.Force {
		confirmed, err := confirmFn(len(params.Versions))
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
	for i, ver := range params.Versions {
		fmt.Fprintf(w, "Deleting version %d/%d (ID: %d)...\n", i+1, len(params.Versions), ver.ID)
		err := deleter.DeletePackageVersion(ctx, params.Owner, params.OwnerType, params.ImageName, ver.ID)
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

// deleteVersionsInOrder deletes versions in the correct order
func deleteVersionsInOrder(ctx context.Context, client *gh.Client, owner, ownerType, packageName string, versionIDs []int64, w io.Writer) error {
	for i, versionID := range versionIDs {
		fmt.Fprintf(w, "Deleting version %d/%d (ID: %d)...\n", i+1, len(versionIDs), versionID)
		err := client.DeletePackageVersion(ctx, owner, ownerType, packageName, versionID)
		if err != nil {
			return fmt.Errorf("failed to delete version %d: %w", versionID, err)
		}
	}
	return nil
}

// countIncomingRefs returns how many other versions reference the given version ID.
func countIncomingRefs(ctx context.Context, client *gh.Client, owner, ownerType, packageName string, versionID int64) int {
	// Get all versions for this package
	allVersions, err := client.ListPackageVersions(ctx, owner, ownerType, packageName)
	if err != nil {
		return 0
	}

	fullImage := fmt.Sprintf("ghcr.io/%s/%s", owner, packageName)

	// Use discover package to get version relationships
	discoverer := discover.NewPackageDiscoverer()
	versions, err := discoverer.DiscoverPackage(ctx, fullImage, allVersions, nil)
	if err != nil {
		return 0
	}

	// Find the version by ID and return its incoming ref count
	for _, v := range versions {
		if v.ID == versionID {
			return len(v.IncomingRefs)
		}
	}
	return 0
}
