package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/mkoepf/ghcrctl/internal/discover"
	"github.com/mkoepf/ghcrctl/internal/display"
)

// fetchAndDisplayArtifact fetches and displays a single artifact
func fetchAndDisplayArtifact(w io.Writer, ctx context.Context, image, digest string, jsonOutput bool, artifactType string) error {
	// Fetch the artifact content
	content, err := discover.FetchArtifactContent(ctx, image, digest)
	if err != nil {
		return fmt.Errorf("failed to fetch %s: %w", artifactType, err)
	}

	// Display the content
	if jsonOutput {
		return display.OutputJSON(w, content)
	}
	return outputArtifactReadable(w, content, digest, artifactType)
}

// fetchAndDisplayAllArtifacts fetches and displays all artifacts
func fetchAndDisplayAllArtifacts(w io.Writer, ctx context.Context, image string, artifacts []discover.VersionInfo, jsonOutput bool, artifactType string) error {
	allContent := make([]interface{}, 0, len(artifacts))

	for _, artifact := range artifacts {
		content, err := discover.FetchArtifactContent(ctx, image, artifact.Digest)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to fetch %s %s: %v\n", artifactType, artifact.Digest, err)
			continue
		}

		if jsonOutput {
			allContent = append(allContent, map[string]interface{}{
				"digest":  artifact.Digest,
				"content": content,
			})
		} else {
			fmt.Fprintf(w, "\n=== %s: %s ===\n", capitalizeFirst(artifactType), display.ShortDigest(artifact.Digest))
			if err := outputArtifactReadable(w, content, artifact.Digest, artifactType); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to display %s %s: %v\n", artifactType, artifact.Digest, err)
			}
		}
	}

	if jsonOutput {
		data, err := json.MarshalIndent(allContent, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Fprintln(w, string(data))
	}

	return nil
}

// listArtifacts lists available artifacts without fetching their content
// selectorType is "tag", "digest", or "version" to describe how the image was selected
// selectorValue is the actual value used (e.g., "v1.0.0", "abc123", "12345678")
func listArtifacts(w io.Writer, artifacts []discover.VersionInfo, imageName, artifactType, selectorType, selectorValue string) error {
	// Build context-aware message
	var imageDesc string
	switch selectorType {
	case "tag":
		imageDesc = fmt.Sprintf("image tagged '%s'", selectorValue)
	case "version":
		imageDesc = fmt.Sprintf("image containing version %s", selectorValue)
	default: // digest
		imageDesc = fmt.Sprintf("image containing digest %s", selectorValue)
	}

	fmt.Fprintf(w, "Multiple %s documents found in %s\n\n", artifactType, imageDesc)
	fmt.Fprintf(w, "Select one by digest, or use --all to show all:\n\n")

	for i, artifact := range artifacts {
		fmt.Fprintf(w, "  %d. %s\n", i+1, artifact.Digest)
	}

	fmt.Fprintf(w, "\nExample: ghcrctl get %s %s --digest %s\n", artifactType, imageName, display.ShortDigest(artifacts[0].Digest))

	return nil
}

// outputArtifactReadable outputs artifact content in human-readable format
func outputArtifactReadable(w io.Writer, content []map[string]interface{}, digest, artifactType string) error {
	fmt.Fprintf(w, "%s: %s\n\n", capitalizeFirst(artifactType), display.ShortDigest(digest))

	for _, attestation := range content {
		// Pretty-print the JSON
		data, err := json.MarshalIndent(attestation, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to format content: %w", err)
		}
		fmt.Fprintln(w, string(data))
	}

	return nil
}

// capitalizeFirst returns the string with the first letter capitalized
func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	return string(s[0]-32) + s[1:]
}
