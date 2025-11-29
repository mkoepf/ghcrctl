package cmd

import (
	"fmt"
	"os"

	"github.com/mkoepf/ghcrctl/internal/logging"
	"github.com/spf13/cobra"
)

var (
	// Version is set at build time via ldflags
	// Example: go build -ldflags "-X github.com/mkoepf/ghcrctl/cmd.Version=v1.0.0"
	Version = "dev"
)

// NewRootCmd creates a new root command with isolated flag state.
// This enables parallel test execution by avoiding shared global state.
func NewRootCmd() *cobra.Command {
	var logAPICalls bool

	root := &cobra.Command{
		Use:   "ghcrctl",
		Short: "A CLI tool for managing GitHub Container Registry",
		Long: `ghcrctl is a command-line tool for interacting with GitHub Container Registry (GHCR).

It provides functionality for:
- Exploring packages and their OCI artifact graph (image, SBOM, provenance)
- Managing GHCR version metadata (labels, tags)
- Safe deletion of package versions`,
		SilenceErrors: true,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			// Enable API call logging if flag is set
			if logAPICalls {
				ctx := logging.EnableLogging(cmd.Context())
				cmd.SetContext(ctx)
			}
		},
	}

	// Set version for --version flag
	root.Version = Version

	// Add persistent flag for API call logging
	root.PersistentFlags().BoolVar(&logAPICalls, "log-api-calls", false, "Log all API calls with timing and categorization to stderr")

	// Add subcommands via their factories
	root.AddCommand(newPackagesCmd())
	root.AddCommand(newVersionsCmd())
	root.AddCommand(newDeleteCmd())
	root.AddCommand(newLabelsCmd())
	root.AddCommand(newSBOMCmd())
	root.AddCommand(newProvenanceCmd())
	root.AddCommand(newTagCmd())
	root.AddCommand(newCompletionCmd())

	return root
}

// rootCmd is the global command instance used by main.go
var rootCmd = NewRootCmd()

// Execute runs the root command
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
