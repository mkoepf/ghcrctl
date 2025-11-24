package cmd

import (
	"context"
	"fmt"

	"github.com/mhk/ghcrctl/internal/config"
	"github.com/mhk/ghcrctl/internal/gh"
	"github.com/mhk/ghcrctl/internal/oras"
	"github.com/spf13/cobra"
)

var tagCmd = &cobra.Command{
	Use:   "tag <image> <existing-tag> <new-tag>",
	Short: "Add a new tag to an existing image version",
	Long: `Add a new tag to an existing GHCR package version.

This command:
1. Resolves the existing tag to a digest using ORAS
2. Finds the GHCR version ID for that digest
3. Updates the package version metadata to add the new tag

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

		// Get GitHub token
		token, err := gh.GetToken()
		if err != nil {
			cmd.SilenceUsage = true
			return err
		}

		// Construct full image reference
		fullImage := fmt.Sprintf("ghcr.io/%s/%s", owner, imageName)

		// Resolve existing tag to digest
		ctx := context.Background()
		digest, err := oras.ResolveTag(ctx, fullImage, existingTag)
		if err != nil {
			cmd.SilenceUsage = true
			return fmt.Errorf("failed to resolve tag '%s': %w", existingTag, err)
		}

		// Create GitHub client
		ghClient, err := gh.NewClient(token)
		if err != nil {
			cmd.SilenceUsage = true
			return fmt.Errorf("failed to create GitHub client: %w", err)
		}

		// Get version ID for the digest
		versionID, err := ghClient.GetVersionIDByDigest(ctx, owner, ownerType, imageName, digest)
		if err != nil {
			cmd.SilenceUsage = true
			return fmt.Errorf("failed to find version for digest: %w", err)
		}

		// Add the new tag to the version
		err = ghClient.AddTagToVersion(ctx, owner, ownerType, imageName, versionID, newTag)
		if err != nil {
			cmd.SilenceUsage = true
			return fmt.Errorf("failed to add tag: %w", err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Successfully added tag '%s' to %s (version %d)\n", newTag, imageName, versionID)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(tagCmd)
}
