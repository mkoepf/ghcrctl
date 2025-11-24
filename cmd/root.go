package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "ghcrctl",
	Short: "A CLI tool for managing GitHub Container Registry",
	Long: `ghcrctl is a command-line tool for interacting with GitHub Container Registry (GHCR).

It provides functionality for:
- Exploring images and their OCI artifact graph (image, SBOM, provenance)
- Managing GHCR version metadata (labels, tags)
- Safe deletion of package versions
- Configuration of owner/org and authentication`,
	SilenceErrors: true,
}

// Execute runs the root command
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
