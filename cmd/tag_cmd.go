package cmd

import (
	"context"
	"fmt"
	"io"

	"github.com/mkoepf/ghcrctl/internal/discover"
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

// TagCopier is an interface for tag copying operations
type TagCopier interface {
	ResolveTag(ctx context.Context, fullImage, tag string) (string, error)
	CopyTagByDigest(ctx context.Context, fullImage, digest, newTag string) error
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
func ExecuteTagAdd(ctx context.Context, copier TagCopier, params TagAddParams, out io.Writer) error {
	fullImage := fmt.Sprintf("ghcr.io/%s/%s", params.Owner, params.PackageName)

	// Resolve source to digest if tag was provided
	targetDigest := params.SourceDigest
	if params.SourceTag != "" {
		var err error
		targetDigest, err = copier.ResolveTag(ctx, fullImage, params.SourceTag)
		if err != nil {
			return fmt.Errorf("failed to resolve source tag '%s': %w", params.SourceTag, err)
		}
	}

	// Use ORAS to copy the tag
	err := copier.CopyTagByDigest(ctx, fullImage, targetDigest, params.NewTag)
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
		sourceTag    string
		sourceDigest string
	)

	cmd := &cobra.Command{
		Use:   "add <owner/package> <new-tag>",
		Short: "Add a new tag to an existing version",
		Long: `Add a new tag to an existing GHCR package version.

This command uses the OCI registry API to copy a tag, creating a new tag
reference that points to the same image digest as the source.

Requires a selector to identify the source version: --tag or --digest.

Examples:
  # Promote version to latest
  ghcrctl tag add mkoepf/myimage latest --tag v1.0.0

  # Add semantic version alias
  ghcrctl tag add mkoepf/myimage v1.2 --tag v1.2.3

  # Tag for environment deployment
  ghcrctl tag add mkoepf/myimage production --tag v2.1.0

  # Tag by digest
  ghcrctl tag add mkoepf/myimage stable --digest sha256:abc123...`,
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
			if sourceTag == "" && sourceDigest == "" {
				cmd.SilenceUsage = true
				return fmt.Errorf("selector required: use --tag or --digest to specify the source version")
			}

			// Construct full image reference
			fullImage := fmt.Sprintf("ghcr.io/%s/%s", owner, packageName)

			ctx := cmd.Context()

			// Resolve source to digest if tag was provided
			targetDigest := sourceDigest
			if sourceTag != "" {
				var err error
				targetDigest, err = discover.ResolveTag(ctx, fullImage, sourceTag)
				if err != nil {
					cmd.SilenceUsage = true
					return fmt.Errorf("failed to resolve source tag '%s': %w", sourceTag, err)
				}
			}

			// Use ORAS to copy the tag (creates new tag pointing to same digest)
			err = discover.CopyTagByDigest(ctx, fullImage, targetDigest, newTag)
			if err != nil {
				cmd.SilenceUsage = true
				return fmt.Errorf("failed to add tag: %w", err)
			}

			// Display success message
			if sourceTag != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "Successfully added tag '%s' to %s (source: %s)\n", newTag, packageName, sourceTag)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "Successfully added tag '%s' to %s (source: %s)\n", newTag, packageName, sourceDigest[:19])
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&sourceTag, "tag", "", "Source version by tag")
	cmd.Flags().StringVar(&sourceDigest, "digest", "", "Source version by digest")
	cmd.MarkFlagsMutuallyExclusive("tag", "digest")

	return cmd
}
