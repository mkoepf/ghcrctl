package cmd

import (
	"context"
	"fmt"
	"io"
	"sort"

	"github.com/mkoepf/ghcrctl/internal/config"
	"github.com/mkoepf/ghcrctl/internal/display"
	"github.com/mkoepf/ghcrctl/internal/oras"
	"github.com/spf13/cobra"
)

// newLabelsCmd creates the labels command with isolated flag state.
func newLabelsCmd() *cobra.Command {
	var (
		tag          string
		key          string
		jsonOutput   bool
		outputFormat string
	)

	cmd := &cobra.Command{
		Use:   "labels <image>",
		Short: "Display OCI labels from a container image",
		Long: `Display OCI labels (annotations/metadata) from a container image.

Labels are key-value pairs embedded in the image config at build time using
Docker LABEL instructions or equivalent mechanisms. Common labels include:
  - org.opencontainers.image.source
  - org.opencontainers.image.description
  - org.opencontainers.image.version
  - org.opencontainers.image.licenses

Examples:
  # Show all labels from latest tag
  ghcrctl labels myimage

  # Show labels from specific tag
  ghcrctl labels myimage --tag v1.0.0

  # Show specific label key
  ghcrctl labels myimage --key org.opencontainers.image.source

  # JSON output
  ghcrctl labels myimage --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			imageName := args[0]

			// Handle output format flag (-o)
			if outputFormat != "" {
				switch outputFormat {
				case "json":
					jsonOutput = true
				case "table":
					jsonOutput = false
				default:
					cmd.SilenceUsage = true
					return fmt.Errorf("invalid output format %q. Supported formats: json, table", outputFormat)
				}
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

			// Construct full image reference
			fullImage := fmt.Sprintf("ghcr.io/%s/%s", owner, imageName)

			// Get labels from image
			labels, err := getImageLabels(cmd.Context(), fullImage, tag)
			if err != nil {
				cmd.SilenceUsage = true
				return fmt.Errorf("failed to get labels: %w", err)
			}

			// Filter by key if specified
			if key != "" {
				if value, ok := labels[key]; ok {
					labels = map[string]string{key: value}
				} else {
					cmd.SilenceUsage = true
					return fmt.Errorf("label key %q not found", key)
				}
			}

			// Output results
			if jsonOutput {
				return display.OutputJSON(cmd.OutOrStdout(), labels)
			}
			return outputLabelsTable(cmd.OutOrStdout(), labels, imageName, tag)
		},
	}

	cmd.Flags().StringVar(&tag, "tag", "latest", "Tag to resolve (default: latest)")
	cmd.Flags().StringVar(&key, "key", "", "Show only specific label key")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output in JSON format")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "", "Output format (json, table)")

	return cmd
}

func getImageLabels(ctx context.Context, image, tag string) (map[string]string, error) {
	// Resolve tag to digest
	digest, err := oras.ResolveTag(ctx, image, tag)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve tag: %w", err)
	}

	// Fetch image config to get labels
	config, err := oras.FetchImageConfig(ctx, image, digest)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch image config: %w", err)
	}

	return config.Config.Labels, nil
}

func outputLabelsTable(w io.Writer, labels map[string]string, image, tag string) error {
	if len(labels) == 0 {
		fmt.Fprintf(w, "No labels found for %s:%s\n", image, tag)
		return nil
	}

	fmt.Fprintf(w, "Labels for %s:%s\n\n", image, tag)

	// Sort keys for consistent output
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Find max key length for alignment
	maxKeyLen := 0
	for _, k := range keys {
		if len(k) > maxKeyLen {
			maxKeyLen = len(k)
		}
	}

	// Print labels
	for _, k := range keys {
		fmt.Fprintf(w, "  %-*s  %s\n", maxKeyLen, k, labels[k])
	}

	fmt.Fprintf(w, "\nTotal: %d label(s)\n", len(labels))
	return nil
}
