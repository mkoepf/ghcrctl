package cmd

import (
	"fmt"

	"github.com/mhk/ghcrctl/internal/config"
	"github.com/mhk/ghcrctl/internal/oras"
	"github.com/spf13/cobra"
)

var tagCmd = &cobra.Command{
	Use:   "tag <image> <existing-tag> <new-tag>",
	Short: "Add a new tag to an existing image version",
	Long: `Add a new tag to an existing GHCR package version.

This command uses the OCI registry API to copy a tag, creating a new tag
reference that points to the same image digest as the existing tag.

Example:
  ghcrctl tag myimage v1.0.0 latest`,
	Args: cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		imageName := args[0]
		existingTag := args[1]
		newTag := args[2]

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

		// Construct full image reference
		fullImage := fmt.Sprintf("ghcr.io/%s/%s", owner, imageName)

		// Use ORAS to copy the tag (creates new tag pointing to same digest)
		ctx := cmd.Context()
		err = oras.CopyTag(ctx, fullImage, existingTag, newTag)
		if err != nil {
			cmd.SilenceUsage = true
			return fmt.Errorf("failed to add tag: %w", err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Successfully added tag '%s' to %s:%s\n", newTag, imageName, existingTag)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(tagCmd)
}
