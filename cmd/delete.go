package cmd

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/mhk/ghcrctl/internal/config"
	"github.com/mhk/ghcrctl/internal/gh"
	"github.com/mhk/ghcrctl/internal/prompts"
	"github.com/spf13/cobra"
)

var (
	deleteForce  bool
	deleteDryRun bool
)

var deleteCmd = &cobra.Command{
	Use:   "delete <image> <version-id>",
	Short: "Delete a package version",
	Long: `Safely delete a package version from GitHub Container Registry.

This command deletes a single package version by its version ID. By default,
it will prompt for confirmation before deleting. Use --force to skip confirmation
or --dry-run to see what would be deleted without actually deleting it.

IMPORTANT: Deletion is permanent and cannot be undone (except within 30 days
via the GitHub web UI if the package namespace is available).

Examples:
  # Delete with confirmation
  ghcrctl delete myimage 12345678

  # Delete without confirmation
  ghcrctl delete myimage 12345678 --force

  # Dry run (show what would be deleted)
  ghcrctl delete myimage 12345678 --dry-run`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		imageName := args[0]
		versionIDStr := args[1]

		// Parse version ID
		versionID, err := strconv.ParseInt(versionIDStr, 10, 64)
		if err != nil {
			cmd.SilenceUsage = true
			return fmt.Errorf("invalid version-id: must be a number, got %q", versionIDStr)
		}

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
		client, err := gh.NewClient(token)
		if err != nil {
			cmd.SilenceUsage = true
			return fmt.Errorf("failed to create GitHub client: %w", err)
		}

		ctx := context.Background()

		// Get version tags to show what we're deleting
		tags, err := client.GetVersionTags(ctx, owner, ownerType, imageName, versionID)
		if err != nil {
			cmd.SilenceUsage = true
			return fmt.Errorf("failed to get version details: %w", err)
		}

		// Show what will be deleted
		fmt.Fprintf(cmd.OutOrStdout(), "Preparing to delete package version:\n")
		fmt.Fprintf(cmd.OutOrStdout(), "  Image:      %s\n", imageName)
		fmt.Fprintf(cmd.OutOrStdout(), "  Owner:      %s (%s)\n", owner, ownerType)
		fmt.Fprintf(cmd.OutOrStdout(), "  Version ID: %d\n", versionID)
		if len(tags) > 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "  Tags:       %v\n", tags)
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "  Tags:       (untagged)\n")
		}
		fmt.Fprintln(cmd.OutOrStdout())

		// Handle dry-run
		if deleteDryRun {
			fmt.Fprintln(cmd.OutOrStdout(), "DRY RUN: No changes made")
			return nil
		}

		// Confirm deletion unless --force is used
		if !deleteForce {
			confirmed, err := prompts.Confirm(os.Stdin, cmd.OutOrStdout(), "Are you sure you want to delete this version?")
			if err != nil {
				return fmt.Errorf("failed to read confirmation: %w", err)
			}

			if !confirmed {
				fmt.Fprintln(cmd.OutOrStdout(), "Deletion cancelled")
				return nil
			}
		}

		// Perform deletion
		err = client.DeletePackageVersion(ctx, owner, ownerType, imageName, versionID)
		if err != nil {
			cmd.SilenceUsage = true
			return fmt.Errorf("failed to delete package version: %w", err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Successfully deleted version %d of %s\n", versionID, imageName)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(deleteCmd)
	deleteCmd.Flags().BoolVar(&deleteForce, "force", false, "Skip confirmation prompt")
	deleteCmd.Flags().BoolVar(&deleteDryRun, "dry-run", false, "Show what would be deleted without deleting")
}
