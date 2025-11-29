package discover

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/mkoepf/ghcrctl/internal/display"
)

// FormatTable outputs versions in a flat table format.
func FormatTable(w io.Writer, versions []VersionInfo, allVersions map[string]VersionInfo) {
	// Sort versions by ID descending
	sortedVersions := make([]VersionInfo, len(versions))
	copy(sortedVersions, versions)
	sort.Slice(sortedVersions, func(i, j int) bool {
		return sortedVersions[i].ID > sortedVersions[j].ID
	})

	// Calculate dynamic column widths
	idWidth := len("VERSION ID")
	typeWidth := len("TYPE")
	digestWidth := 12
	tagWidth := 20
	refWidth := 20
	for _, v := range sortedVersions {
		idLen := len(fmt.Sprintf("%d", v.ID))
		if idLen > idWidth {
			idWidth = idLen
		}
		typeLen := len(formatTypes(v.Types))
		if typeLen > typeWidth {
			typeWidth = typeLen
		}
		// Check tag widths
		for _, tag := range v.Tags {
			if len(tag) > tagWidth {
				tagWidth = len(tag)
			}
		}
	}

	// Print header (pad first, then color to avoid ANSI length issues)
	fmt.Fprintf(w, "  %s  %s  %s  %s  %s  %s\n",
		display.ColorHeader(fmt.Sprintf("%-*s", idWidth, "VERSION ID")),
		display.ColorHeader(fmt.Sprintf("%-*s", typeWidth, "TYPE")),
		display.ColorHeader(fmt.Sprintf("%-*s", digestWidth, "DIGEST")),
		display.ColorHeader(fmt.Sprintf("%-*s", tagWidth, "TAGS")),
		display.ColorHeader(fmt.Sprintf("%-*s", refWidth, "REFS")),
		display.ColorHeader("CREATED"))
	fmt.Fprintf(w, "  %s  %s  %s  %s  %s  %s\n",
		display.ColorSeparator(strings.Repeat("-", idWidth)),
		display.ColorSeparator(strings.Repeat("-", typeWidth)),
		display.ColorSeparator(strings.Repeat("-", digestWidth)),
		display.ColorSeparator(strings.Repeat("-", tagWidth)),
		display.ColorSeparator(strings.Repeat("-", refWidth)),
		display.ColorSeparator("-------------------"))

	for _, v := range sortedVersions {
		refs := buildRefList(v, allVersions)
		typeStr := formatTypes(v.Types)

		// Combine tags and refs into a single list of "extra" rows
		// First row shows version info + first tag (if any) + first ref (if any)
		// Subsequent rows show remaining tags and refs
		maxRows := max(max(len(v.Tags), len(refs)), 1)

		for row := 0; row < maxRows; row++ {
			var idStr, typeOut, digestOut, tagOut, refOut, createdOut string

			if row == 0 {
				// Pad raw strings first, then apply color to preserve alignment
				idStr = fmt.Sprintf("%-*d", idWidth, v.ID)
				typeOut = display.ColorVersionType(fmt.Sprintf("%-*s", typeWidth, typeStr))
				digestOut = display.ColorDigest(fmt.Sprintf("%-*s", digestWidth, shortDigest(v.Digest)))
				createdOut = v.CreatedAt
			} else {
				// Empty cells need proper padding
				idStr = strings.Repeat(" ", idWidth)
				typeOut = strings.Repeat(" ", typeWidth)
				digestOut = strings.Repeat(" ", digestWidth)
			}

			if row < len(v.Tags) {
				tagOut = fmt.Sprintf("%-*s", tagWidth, v.Tags[row])
			} else {
				tagOut = strings.Repeat(" ", tagWidth)
			}

			if row < len(refs) {
				refOut = padRefString(refs[row], refWidth)
			} else {
				refOut = strings.Repeat(" ", refWidth)
			}

			fmt.Fprintf(w, "  %s  %s  %s  %s  %s  %s\n",
				idStr,
				typeOut,
				digestOut,
				tagOut,
				refOut,
				createdOut)
		}
	}

	// Print summary
	printSummary(w, versions, allVersions)
}

// padRefString pads a ref string (which contains ANSI codes) to the target width.
// The ref format is "[⬇✓] digest" where [⬇✓] is 4 visible chars but more bytes.
func padRefString(ref string, width int) string {
	// The ref indicator "[⬇✓] " is 5 visible chars (including space after)
	// Plus 12 chars for digest = 17 visible chars total
	visibleLen := 5 + 12 // indicator + space + digest
	if len(ref) == 0 {
		return strings.Repeat(" ", width)
	}
	padding := width - visibleLen
	if padding < 0 {
		padding = 0
	}
	return ref + strings.Repeat(" ", padding)
}

// FormatTree outputs versions in a tree-style grouped format.
func FormatTree(w io.Writer, versions []VersionInfo, allVersions map[string]VersionInfo) {
	// Find roots
	var roots []VersionInfo
	for _, v := range versions {
		if v.IsRoot(allVersions) {
			roots = append(roots, v)
		}
	}

	// Sort roots by ID descending
	sort.Slice(roots, func(i, j int) bool {
		return roots[i].ID > roots[j].ID
	})

	// Calculate ref counts: how many roots reference each version
	refCounts := calculateRefCounts(allVersions)

	// Calculate max multiplicity indicator width (e.g., " (2*)" = 5 chars)
	maxMultiplicityWidth := 0
	for _, count := range refCounts {
		if count > 1 {
			indicatorLen := len(fmt.Sprintf(" (%d*)", count))
			if indicatorLen > maxMultiplicityWidth {
				maxMultiplicityWidth = indicatorLen
			}
		}
	}

	// Calculate dynamic column widths
	idWidth := 10 // minimum width
	typeWidth := 4
	for _, v := range versions {
		idLen := len(fmt.Sprintf("%d", v.ID))
		if idLen > idWidth {
			idWidth = idLen
		}
		typeLen := len(formatTypes(v.Types))
		if typeLen > typeWidth {
			typeWidth = typeLen
		}
	}

	for i, root := range roots {
		if i > 0 {
			fmt.Fprintln(w)
		}
		printTree(w, root, allVersions, refCounts, "", true, idWidth, typeWidth, maxMultiplicityWidth)
	}

	// Print summary
	printSummary(w, versions, allVersions)
}

// calculateRefCounts counts how many existing versions reference each version.
// This is used to show multiplicity indicators like (2*) for shared versions.
func calculateRefCounts(allVersions map[string]VersionInfo) map[string]int {
	refCounts := make(map[string]int)
	for _, v := range allVersions {
		// Count incoming refs from versions that exist in allVersions
		for _, inRef := range v.IncomingRefs {
			if _, exists := allVersions[inRef]; exists {
				refCounts[v.Digest]++
			}
		}
	}
	return refCounts
}

func printTree(w io.Writer, v VersionInfo, allVersions map[string]VersionInfo, refCounts map[string]int, prefix string, isRoot bool, idWidth, typeWidth, maxMultiplicityWidth int) {
	typeStr := formatTypes(v.Types)
	tagsStr := ""
	if len(v.Tags) > 0 {
		tagsStr = "  " + formatTags(v.Tags)
	}

	// Collect all children (outgoing and incoming refs)
	var children []struct {
		ref       string
		direction string // "out" or "in"
		found     bool
	}

	for _, outRef := range v.OutgoingRefs {
		_, found := allVersions[outRef]
		children = append(children, struct {
			ref       string
			direction string
			found     bool
		}{outRef, "out", found})
	}

	for _, inRef := range v.IncomingRefs {
		// Only show incoming refs if they're referrers (signatures/attestations)
		if inVer, found := allVersions[inRef]; found {
			if inVer.IsReferrer() {
				children = append(children, struct {
					ref       string
					direction string
					found     bool
				}{inRef, "in", true})
			}
		}
	}

	// Format for alignment:
	// Root with children: "┌      " = 7 visual chars before VERSION ID
	// Root without children: "       " = 7 visual chars before VERSION ID
	// Child: "├ [⬇✓] " = 2 + 5 visible = 7 visual chars before VERSION ID
	// Pad raw strings first, then apply color to preserve alignment
	if isRoot {
		paddedType := display.ColorVersionType(fmt.Sprintf("%-*s", typeWidth, typeStr))
		// Roots don't have multiplicity indicator, so pad with spaces to align with children
		multiPadding := strings.Repeat(" ", maxMultiplicityWidth)
		if len(children) > 0 {
			fmt.Fprintf(w, "%s┌      %-*d%s  %s  %s%s\n",
				prefix, idWidth, v.ID, multiPadding, paddedType, display.ColorDigest(shortDigest(v.Digest)), tagsStr)
		} else {
			fmt.Fprintf(w, "%s       %-*d%s  %s  %s%s\n",
				prefix, idWidth, v.ID, multiPadding, paddedType, display.ColorDigest(shortDigest(v.Digest)), tagsStr)
		}
	}

	for i, child := range children {
		isLast := i == len(children)-1
		connector := "├"
		if isLast {
			connector = "└"
		}

		indicator := buildRefIndicator(child.direction, child.found)

		if child.found {
			childVer := allVersions[child.ref]
			childTypeStr := formatTypes(childVer.Types)
			childTagsStr := ""
			if len(childVer.Tags) > 0 {
				childTagsStr = "  " + formatTags(childVer.Tags)
			}
			// Add multiplicity indicator if version is referenced by multiple roots
			// Pad to maxMultiplicityWidth for alignment
			var multiplicityStr string
			if count := refCounts[childVer.Digest]; count > 1 {
				rawIndicator := fmt.Sprintf(" (%d*)", count)
				padding := maxMultiplicityWidth - len(rawIndicator)
				multiplicityStr = display.ColorShared(rawIndicator) + strings.Repeat(" ", padding)
			} else {
				multiplicityStr = strings.Repeat(" ", maxMultiplicityWidth)
			}
			paddedType := display.ColorVersionType(fmt.Sprintf("%-*s", typeWidth, childTypeStr))
			fmt.Fprintf(w, "%s%s %s %-*d%s  %s  %s%s\n",
				prefix, connector, indicator, idWidth, childVer.ID, multiplicityStr, paddedType,
				display.ColorDigest(shortDigest(childVer.Digest)), childTagsStr)
		} else {
			multiPadding := strings.Repeat(" ", maxMultiplicityWidth)
			paddedType := fmt.Sprintf("%-*s", typeWidth, "???")
			fmt.Fprintf(w, "%s%s %s %-*s%s  %s  %s  (not found)\n",
				prefix, connector, indicator, idWidth, "-", multiPadding, paddedType, display.ColorDigest(shortDigest(child.ref)))
		}
	}
}

func buildRefList(v VersionInfo, allVersions map[string]VersionInfo) []string {
	var refs []string

	// Outgoing refs
	for _, outRef := range v.OutgoingRefs {
		_, found := allVersions[outRef]
		indicator := buildRefIndicator("out", found)
		refs = append(refs, fmt.Sprintf("%s %s", indicator, shortDigest(outRef)))
	}

	// Incoming refs
	for _, inRef := range v.IncomingRefs {
		_, found := allVersions[inRef]
		indicator := buildRefIndicator("in", found)
		refs = append(refs, fmt.Sprintf("%s %s", indicator, shortDigest(inRef)))
	}

	return refs
}

func buildRefIndicator(direction string, found bool) string {
	if direction == "out" {
		if found {
			return display.ColorSuccess("[⬇✓]")
		}
		return display.ColorError("[⬇✗]")
	}
	// incoming
	if found {
		return display.ColorSuccess("[⬆✓]")
	}
	return display.ColorError("[⬆✗]")
}

func shortDigest(digest string) string {
	digest = strings.TrimPrefix(digest, "sha256:")
	if len(digest) > 12 {
		return digest[:12]
	}
	return digest
}

func formatTags(tags []string) string {
	if len(tags) == 0 {
		return "[]"
	}
	return "[" + strings.Join(tags, ", ") + "]"
}

func formatTypes(types []string) string {
	if len(types) == 0 {
		return "unknown"
	}
	return strings.Join(types, ", ")
}

// printSummary prints a summary line with version and graph counts.
func printSummary(w io.Writer, versions []VersionInfo, allVersions map[string]VersionInfo) {
	totalVersions := len(versions)

	// Count roots (graphs)
	var rootCount int
	for _, v := range versions {
		if v.IsRoot(allVersions) {
			rootCount++
		}
	}

	// Count shared versions (appear in multiple graphs)
	refCounts := calculateRefCounts(allVersions)
	var sharedCount int
	for _, count := range refCounts {
		if count > 1 {
			sharedCount++
		}
	}

	// Build summary
	versionWord := "versions"
	if totalVersions == 1 {
		versionWord = "version"
	}
	graphWord := "graphs"
	if rootCount == 1 {
		graphWord = "graph"
	}

	if sharedCount > 0 {
		sharedWord := "versions appear"
		if sharedCount == 1 {
			sharedWord = "version appears"
		}
		fmt.Fprintf(w, "\nTotal: %s %s in %s %s. %s %s in multiple graphs.\n",
			display.ColorCount(totalVersions), versionWord,
			display.ColorCount(rootCount), graphWord,
			display.ColorShared(fmt.Sprintf("%d", sharedCount)), sharedWord)
	} else {
		fmt.Fprintf(w, "\nTotal: %s %s in %s %s.\n",
			display.ColorCount(totalVersions), versionWord,
			display.ColorCount(rootCount), graphWord)
	}
}
