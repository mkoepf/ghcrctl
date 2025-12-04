package discover

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFormatTable_Basic(t *testing.T) {
	versions := []VersionInfo{
		{
			ID:           123,
			Digest:       "sha256:abc123def456abc123def456abc123def456abc123def456abc123def456abc1",
			Tags:         []string{"v1.0.0"},
			Types:        []string{"index"},
			Size:         1258291, // ~1.2 MB
			OutgoingRefs: []string{"sha256:def456"},
			CreatedAt:    "2025-01-15 10:30:45",
		},
	}

	allVersions := make(map[string]VersionInfo)
	allVersions["sha256:def456"] = VersionInfo{Digest: "sha256:def456"}

	var buf bytes.Buffer
	FormatTable(&buf, versions, allVersions)

	output := buf.String()
	assert.Contains(t, output, "VERSION ID")
	assert.Contains(t, output, "SIZE")
	assert.Contains(t, output, "123")
	assert.Contains(t, output, "index")
	assert.Contains(t, output, "1.2 MB")
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
	assert.Contains(t, output, "[⬇✓]", "expected [⬇✓] for found outgoing ref")
	// Should have [⬇✗] for missing outgoing ref
	assert.Contains(t, output, "[⬇✗]", "expected [⬇✗] for missing outgoing ref")
	// Should have [⬆✓] for found incoming ref
	assert.Contains(t, output, "[⬆✓]", "expected [⬆✓] for found incoming ref")
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
		assert.Equal(t, tt.expected, result, "shortDigest(%s)", tt.input)
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
		assert.Equal(t, tt.expected, result, "formatTags(%v)", tt.tags)
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
	assert.Equal(t, 1, linesWithV1, "v1.0.0 should appear on one line")
	assert.Equal(t, 1, linesWithLatest, "latest should appear on one line")
	assert.Equal(t, 1, linesWithStable, "stable should appear on one line")

	// Also verify tags are NOT all on the same line
	for _, line := range lines {
		allOnSameLine := strings.Contains(line, "v1.0.0") && strings.Contains(line, "latest") && strings.Contains(line, "stable")
		assert.False(t, allOnSameLine, "all tags should not be on the same line")
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
	assert.Contains(t, output, "(2*)", "expected multiplicity indicator (2*) for shared version")
}

func TestFormatTree_Header(t *testing.T) {
	versions := []VersionInfo{
		{
			ID:           100,
			Digest:       "sha256:root1",
			Types:        []string{"index"},
			Size:         1536,
			OutgoingRefs: []string{"sha256:child1"},
		},
		{
			ID:           200,
			Digest:       "sha256:child1",
			Types:        []string{"linux/amd64"},
			Size:         1024,
			IncomingRefs: []string{"sha256:root1"},
		},
	}

	allVersions := make(map[string]VersionInfo)
	for _, v := range versions {
		allVersions[v.Digest] = v
	}

	var buf bytes.Buffer
	FormatTree(&buf, versions, allVersions)

	output := buf.String()
	// Tree view should have column headers like table view
	assert.Contains(t, output, "VERSION ID")
	assert.Contains(t, output, "TYPE")
	assert.Contains(t, output, "DIGEST")
	assert.Contains(t, output, "SIZE")
}

func TestFormatTree_Size(t *testing.T) {
	versions := []VersionInfo{
		{
			ID:           100,
			Digest:       "sha256:root1",
			Types:        []string{"index"},
			Size:         1536, // 1.5 KB
			OutgoingRefs: []string{"sha256:child1"},
		},
		{
			ID:           200,
			Digest:       "sha256:child1",
			Types:        []string{"linux/amd64"},
			Size:         52428800, // 50 MB
			IncomingRefs: []string{"sha256:root1"},
		},
	}

	allVersions := make(map[string]VersionInfo)
	for _, v := range versions {
		allVersions[v.Digest] = v
	}

	var buf bytes.Buffer
	FormatTree(&buf, versions, allVersions)

	output := buf.String()
	// Root should show 1.5 KB
	assert.Contains(t, output, "1.5 KB", "expected root size 1.5 KB in output")
	// Child should show 50.0 MB
	assert.Contains(t, output, "50.0 MB", "expected child size 50.0 MB in output")
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
	assert.Contains(t, output, "Total:")
	assert.Contains(t, output, "3 versions")
	assert.Contains(t, output, "2 graphs")
	// Should mention shared versions
	assert.Contains(t, output, "1 version appears in multiple graphs")
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
	assert.Contains(t, output, "Total:")
	assert.Contains(t, output, "1 version")
}

func TestFormatSize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		bytes int64
		want  string
	}{
		{
			name:  "zero bytes",
			bytes: 0,
			want:  "-",
		},
		{
			name:  "negative bytes",
			bytes: -1,
			want:  "-",
		},
		{
			name:  "small bytes",
			bytes: 500,
			want:  "500 B",
		},
		{
			name:  "exactly 1 KB",
			bytes: 1024,
			want:  "1.0 KB",
		},
		{
			name:  "1.5 KB",
			bytes: 1536,
			want:  "1.5 KB",
		},
		{
			name:  "exactly 1 MB",
			bytes: 1024 * 1024,
			want:  "1.0 MB",
		},
		{
			name:  "50 MB",
			bytes: 50 * 1024 * 1024,
			want:  "50.0 MB",
		},
		{
			name:  "exactly 1 GB",
			bytes: 1024 * 1024 * 1024,
			want:  "1.0 GB",
		},
		{
			name:  "2.5 GB",
			bytes: int64(2.5 * 1024 * 1024 * 1024),
			want:  "2.5 GB",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatSize(tt.bytes)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFormatTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		types []string
		want  string
	}{
		{
			name:  "nil types",
			types: nil,
			want:  "unknown",
		},
		{
			name:  "empty types",
			types: []string{},
			want:  "unknown",
		},
		{
			name:  "single type",
			types: []string{"index"},
			want:  "index",
		},
		{
			name:  "multiple types",
			types: []string{"sbom", "provenance"},
			want:  "sbom, provenance",
		},
		{
			name:  "platform type",
			types: []string{"linux/amd64"},
			want:  "linux/amd64",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatTypes(tt.types)
			assert.Equal(t, tt.want, got)
		})
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
			assert.Equal(t, idPositions[0], idPositions[i], "version IDs not aligned: visual positions %v", idPositions)
		}
	}
}
