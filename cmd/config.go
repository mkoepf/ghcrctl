package cmd

import (
	"fmt"

	"github.com/mhk/ghcrctl/internal/config"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage ghcrctl configuration",
	Long:  `Manage configuration stored in ~/.ghcrctl/config.yaml`,
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Display current configuration",
	Long:  `Display the current ghcrctl configuration from ~/.ghcrctl/config.yaml`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.New()
		name, ownerType, err := cfg.GetOwner()
		if err != nil {
			return fmt.Errorf("failed to read configuration: %w", err)
		}

		if name == "" || ownerType == "" {
			fmt.Println("No configuration found.")
			fmt.Println("Set an organization with: ghcrctl config org <org-name>")
			fmt.Println("Set a user with: ghcrctl config user <user-name>")
		} else {
			fmt.Printf("owner-name: %s\n", name)
			fmt.Printf("owner-type: %s\n", ownerType)
		}

		return nil
	},
}

var configOrgCmd = &cobra.Command{
	Use:   "org <org-name>",
	Short: "Set the GHCR owner as an organization",
	Long:  `Set the GitHub Container Registry owner as an organization in the configuration.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		// Validate that name is not empty
		if name == "" {
			return fmt.Errorf("owner name cannot be empty")
		}

		cfg := config.New()

		if err := cfg.SetOwner(name, "org"); err != nil {
			return fmt.Errorf("failed to set owner: %w", err)
		}

		fmt.Printf("Successfully set owner to organization: %s\n", name)
		return nil
	},
}

var configUserCmd = &cobra.Command{
	Use:   "user <user-name>",
	Short: "Set the GHCR owner as a user",
	Long:  `Set the GitHub Container Registry owner as a user in the configuration.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		// Validate that name is not empty
		if name == "" {
			return fmt.Errorf("owner name cannot be empty")
		}

		cfg := config.New()

		if err := cfg.SetOwner(name, "user"); err != nil {
			return fmt.Errorf("failed to set owner: %w", err)
		}

		fmt.Printf("Successfully set owner to user: %s\n", name)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configOrgCmd)
	configCmd.AddCommand(configUserCmd)
}
