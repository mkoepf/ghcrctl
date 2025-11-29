package cmd

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/mkoepf/ghcrctl/internal/display"
	"github.com/mkoepf/ghcrctl/internal/filter"
	"github.com/mkoepf/ghcrctl/internal/gh"
	"github.com/spf13/cobra"
)

// newVersionsCmd creates the versions command with isolated flag state.
func newVersionsCmd() *cobra.Command {
	var (
		jsonOutput    bool
		tag           string
		tagPattern    string
		onlyTagged    bool
		onlyUntagged  bool
		olderThan     string
		newerThan     string
		olderThanDays int
		newerThanDays int
		outputFormat  string
		versionID     int64
		digest        string
	)

	cmd := &cobra.Command{
		Use:   "versions <owner/package>",
		Short: "List all versions of a package",
		Long: `List all versions of a container package.

This command displays all versions of a package with their version ID, digest,
tags, and creation date. The version ID can be used with the delete command.

To see artifact relationships (platform manifests, attestations, signatures),
use the 'ghcrctl images' command instead.

Examples:
  # List all versions
  ghcrctl versions mkoepf/myimage

  # List versions for a specific tag
  ghcrctl versions mkoepf/myimage --tag v1.0

  # List only tagged versions
  ghcrctl versions mkoepf/myimage --tagged

  # List only untagged versions
  ghcrctl versions mkoepf/myimage --untagged

  # List versions matching a tag pattern (regex)
  ghcrctl versions mkoepf/myimage --tag-pattern "^v1\\..*"

  # List versions older than a specific date
  ghcrctl versions mkoepf/myimage --older-than 2025-01-01

  # List versions newer than a specific date
  ghcrctl versions mkoepf/myimage --newer-than 2025-11-01

  # List versions older than 30 days
  ghcrctl versions mkoepf/myimage --older-than-days 30

  # Combine filters: untagged versions older than 7 days
  ghcrctl versions mkoepf/myimage --untagged --older-than-days 7

  # Filter by specific version ID
  ghcrctl versions mkoepf/myimage --version 12345678

  # Filter by digest (supports prefix matching)
  ghcrctl versions mkoepf/myimage --digest sha256:abc123

  # List versions in JSON format
  ghcrctl versions mkoepf/myimage --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Parse owner/image reference
			owner, imageName, _, err := parseImageRef(args[0])
			if err != nil {
				cmd.SilenceUsage = true
				return err
			}

			// Handle output format flag (-o)
			if outputFormat != "" {
				switch outputFormat {
				case "json":
					jsonOutput = true
				case "table":
					jsonOutput = false
				default:
					cmd.SilenceUsage = true
					return fmt.Errorf("invalid output format %q. Supported formats: json, table", outputFormat)
				}
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

			// List package versions
			allVersions, err := client.ListPackageVersions(ctx, owner, ownerType, imageName)
			if err != nil {
				cmd.SilenceUsage = true
				return fmt.Errorf("failed to list versions: %w", err)
			}

			// Build filter from command-line flags
			versionFilter, err := buildVersionFilter(tag, tagPattern, onlyTagged, onlyUntagged,
				olderThan, newerThan, olderThanDays, newerThanDays, versionID, digest)
			if err != nil {
				cmd.SilenceUsage = true
				return fmt.Errorf("invalid filter options: %w", err)
			}

			// Apply filters to determine which versions to display
			filteredVersions := versionFilter.Apply(allVersions)
			if len(filteredVersions) == 0 {
				cmd.SilenceUsage = true
				return fmt.Errorf("no versions found matching filter criteria")
			}

			// JSON output
			if jsonOutput {
				return display.OutputJSON(cmd.OutOrStdout(), filteredVersions)
			}

			// Table output (default)
			return outputVersionsTable(cmd.OutOrStdout(), filteredVersions, imageName)
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output in JSON format")
	cmd.Flags().StringVar(&tag, "tag", "", "Filter versions by exact tag match")
	cmd.Flags().StringVar(&tagPattern, "tag-pattern", "", "Filter versions by tag regex pattern")
	cmd.Flags().BoolVar(&onlyTagged, "tagged", false, "Show only tagged versions")
	cmd.Flags().BoolVar(&onlyUntagged, "untagged", false, "Show only untagged versions")
	cmd.Flags().StringVar(&olderThan, "older-than", "", "Show versions older than date (YYYY-MM-DD or RFC3339)")
	cmd.Flags().StringVar(&newerThan, "newer-than", "", "Show versions newer than date (YYYY-MM-DD or RFC3339)")
	cmd.Flags().IntVar(&olderThanDays, "older-than-days", 0, "Show versions older than N days")
	cmd.Flags().IntVar(&newerThanDays, "newer-than-days", 0, "Show versions newer than N days")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "", "Output format (json, table)")
	cmd.Flags().Int64Var(&versionID, "version", 0, "Filter by exact version ID")
	cmd.Flags().StringVar(&digest, "digest", "", "Filter by digest (supports prefix matching)")

	// Enable dynamic completion for image reference
	cmd.ValidArgsFunction = imageRefValidArgsFunc

	return cmd
}

// outputVersionsTable outputs a flat list of versions
func outputVersionsTable(w io.Writer, versions []gh.PackageVersionInfo, imageName string) error {
	if len(versions) == 0 {
		fmt.Fprintf(w, "No versions found for %s\n", imageName)
		return nil
	}

	fmt.Fprintf(w, "Versions for %s:\n\n", imageName)

	// Find column widths
	maxIDLen := len("VERSION ID")
	maxDigestLen := len("DIGEST")
	maxTagsLen := len("TAGS")

	for _, ver := range versions {
		if idLen := len(fmt.Sprintf("%d", ver.ID)); idLen > maxIDLen {
			maxIDLen = idLen
		}
		digest := display.ShortDigest(ver.Name)
		if len(digest) > maxDigestLen {
			maxDigestLen = len(digest)
		}
		if tagsStr := display.FormatTags(ver.Tags); len(tagsStr) > maxTagsLen {
			maxTagsLen = len(tagsStr)
		}
	}

	// Print header
	fmt.Fprintf(w, "  %s  %s  %s  %s\n",
		display.ColorHeader(fmt.Sprintf("%-*s", maxIDLen, "VERSION ID")),
		display.ColorHeader(fmt.Sprintf("%-*s", maxDigestLen, "DIGEST")),
		display.ColorHeader(fmt.Sprintf("%-*s", maxTagsLen, "TAGS")),
		display.ColorHeader("CREATED"))
	fmt.Fprintf(w, "  %s  %s  %s  %s\n",
		display.ColorSeparator(strings.Repeat("-", maxIDLen)),
		display.ColorSeparator(strings.Repeat("-", maxDigestLen)),
		display.ColorSeparator(strings.Repeat("-", maxTagsLen)),
		display.ColorSeparator(strings.Repeat("-", len("CREATED"))))

	// Print versions
	for _, ver := range versions {
		tagsStr := display.FormatTags(ver.Tags)
		digest := display.ShortDigest(ver.Name)

		fmt.Fprintf(w, "  %-*d  %s  %s%s  %s\n",
			maxIDLen, ver.ID,
			display.ColorDigest(fmt.Sprintf("%-*s", maxDigestLen, digest)),
			display.ColorTags(ver.Tags),
			strings.Repeat(" ", maxTagsLen-len(tagsStr)),
			ver.CreatedAt)
	}

	// Summary
	versionWord := "versions"
	if len(versions) == 1 {
		versionWord = "version"
	}
	fmt.Fprintf(w, "\nTotal: %s %s.\n", display.ColorCount(len(versions)), versionWord)

	return nil
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
func buildVersionFilter(tag, tagPattern string, onlyTagged, onlyUntagged bool,
	olderThan, newerThan string, olderThanDays, newerThanDays int,
	versionID int64, digest string) (*filter.VersionFilter, error) {
	vf := &filter.VersionFilter{
		OnlyTagged:    onlyTagged,
		OnlyUntagged:  onlyUntagged,
		TagPattern:    tagPattern,
		OlderThanDays: olderThanDays,
		NewerThanDays: newerThanDays,
		VersionID:     versionID,
		Digest:        digest,
	}

	// Handle exact tag match (backward compatibility with --tag flag)
	if tag != "" {
		vf.Tags = []string{tag}
	}

	// Parse absolute date filters
	if olderThan != "" {
		t, err := parseUserDate(olderThan)
		if err != nil {
			return nil, fmt.Errorf("invalid --older-than date format: %w", err)
		}
		vf.OlderThan = t
	}

	if newerThan != "" {
		t, err := parseUserDate(newerThan)
		if err != nil {
			return nil, fmt.Errorf("invalid --newer-than date format: %w", err)
		}
		vf.NewerThan = t
	}

	// Validate conflicting flags
	if onlyTagged && onlyUntagged {
		return nil, fmt.Errorf("cannot use --tagged and --untagged together")
	}

	return vf, nil
}
