package display

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// FormatTags formats a list of tags into a bracketed string representation.
// Empty or nil slices return "[]".
func FormatTags(tags []string) string {
	if len(tags) == 0 {
		return "[]"
	}
	result := "["
	for i, tag := range tags {
		if i > 0 {
			result += ", "
		}
		result += tag
	}
	result += "]"
	return result
}

// ShortDigest returns a shortened version of a digest string.
// It removes the "sha256:" prefix and returns the first 12 characters.
func ShortDigest(digest string) string {
	// Remove sha256: prefix and take first 12 characters
	digest = strings.TrimPrefix(digest, "sha256:")
	if len(digest) > 12 {
		return digest[:12]
	}
	return digest
}

// OutputJSON marshals data to indented JSON and writes it to the provided writer.
// This is a common helper used across multiple commands for consistent JSON output.
func OutputJSON(w io.Writer, data interface{}) error {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}
	fmt.Fprintln(w, string(jsonData))
	return nil
}
