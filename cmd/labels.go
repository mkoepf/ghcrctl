package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"

	"github.com/mhk/ghcrctl/internal/config"
	"github.com/mhk/ghcrctl/internal/oras"
	"github.com/spf13/cobra"
)

var (
	labelsTag  string
	labelsKey  string
	labelsJSON bool
)

var labelsCmd = &cobra.Command{
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
		labels, err := getImageLabels(cmd.Context(), fullImage, labelsTag)
		if err != nil {
			cmd.SilenceUsage = true
			return fmt.Errorf("failed to get labels: %w", err)
		}

		// Filter by key if specified
		if labelsKey != "" {
			if value, ok := labels[labelsKey]; ok {
				labels = map[string]string{labelsKey: value}
			} else {
				cmd.SilenceUsage = true
				return fmt.Errorf("label key %q not found", labelsKey)
			}
		}

		// Output results
		if labelsJSON {
			return outputLabelsJSON(cmd.OutOrStdout(), labels)
		}
		return outputLabelsTable(cmd.OutOrStdout(), labels, imageName, labelsTag)
	},
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

func outputLabelsJSON(w io.Writer, labels map[string]string) error {
	data, err := json.MarshalIndent(labels, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}
	fmt.Fprintln(w, string(data))
	return nil
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

func init() {
	rootCmd.AddCommand(labelsCmd)
	labelsCmd.Flags().StringVar(&labelsTag, "tag", "latest", "Tag to resolve (default: latest)")
	labelsCmd.Flags().StringVar(&labelsKey, "key", "", "Show only specific label key")
	labelsCmd.Flags().BoolVar(&labelsJSON, "json", false, "Output in JSON format")
}
