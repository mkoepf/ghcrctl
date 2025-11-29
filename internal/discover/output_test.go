package discover

import (
	"bytes"
	"strings"
	"testing"
)

func TestFormatTable_Basic(t *testing.T) {
	versions := []VersionInfo{
		{
			ID:           123,
			Digest:       "sha256:abc123def456abc123def456abc123def456abc123def456abc123def456abc1",
			Tags:         []string{"v1.0.0"},
			Types:        []string{"index"},
			OutgoingRefs: []string{"sha256:def456"},
			CreatedAt:    "2025-01-15 10:30:45",
		},
	}

	allVersions := make(map[string]VersionInfo)
	allVersions["sha256:def456"] = VersionInfo{Digest: "sha256:def456"}

	var buf bytes.Buffer
	FormatTable(&buf, versions, allVersions)

	output := buf.String()
	if !strings.Contains(output, "VERSION ID") {
		t.Error("expected header with VERSION ID")
	}
	if !strings.Contains(output, "123") {
		t.Error("expected version ID 123")
	}
	if !strings.Contains(output, "index") {
		t.Error("expected type index")
	}
}

func TestFormatTable_RefIndicators(t *testing.T) {
	versions := []VersionInfo{
		{
			ID:           123,
			Digest:       "sha256:abc123def456abc123def456abc123def456abc123def456abc123def456abc1",
			Tags:         []string{"v1.0.0"},
			Types:        []string{"index"},
			OutgoingRefs: []string{"sha256:found123", "sha256:missing1"},
			IncomingRefs: []string{"sha256:incoming"},
			CreatedAt:    "2025-01-15 10:30:45",
		},
	}

	allVersions := make(map[string]VersionInfo)
	allVersions["sha256:abc123def456abc123def456abc123def456abc123def456abc123def456abc1"] = versions[0]
	allVersions["sha256:found123"] = VersionInfo{Digest: "sha256:found123"}
	allVersions["sha256:incoming"] = VersionInfo{Digest: "sha256:incoming"}
	// sha256:missing1 is NOT in allVersions

	var buf bytes.Buffer
	FormatTable(&buf, versions, allVersions)

	output := buf.String()
	// Should have [⬇✓] for found outgoing ref
	if !strings.Contains(output, "[⬇✓]") {
		t.Error("expected [⬇✓] for found outgoing ref")
	}
	// Should have [⬇✗] for missing outgoing ref
	if !strings.Contains(output, "[⬇✗]") {
		t.Error("expected [⬇✗] for missing outgoing ref")
	}
	// Should have [⬆✓] for found incoming ref
	if !strings.Contains(output, "[⬆✓]") {
		t.Error("expected [⬆✓] for found incoming ref")
	}
}

func TestShortDigest(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"sha256:abc123def456abc123def456abc123def456abc123def456abc123def456abc1", "abc123def456"},
		{"abc123def456", "abc123def456"},
		{"short", "short"},
	}

	for _, tt := range tests {
		result := shortDigest(tt.input)
		if result != tt.expected {
			t.Errorf("shortDigest(%s) = %s, want %s", tt.input, result, tt.expected)
		}
	}
}

func TestFormatTags(t *testing.T) {
	tests := []struct {
		tags     []string
		expected string
	}{
		{nil, "[]"},
		{[]string{}, "[]"},
		{[]string{"v1.0.0"}, "[v1.0.0]"},
		{[]string{"v1.0.0", "latest"}, "[v1.0.0, latest]"},
	}

	for _, tt := range tests {
		result := formatTags(tt.tags)
		if result != tt.expected {
			t.Errorf("formatTags(%v) = %s, want %s", tt.tags, result, tt.expected)
		}
	}
}

func TestFormatTable_MultipleTagsAsRows(t *testing.T) {
	versions := []VersionInfo{
		{
			ID:        123,
			Digest:    "sha256:abc123",
			Tags:      []string{"v1.0.0", "latest", "stable"},
			Types:     []string{"index"},
			CreatedAt: "2025-01-15 10:30:45",
		},
	}

	allVersions := make(map[string]VersionInfo)
	allVersions["sha256:abc123"] = versions[0]

	var buf bytes.Buffer
	FormatTable(&buf, versions, allVersions)

	output := buf.String()
	lines := strings.Split(output, "\n")

	// Each tag should be on a SEPARATE row (not all on one line)
	// Count lines containing each tag individually
	linesWithV1 := 0
	linesWithLatest := 0
	linesWithStable := 0
	for _, line := range lines {
		if strings.Contains(line, "v1.0.0") {
			linesWithV1++
		}
		if strings.Contains(line, "latest") {
			linesWithLatest++
		}
		if strings.Contains(line, "stable") {
			linesWithStable++
		}
	}

	// Each tag should appear on exactly one line, and they should be different lines
	if linesWithV1 != 1 || linesWithLatest != 1 || linesWithStable != 1 {
		t.Errorf("expected each tag on one line, got v1.0.0=%d, latest=%d, stable=%d\noutput:\n%s",
			linesWithV1, linesWithLatest, linesWithStable, output)
	}

	// Also verify tags are NOT all on the same line
	for _, line := range lines {
		if strings.Contains(line, "v1.0.0") && strings.Contains(line, "latest") && strings.Contains(line, "stable") {
			t.Errorf("all tags should not be on the same line\noutput:\n%s", output)
		}
	}
}

func TestFormatTree_MultiplicityIndicator(t *testing.T) {
	// Create versions where one child is referenced by multiple roots
	versions := []VersionInfo{
		{
			ID:           100,
			Digest:       "sha256:root1",
			Types:        []string{"index"},
			OutgoingRefs: []string{"sha256:shared"},
		},
		{
			ID:           200,
			Digest:       "sha256:root2",
			Types:        []string{"index"},
			OutgoingRefs: []string{"sha256:shared"},
		},
		{
			ID:           300,
			Digest:       "sha256:shared",
			Types:        []string{"linux/amd64"},
			IncomingRefs: []string{"sha256:root1", "sha256:root2"},
		},
	}

	allVersions := make(map[string]VersionInfo)
	for _, v := range versions {
		allVersions[v.Digest] = v
	}

	var buf bytes.Buffer
	FormatTree(&buf, versions, allVersions)

	output := buf.String()
	// Should contain "(2*)" indicator for shared version
	if !strings.Contains(output, "(2*)") {
		t.Errorf("expected multiplicity indicator (2*) for shared version\noutput:\n%s", output)
	}
}

func TestFormatTree_Summary(t *testing.T) {
	versions := []VersionInfo{
		{
			ID:           100,
			Digest:       "sha256:root1",
			Types:        []string{"index"},
			OutgoingRefs: []string{"sha256:shared"},
		},
		{
			ID:           200,
			Digest:       "sha256:root2",
			Types:        []string{"index"},
			OutgoingRefs: []string{"sha256:shared"},
		},
		{
			ID:           300,
			Digest:       "sha256:shared",
			Types:        []string{"linux/amd64"},
			IncomingRefs: []string{"sha256:root1", "sha256:root2"},
		},
	}

	allVersions := make(map[string]VersionInfo)
	for _, v := range versions {
		allVersions[v.Digest] = v
	}

	var buf bytes.Buffer
	FormatTree(&buf, versions, allVersions)

	output := buf.String()

	// Should contain summary with version count
	if !strings.Contains(output, "Total:") {
		t.Errorf("expected summary with 'Total:'\noutput:\n%s", output)
	}
	if !strings.Contains(output, "3 versions") {
		t.Errorf("expected '3 versions' in summary\noutput:\n%s", output)
	}
	if !strings.Contains(output, "2 graphs") {
		t.Errorf("expected '2 graphs' in summary\noutput:\n%s", output)
	}
	// Should mention shared versions
	if !strings.Contains(output, "1 version appears in multiple graphs") {
		t.Errorf("expected shared version count in summary\noutput:\n%s", output)
	}
}

func TestFormatTable_Summary(t *testing.T) {
	versions := []VersionInfo{
		{
			ID:        123,
			Digest:    "sha256:abc123",
			Tags:      []string{"v1.0.0"},
			Types:     []string{"index"},
			CreatedAt: "2025-01-15 10:30:45",
		},
	}

	allVersions := make(map[string]VersionInfo)
	allVersions["sha256:abc123"] = versions[0]

	var buf bytes.Buffer
	FormatTable(&buf, versions, allVersions)

	output := buf.String()

	// Should contain summary
	if !strings.Contains(output, "Total:") {
		t.Errorf("expected summary with 'Total:'\noutput:\n%s", output)
	}
	if !strings.Contains(output, "1 version") {
		t.Errorf("expected '1 version' in summary\noutput:\n%s", output)
	}
}

func TestFormatTree_AlignedVersionIDs(t *testing.T) {
	versions := []VersionInfo{
		{
			ID:           123,
			Digest:       "sha256:root123",
			Tags:         []string{"v1.0.0"},
			Types:        []string{"index"},
			OutgoingRefs: []string{"sha256:child456"},
			CreatedAt:    "2025-01-15 10:30:45",
		},
		{
			ID:           456,
			Digest:       "sha256:child456",
			Tags:         []string{},
			Types:        []string{"linux/amd64"},
			IncomingRefs: []string{"sha256:root123"},
			CreatedAt:    "2025-01-15 10:30:45",
		},
	}

	allVersions := make(map[string]VersionInfo)
	for _, v := range versions {
		allVersions[v.Digest] = v
	}

	var buf bytes.Buffer
	FormatTree(&buf, versions, allVersions)

	output := buf.String()
	lines := strings.Split(output, "\n")

	// Find where VERSION ID starts on each line (visual position, not byte position)
	var idPositions []int
	for _, line := range lines {
		if line == "" {
			continue
		}
		runes := []rune(line)
		// Find visual position of first digit (version ID)
		for i, ch := range runes {
			if ch >= '0' && ch <= '9' {
				idPositions = append(idPositions, i)
				break
			}
		}
	}

	// All version IDs should start at the same visual position
	if len(idPositions) >= 2 {
		for i := 1; i < len(idPositions); i++ {
			if idPositions[i] != idPositions[0] {
				t.Errorf("version IDs not aligned: visual positions %v\noutput:\n%s", idPositions, output)
				break
			}
		}
	}
}
