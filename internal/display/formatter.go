package display

import (
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
