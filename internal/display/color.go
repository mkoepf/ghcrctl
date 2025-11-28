package display

import (
	"fmt"
	"strings"

	"github.com/fatih/color"
)

// Color definitions for consistent styling across the application
var (
	// Type colors
	colorIndex       = color.New(color.FgCyan)
	colorManifest    = color.New(color.FgBlue)
	colorPlatform    = color.New(color.FgGreen)
	colorAttestation = color.New(color.FgYellow)

	// Tag colors
	colorTag      = color.New(color.FgGreen, color.Bold)
	colorEmptyTag = color.New(color.Faint)

	// Structure colors
	colorTree      = color.New(color.Faint)
	colorDigest    = color.New(color.Faint)
	colorHeader    = color.New(color.Bold)
	colorSeparator = color.New(color.Faint)

	// Status colors
	colorSuccess = color.New(color.FgGreen)
	colorWarning = color.New(color.FgYellow)
	colorError   = color.New(color.FgRed)
	colorDryRun  = color.New(color.FgCyan)
	colorCount   = color.New(color.Bold)

	// Shared indicator color (magenta for visibility)
	colorShared = color.New(color.FgMagenta, color.Bold)
)

// ColorVersionType applies color to version type strings based on their type.
// - index: cyan
// - manifest: blue
// - platforms (linux/amd64, etc.): green
// - attestations (sbom, provenance): yellow
func ColorVersionType(versionType string) string {
	lower := strings.ToLower(versionType)

	switch {
	case lower == "index":
		return colorIndex.Sprint(versionType)
	case lower == "manifest":
		return colorManifest.Sprint(versionType)
	case strings.Contains(lower, "/"):
		// Platform like "linux/amd64" or "platform: linux/amd64"
		return colorPlatform.Sprint(versionType)
	case lower == "sbom" || lower == "provenance" || lower == "attestation" ||
		strings.HasPrefix(lower, "attestation:"):
		return colorAttestation.Sprint(versionType)
	default:
		return versionType
	}
}

// ColorTags formats and colors a list of tags.
// Empty tags are dimmed, while tags with values are green and bold.
func ColorTags(tags []string) string {
	if len(tags) == 0 {
		return colorEmptyTag.Sprint("[]")
	}

	result := colorTag.Sprint("[")
	for i, tag := range tags {
		if i > 0 {
			result += colorTag.Sprint(", ")
		}
		result += colorTag.Sprint(tag)
	}
	result += colorTag.Sprint("]")
	return result
}

// ColorTreeIndicator applies dim styling to tree structure characters.
func ColorTreeIndicator(indicator string) string {
	return colorTree.Sprint(indicator)
}

// ColorDigest applies dim styling to digest strings.
func ColorDigest(digest string) string {
	return colorDigest.Sprint(digest)
}

// ColorHeader applies bold styling to header text.
func ColorHeader(header string) string {
	return colorHeader.Sprint(header)
}

// ColorSeparator applies dim styling to separator lines.
func ColorSeparator(separator string) string {
	return colorSeparator.Sprint(separator)
}

// ColorSuccess applies green styling for success messages.
func ColorSuccess(msg string) string {
	return colorSuccess.Sprint(msg)
}

// ColorWarning applies yellow styling for warning/confirmation messages.
func ColorWarning(msg string) string {
	return colorWarning.Sprint(msg)
}

// ColorError applies red styling for error messages.
func ColorError(msg string) string {
	return colorError.Sprint(msg)
}

// ColorDryRun applies cyan styling for dry-run indicators.
func ColorDryRun(msg string) string {
	return colorDryRun.Sprint(msg)
}

// ColorCount applies bold styling to counts/numbers.
func ColorCount(n int) string {
	return colorCount.Sprint(fmt.Sprintf("%d", n))
}

// ColorShared applies magenta bold styling to shared indicators.
func ColorShared(msg string) string {
	return colorShared.Sprint(msg)
}
