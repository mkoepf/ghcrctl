package cmd

import (
	"context"
	"fmt"
	"io"

	"github.com/mkoepf/ghcrctl/internal/discover"
	"github.com/mkoepf/ghcrctl/internal/display"
	"github.com/mkoepf/ghcrctl/internal/gh"
	"github.com/spf13/cobra"
)

// newTagCmd creates the tag parent command with its subcommands.
func newTagCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tag",
		Short: "Manage tags on package versions",
		Long: `Manage tags on GitHub Container Registry package versions.

Available subcommands:
  add    Add a new tag to an existing version`,
	}

	cmd.AddCommand(newTagAddCmd())

	return cmd
}

// TagAdder is an interface for tag add operations
type TagAdder interface {
	ResolveTag(ctx context.Context, fullImage, tag string) (string, error)
	AddTagByDigest(ctx context.Context, fullImage, digest, newTag string) error
}

// TagAddParams contains parameters for tag add execution
type TagAddParams struct {
	Owner        string
	PackageName  string
	NewTag       string
	SourceTag    string
	SourceDigest string
}

// ExecuteTagAdd executes the tag add logic with injected dependencies
func ExecuteTagAdd(ctx context.Context, adder TagAdder, params TagAddParams, out io.Writer) error {
	fullImage := fmt.Sprintf("ghcr.io/%s/%s", params.Owner, params.PackageName)

	// Resolve source to digest if tag was provided
	targetDigest := params.SourceDigest
	if params.SourceTag != "" {
		var err error
		targetDigest, err = adder.ResolveTag(ctx, fullImage, params.SourceTag)
		if err != nil {
			return fmt.Errorf("failed to resolve source tag '%s': %w", params.SourceTag, err)
		}
	}

	// Add the new tag
	err := adder.AddTagByDigest(ctx, fullImage, targetDigest, params.NewTag)
	if err != nil {
		return fmt.Errorf("failed to add tag: %w", err)
	}

	// Display success message
	if params.SourceTag != "" {
		fmt.Fprintf(out, "Successfully added tag '%s' to %s (source: %s)\n", params.NewTag, params.PackageName, params.SourceTag)
	} else {
		fmt.Fprintf(out, "Successfully added tag '%s' to %s (source: %s)\n", params.NewTag, params.PackageName, params.SourceDigest[:19])
	}
	return nil
}

// newTagAddCmd creates the tag add subcommand.
func newTagAddCmd() *cobra.Command {
	var (
		sourceTag       string
		sourceDigest    string
		sourceVersionID int64
	)

	cmd := &cobra.Command{
		Use:   "add <owner/package> <new-tag>",
		Short: "Add a new tag to an existing version",
		Long: `Add a new tag to an existing GHCR package version.

This command uses the OCI registry API to copy a tag, creating a new tag
reference that points to the same image digest as the source.

Requires a selector to identify the source version: --tag, --digest, or --version.

Examples:
  # Promote version to latest
  ghcrctl tag add mkoepf/myimage latest --tag v1.0.0

  # Add semantic version alias
  ghcrctl tag add mkoepf/myimage v1.2 --tag v1.2.3

  # Tag for environment deployment
  ghcrctl tag add mkoepf/myimage production --tag v2.1.0

  # Tag by version ID
  ghcrctl tag add mkoepf/myimage stable --version 12345678

  # Tag by digest (short form supported)
  ghcrctl tag add mkoepf/myimage stable --digest abc123`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Parse owner/package reference (reject inline tags)
			owner, packageName, err := parsePackageRef(args[0])
			if err != nil {
				cmd.SilenceUsage = true
				return err
			}

			newTag := args[1]

			// Require at least one selector
			if sourceTag == "" && sourceDigest == "" && sourceVersionID == 0 {
				cmd.SilenceUsage = true
				return fmt.Errorf("selector required: use --tag, --digest, or --version to specify the source version")
			}

			// Construct full image reference
			fullImage := fmt.Sprintf("ghcr.io/%s/%s", owner, packageName)

			ctx := cmd.Context()

			var targetDigest string
			var selectorDesc string

			if sourceTag != "" {
				targetDigest, err = discover.ResolveTag(ctx, fullImage, sourceTag)
				if err != nil {
					cmd.SilenceUsage = true
					return fmt.Errorf("failed to resolve source tag '%s': %w", sourceTag, err)
				}
				selectorDesc = sourceTag
			} else if sourceVersionID != 0 || sourceDigest != "" {
				// Need to fetch versions to resolve version ID or short digest
				token, err := gh.GetToken()
				if err != nil {
					cmd.SilenceUsage = true
					return err
				}

				ghClient, err := gh.NewClient(token)
				if err != nil {
					cmd.SilenceUsage = true
					return fmt.Errorf("failed to create GitHub client: %w", err)
				}

				ownerType, err := ghClient.GetOwnerType(ctx, owner)
				if err != nil {
					cmd.SilenceUsage = true
					return fmt.Errorf("failed to determine owner type: %w", err)
				}

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

				if sourceVersionID != 0 {
					targetDigest, err = discover.FindDigestByVersionID(versionMap, sourceVersionID)
					if err != nil {
						cmd.SilenceUsage = true
						return fmt.Errorf("failed to find version ID %d: %w", sourceVersionID, err)
					}
					selectorDesc = fmt.Sprintf("version %d", sourceVersionID)
				} else {
					targetDigest, err = discover.FindDigestByShortDigest(versionMap, sourceDigest)
					if err != nil {
						cmd.SilenceUsage = true
						return fmt.Errorf("failed to find digest '%s': %w", sourceDigest, err)
					}
					selectorDesc = display.ShortDigest(targetDigest)
				}
			}

			// Add the new tag (creates new tag pointing to same digest)
			err = discover.AddTagByDigest(ctx, fullImage, targetDigest, newTag)
			if err != nil {
				cmd.SilenceUsage = true
				return fmt.Errorf("failed to add tag: %w", err)
			}

			// Display success message
			fmt.Fprintf(cmd.OutOrStdout(), "Successfully added tag '%s' to %s (source: %s)\n", newTag, packageName, selectorDesc)
			return nil
		},
	}

	cmd.Flags().StringVar(&sourceTag, "tag", "", "Source version by tag")
	cmd.Flags().StringVar(&sourceDigest, "digest", "", "Source version by digest (supports short form)")
	cmd.Flags().Int64Var(&sourceVersionID, "version", 0, "Source version by ID")
	cmd.MarkFlagsMutuallyExclusive("tag", "digest", "version")

	return cmd
}
