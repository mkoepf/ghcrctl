package cmd

import (
	"fmt"
	"os"

	"github.com/mkoepf/ghcrctl/internal/logging"
	"github.com/spf13/cobra"
)

var (
	logAPICalls bool
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
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Enable API call logging if flag is set
		if logAPICalls {
			ctx := logging.EnableLogging(cmd.Context())
			cmd.SetContext(ctx)
		}
	},
}

// Execute runs the root command
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	// Add persistent flag for API call logging
	rootCmd.PersistentFlags().BoolVar(&logAPICalls, "log-api-calls", false, "Log all API calls with timing and categorization to stderr")
}
