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
	for _, v := range sortedVersions {
		idLen := len(fmt.Sprintf("%d", v.ID))
		if idLen > idWidth {
			idWidth = idLen
		}
		typeLen := len(formatTypes(v.Types))
		if typeLen > typeWidth {
			typeWidth = typeLen
		}
	}

	// Print header
	fmt.Fprintf(w, "  %-*s  %-*s  %-12s  %-20s  %-20s  %s\n",
		idWidth, display.ColorHeader("VERSION ID"),
		typeWidth, display.ColorHeader("TYPE"),
		display.ColorHeader("DIGEST"),
		display.ColorHeader("TAGS"),
		display.ColorHeader("REFS"),
		display.ColorHeader("CREATED"))
	fmt.Fprintf(w, "  %s  %s  %s  %s  %s  %s\n",
		display.ColorSeparator(strings.Repeat("-", idWidth)),
		display.ColorSeparator(strings.Repeat("-", typeWidth)),
		display.ColorSeparator("------------"),
		display.ColorSeparator("--------------------"),
		display.ColorSeparator("--------------------"),
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
				idStr = fmt.Sprintf("%d", v.ID)
				typeOut = display.ColorVersionType(typeStr)
				digestOut = display.ColorDigest(shortDigest(v.Digest))
				createdOut = v.CreatedAt
			}

			if row < len(v.Tags) {
				tagOut = v.Tags[row]
			}
			if row < len(refs) {
				refOut = refs[row]
			}

			fmt.Fprintf(w, "  %-*s  %-*s  %-12s  %-20s  %-20s  %s\n",
				idWidth, idStr,
				typeWidth, typeOut,
				digestOut,
				tagOut,
				refOut,
				createdOut)
		}
	}
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
		printTree(w, root, allVersions, "", true, idWidth, typeWidth)
	}
}

func printTree(w io.Writer, v VersionInfo, allVersions map[string]VersionInfo, prefix string, isRoot bool, idWidth, typeWidth int) {
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
	// Root with children: "┌ " + 5 spaces = 7 visual chars before VERSION ID
	// Root without children: 7 spaces = 7 visual chars before VERSION ID
	// Child: "├ " + "[⬇✓] " = 2 + 5 visible = 7 visual chars before VERSION ID
	// The indicator "[⬇✓] " is 5 visible chars but 9 bytes due to Unicode arrows
	if isRoot {
		if len(children) > 0 {
			fmt.Fprintf(w, "%s┌      %-*d  %-*s  %-12s%s\n",
				prefix, idWidth, v.ID, typeWidth, display.ColorVersionType(typeStr), display.ColorDigest(shortDigest(v.Digest)), tagsStr)
		} else {
			fmt.Fprintf(w, "%s       %-*d  %-*s  %-12s%s\n",
				prefix, idWidth, v.ID, typeWidth, display.ColorVersionType(typeStr), display.ColorDigest(shortDigest(v.Digest)), tagsStr)
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
			fmt.Fprintf(w, "%s%s %s %-*d  %-*s  %-12s%s\n",
				prefix, connector, indicator, idWidth, childVer.ID, typeWidth, display.ColorVersionType(childTypeStr),
				display.ColorDigest(shortDigest(childVer.Digest)), childTagsStr)
		} else {
			fmt.Fprintf(w, "%s%s %s %-*s  %-*s  %-12s  (not found)\n",
				prefix, connector, indicator, idWidth, "-", typeWidth, "???", display.ColorDigest(shortDigest(child.ref)))
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
