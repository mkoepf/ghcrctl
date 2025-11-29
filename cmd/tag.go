package cmd

import (
	"fmt"

	"github.com/mkoepf/ghcrctl/internal/discover"
	"github.com/spf13/cobra"
)

// newTagCmd creates the tag command.
func newTagCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "tag <owner/package:existing-tag> <new-tag>",
		Short: "Add a new tag to an existing image version",
		Long: `Add a new tag to an existing GHCR package version.

This command uses the OCI registry API to copy a tag, creating a new tag
reference that points to the same image digest as the existing tag.

Examples:
  # Promote version to latest
  ghcrctl tag mkoepf/myimage:v1.0.0 latest

  # Add semantic version alias
  ghcrctl tag mkoepf/myimage:v1.2.3 v1.2

  # Tag for environment deployment
  ghcrctl tag mkoepf/myimage:v2.1.0 production`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Parse owner/image:tag reference
			owner, imageName, existingTag, err := parseImageRef(args[0])
			if err != nil {
				cmd.SilenceUsage = true
				return err
			}

			if existingTag == "" {
				cmd.SilenceUsage = true
				return fmt.Errorf("existing tag required: use format owner/image:tag")
			}

			newTag := args[1]

			// Construct full image reference
			fullImage := fmt.Sprintf("ghcr.io/%s/%s", owner, imageName)

			// Use ORAS to copy the tag (creates new tag pointing to same digest)
			ctx := cmd.Context()
			err = discover.CopyTag(ctx, fullImage, existingTag, newTag)
			if err != nil {
				cmd.SilenceUsage = true
				return fmt.Errorf("failed to add tag: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Successfully added tag '%s' to %s:%s\n", newTag, imageName, existingTag)
			return nil
		},
	}
}
