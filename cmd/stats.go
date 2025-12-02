package cmd

import (
	"context"
	"fmt"
	"io"

	"github.com/mkoepf/ghcrctl/internal/display"
	"github.com/mkoepf/ghcrctl/internal/gh"
	"github.com/mkoepf/ghcrctl/internal/quiet"
	"github.com/spf13/cobra"
)

// PackageStats holds computed statistics for a package
type PackageStats struct {
	PackageName      string `json:"package_name"`
	TotalVersions    int    `json:"total_versions"`
	TaggedVersions   int    `json:"tagged_versions"`
	UntaggedVersions int    `json:"untagged_versions"`
	TotalTags        int    `json:"total_tags"`
	OldestVersion    string `json:"oldest_version,omitempty"`
	NewestVersion    string `json:"newest_version,omitempty"`
}

func newStatsCmd() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "stats <owner/package>",
		Short: "Show statistics for a package",
		Long: `Display statistics for a container package including version counts and dates.

Examples:
  # Show statistics for a package
  ghcrctl stats mkoepf/myimage

  # Output as JSON
  ghcrctl stats mkoepf/myimage --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			owner, packageName, err := parsePackageRef(args[0])
			if err != nil {
				cmd.SilenceUsage = true
				return err
			}

			token, err := gh.GetToken()
			if err != nil {
				cmd.SilenceUsage = true
				return err
			}

			ghClient, err := gh.NewClientWithContext(cmd.Context(), token)
			if err != nil {
				cmd.SilenceUsage = true
				return err
			}

			// Get owner type
			ownerType, err := ghClient.GetOwnerType(cmd.Context(), owner)
			if err != nil {
				cmd.SilenceUsage = true
				return fmt.Errorf("failed to determine owner type: %w", err)
			}

			// List all versions
			versions, err := ghClient.ListPackageVersions(cmd.Context(), owner, ownerType, packageName)
			if err != nil {
				cmd.SilenceUsage = true
				return fmt.Errorf("failed to list versions: %w", err)
			}

			// Calculate statistics
			stats := CalculateStats(versions)
			stats.PackageName = packageName

			// Output
			if jsonOutput {
				return display.OutputJSON(cmd.OutOrStdout(), stats)
			}

			return OutputStatsTable(cmd.OutOrStdout(), stats, quiet.IsQuiet(cmd.Context()))
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output in JSON format")

	cmd.ValidArgsFunction = imageRefValidArgsFunc

	return cmd
}

// CalculateStats computes statistics from a list of package versions
func CalculateStats(versions []gh.PackageVersionInfo) PackageStats {
	stats := PackageStats{
		TotalVersions: len(versions),
	}

	if len(versions) == 0 {
		return stats
	}

	oldest := versions[0].CreatedAt
	newest := versions[0].CreatedAt

	for _, v := range versions {
		if len(v.Tags) > 0 {
			stats.TaggedVersions++
			stats.TotalTags += len(v.Tags)
		} else {
			stats.UntaggedVersions++
		}

		// Track oldest/newest
		if v.CreatedAt < oldest {
			oldest = v.CreatedAt
		}
		if v.CreatedAt > newest {
			newest = v.CreatedAt
		}
	}

	stats.OldestVersion = oldest
	stats.NewestVersion = newest

	return stats
}

// VersionLister is an interface for listing package versions
type VersionLister interface {
	ListPackageVersions(ctx context.Context, owner, ownerType, packageName string) ([]gh.PackageVersionInfo, error)
}

// StatsParams contains parameters for stats execution
type StatsParams struct {
	Owner       string
	OwnerType   string
	PackageName string
	JSONOutput  bool
	QuietMode   bool
}

// ExecuteStats executes the stats command logic with injected dependencies
func ExecuteStats(ctx context.Context, lister VersionLister, params StatsParams, out io.Writer) error {
	versions, err := lister.ListPackageVersions(ctx, params.Owner, params.OwnerType, params.PackageName)
	if err != nil {
		return fmt.Errorf("failed to list versions: %w", err)
	}

	stats := CalculateStats(versions)
	stats.PackageName = params.PackageName

	if params.JSONOutput {
		return display.OutputJSON(out, stats)
	}

	return OutputStatsTable(out, stats, params.QuietMode)
}

// OutputStatsTable outputs package statistics in table format
func OutputStatsTable(w io.Writer, stats PackageStats, quietMode bool) error {
	if !quietMode {
		fmt.Fprintf(w, "Statistics for %s:\n\n", stats.PackageName)
	}

	fmt.Fprintf(w, "  %-20s %s\n", "Total versions:", display.ColorCount(stats.TotalVersions))
	fmt.Fprintf(w, "  %-20s %s\n", "Tagged versions:", display.ColorCount(stats.TaggedVersions))
	fmt.Fprintf(w, "  %-20s %s\n", "Untagged versions:", display.ColorCount(stats.UntaggedVersions))
	fmt.Fprintf(w, "  %-20s %s\n", "Total tags:", display.ColorCount(stats.TotalTags))

	if stats.OldestVersion != "" {
		fmt.Fprintf(w, "  %-20s %s\n", "Oldest version:", stats.OldestVersion)
	}
	if stats.NewestVersion != "" {
		fmt.Fprintf(w, "  %-20s %s\n", "Newest version:", stats.NewestVersion)
	}

	return nil
}
