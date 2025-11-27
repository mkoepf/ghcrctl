package filter

import (
	"testing"
	"time"

	"github.com/mhk/ghcrctl/internal/gh"
	"github.com/stretchr/testify/assert"
)

// Helper to create test versions
func createTestVersion(id int64, tags []string, createdAt string) gh.PackageVersionInfo {
	return gh.PackageVersionInfo{
		ID:        id,
		Name:      "sha256:abc123",
		Tags:      tags,
		CreatedAt: createdAt,
		UpdatedAt: createdAt,
	}
}

// Helper to create test version with GitHub's actual date format
func createTestVersionGitHubFormat(id int64, tags []string, createdAt string) gh.PackageVersionInfo {
	return gh.PackageVersionInfo{
		ID:        id,
		Name:      "sha256:abc123",
		Tags:      tags,
		CreatedAt: createdAt, // GitHub format: "2006-01-02 15:04:05"
		UpdatedAt: createdAt,
	}
}

func TestVersionFilter_Apply_NoFilters(t *testing.T) {
	versions := []gh.PackageVersionInfo{
		createTestVersion(1, []string{"v1.0.0"}, "2025-01-01T00:00:00Z"),
		createTestVersion(2, []string{}, "2025-01-02T00:00:00Z"),
	}

	filter := &VersionFilter{}
	result := filter.Apply(versions)

	assert.Equal(t, 2, len(result))
}

func TestVersionFilter_Apply_OnlyTagged(t *testing.T) {
	versions := []gh.PackageVersionInfo{
		createTestVersion(1, []string{"v1.0.0"}, "2025-01-01T00:00:00Z"),
		createTestVersion(2, []string{}, "2025-01-02T00:00:00Z"),
		createTestVersion(3, []string{"v2.0.0", "latest"}, "2025-01-03T00:00:00Z"),
	}

	filter := &VersionFilter{OnlyTagged: true}
	result := filter.Apply(versions)

	assert.Equal(t, 2, len(result))
	assert.Equal(t, int64(1), result[0].ID)
	assert.Equal(t, int64(3), result[1].ID)
}

func TestVersionFilter_Apply_OnlyUntagged(t *testing.T) {
	versions := []gh.PackageVersionInfo{
		createTestVersion(1, []string{"v1.0.0"}, "2025-01-01T00:00:00Z"),
		createTestVersion(2, []string{}, "2025-01-02T00:00:00Z"),
		createTestVersion(3, []string{"v2.0.0"}, "2025-01-03T00:00:00Z"),
		createTestVersion(4, []string{}, "2025-01-04T00:00:00Z"),
	}

	filter := &VersionFilter{OnlyUntagged: true}
	result := filter.Apply(versions)

	assert.Equal(t, 2, len(result))
	assert.Equal(t, int64(2), result[0].ID)
	assert.Equal(t, int64(4), result[1].ID)
}

func TestVersionFilter_Apply_TagPattern_Simple(t *testing.T) {
	versions := []gh.PackageVersionInfo{
		createTestVersion(1, []string{"v1.0.0"}, "2025-01-01T00:00:00Z"),
		createTestVersion(2, []string{"v2.0.0"}, "2025-01-02T00:00:00Z"),
		createTestVersion(3, []string{"latest"}, "2025-01-03T00:00:00Z"),
		createTestVersion(4, []string{"v1.1.0"}, "2025-01-04T00:00:00Z"),
	}

	filter := &VersionFilter{TagPattern: "^v1\\..*"}
	result := filter.Apply(versions)

	assert.Equal(t, 2, len(result))
	assert.Equal(t, int64(1), result[0].ID)
	assert.Equal(t, int64(4), result[1].ID)
}

func TestVersionFilter_Apply_TagPattern_Multiple(t *testing.T) {
	versions := []gh.PackageVersionInfo{
		createTestVersion(1, []string{"latest"}, "2025-01-01T00:00:00Z"),
		createTestVersion(2, []string{"stable"}, "2025-01-02T00:00:00Z"),
		createTestVersion(3, []string{"v1.0.0"}, "2025-01-03T00:00:00Z"),
		createTestVersion(4, []string{"dev"}, "2025-01-04T00:00:00Z"),
	}

	filter := &VersionFilter{TagPattern: "^(latest|stable)$"}
	result := filter.Apply(versions)

	assert.Equal(t, 2, len(result))
	assert.Equal(t, int64(1), result[0].ID)
	assert.Equal(t, int64(2), result[1].ID)
}

func TestVersionFilter_Apply_TagPattern_MultipleTagsPerVersion(t *testing.T) {
	versions := []gh.PackageVersionInfo{
		createTestVersion(1, []string{"v1.0.0", "latest"}, "2025-01-01T00:00:00Z"),
		createTestVersion(2, []string{"v2.0.0", "stable"}, "2025-01-02T00:00:00Z"),
		createTestVersion(3, []string{"dev", "testing"}, "2025-01-03T00:00:00Z"),
	}

	// Should match version 1 because it has "latest"
	filter := &VersionFilter{TagPattern: "latest"}
	result := filter.Apply(versions)

	assert.Equal(t, 1, len(result))
	assert.Equal(t, int64(1), result[0].ID)
}

func TestVersionFilter_Apply_TagPattern_Invalid(t *testing.T) {
	versions := []gh.PackageVersionInfo{
		createTestVersion(1, []string{"v1.0.0"}, "2025-01-01T00:00:00Z"),
	}

	// Invalid regex pattern
	filter := &VersionFilter{TagPattern: "[invalid("}
	result := filter.Apply(versions)

	// Should return empty result on invalid pattern
	assert.Equal(t, 0, len(result))
}

func TestVersionFilter_Apply_OlderThan(t *testing.T) {
	versions := []gh.PackageVersionInfo{
		createTestVersion(1, []string{"v1"}, "2025-01-01T00:00:00Z"),
		createTestVersion(2, []string{"v2"}, "2025-01-05T00:00:00Z"),
		createTestVersion(3, []string{"v3"}, "2025-01-10T00:00:00Z"),
	}

	cutoff, _ := time.Parse(time.RFC3339, "2025-01-06T00:00:00Z")
	filter := &VersionFilter{OlderThan: cutoff}
	result := filter.Apply(versions)

	assert.Equal(t, 2, len(result))
	assert.Equal(t, int64(1), result[0].ID)
	assert.Equal(t, int64(2), result[1].ID)
}

func TestVersionFilter_Apply_NewerThan(t *testing.T) {
	versions := []gh.PackageVersionInfo{
		createTestVersion(1, []string{"v1"}, "2025-01-01T00:00:00Z"),
		createTestVersion(2, []string{"v2"}, "2025-01-05T00:00:00Z"),
		createTestVersion(3, []string{"v3"}, "2025-01-10T00:00:00Z"),
	}

	cutoff, _ := time.Parse(time.RFC3339, "2025-01-06T00:00:00Z")
	filter := &VersionFilter{NewerThan: cutoff}
	result := filter.Apply(versions)

	assert.Equal(t, 1, len(result))
	assert.Equal(t, int64(3), result[0].ID)
}

func TestVersionFilter_Apply_OlderThanDays(t *testing.T) {
	now := time.Now()
	versions := []gh.PackageVersionInfo{
		createTestVersion(1, []string{"v1"}, now.AddDate(0, 0, -10).Format(time.RFC3339)),
		createTestVersion(2, []string{"v2"}, now.AddDate(0, 0, -5).Format(time.RFC3339)),
		createTestVersion(3, []string{"v3"}, now.AddDate(0, 0, -1).Format(time.RFC3339)),
	}

	filter := &VersionFilter{OlderThanDays: 7}
	result := filter.Apply(versions)

	// Should return versions older than 7 days
	assert.Equal(t, 1, len(result))
	assert.Equal(t, int64(1), result[0].ID)
}

func TestVersionFilter_Apply_NewerThanDays(t *testing.T) {
	now := time.Now()
	versions := []gh.PackageVersionInfo{
		createTestVersion(1, []string{"v1"}, now.AddDate(0, 0, -10).Format(time.RFC3339)),
		createTestVersion(2, []string{"v2"}, now.AddDate(0, 0, -5).Format(time.RFC3339)),
		createTestVersion(3, []string{"v3"}, now.AddDate(0, 0, -1).Format(time.RFC3339)),
	}

	filter := &VersionFilter{NewerThanDays: 7}
	result := filter.Apply(versions)

	// Should return versions newer than 7 days
	assert.Equal(t, 2, len(result))
	assert.Equal(t, int64(2), result[0].ID)
	assert.Equal(t, int64(3), result[1].ID)
}

func TestVersionFilter_Apply_CombinedFilters(t *testing.T) {
	now := time.Now()
	versions := []gh.PackageVersionInfo{
		createTestVersion(1, []string{"v1.0.0"}, now.AddDate(0, 0, -10).Format(time.RFC3339)),
		createTestVersion(2, []string{}, now.AddDate(0, 0, -5).Format(time.RFC3339)),
		createTestVersion(3, []string{"v2.0.0"}, now.AddDate(0, 0, -5).Format(time.RFC3339)),
		createTestVersion(4, []string{"latest"}, now.AddDate(0, 0, -1).Format(time.RFC3339)),
	}

	// Untagged AND older than 7 days
	filter := &VersionFilter{
		OnlyUntagged:  true,
		OlderThanDays: 7,
	}
	result := filter.Apply(versions)

	assert.Equal(t, 0, len(result)) // No untagged version older than 7 days

	// Tagged AND matching pattern AND older than 7 days
	filter2 := &VersionFilter{
		OnlyTagged:    true,
		TagPattern:    "^v[0-9]",
		OlderThanDays: 7,
	}
	result2 := filter2.Apply(versions)

	assert.Equal(t, 1, len(result2))
	assert.Equal(t, int64(1), result2[0].ID)
}

func TestVersionFilter_Apply_InvalidDate(t *testing.T) {
	versions := []gh.PackageVersionInfo{
		createTestVersion(1, []string{"v1"}, "invalid-date"),
		createTestVersion(2, []string{"v2"}, "2025-01-05T00:00:00Z"),
	}

	cutoff, _ := time.Parse(time.RFC3339, "2025-01-06T00:00:00Z")
	filter := &VersionFilter{OlderThan: cutoff}
	result := filter.Apply(versions)

	// Should skip version with invalid date
	assert.Equal(t, 1, len(result))
	assert.Equal(t, int64(2), result[0].ID)
}

func TestVersionFilter_Apply_TagsFilter_Exact(t *testing.T) {
	versions := []gh.PackageVersionInfo{
		createTestVersion(1, []string{"v1.0.0"}, "2025-01-01T00:00:00Z"),
		createTestVersion(2, []string{"v2.0.0"}, "2025-01-02T00:00:00Z"),
		createTestVersion(3, []string{"latest"}, "2025-01-03T00:00:00Z"),
	}

	filter := &VersionFilter{Tags: []string{"latest"}}
	result := filter.Apply(versions)

	assert.Equal(t, 1, len(result))
	assert.Equal(t, int64(3), result[0].ID)
}

func TestVersionFilter_Apply_TagsFilter_Multiple(t *testing.T) {
	versions := []gh.PackageVersionInfo{
		createTestVersion(1, []string{"v1.0.0"}, "2025-01-01T00:00:00Z"),
		createTestVersion(2, []string{"v2.0.0"}, "2025-01-02T00:00:00Z"),
		createTestVersion(3, []string{"latest"}, "2025-01-03T00:00:00Z"),
		createTestVersion(4, []string{"stable"}, "2025-01-04T00:00:00Z"),
	}

	filter := &VersionFilter{Tags: []string{"latest", "stable"}}
	result := filter.Apply(versions)

	assert.Equal(t, 2, len(result))
	assert.Equal(t, int64(3), result[0].ID)
	assert.Equal(t, int64(4), result[1].ID)
}

func TestVersionFilter_Apply_GitHubDateFormat_NoFilters(t *testing.T) {
	// This test reproduces the real-world bug: GitHub returns dates as "2006-01-02 15:04:05"
	// When no date filters are active, versions with this format should still pass through
	versions := []gh.PackageVersionInfo{
		createTestVersionGitHubFormat(1, []string{"v1.0.0"}, "2025-01-01 10:30:45"),
		createTestVersionGitHubFormat(2, []string{}, "2025-01-02 14:20:10"),
	}

	filter := &VersionFilter{} // No filters at all
	result := filter.Apply(versions)

	// Should return both versions even though date format doesn't match RFC3339
	assert.Equal(t, 2, len(result), "All versions should pass when no filters are active")
}

func TestVersionFilter_Apply_GitHubDateFormat_TaggedOnly(t *testing.T) {
	// When using non-date filters with GitHub date format
	versions := []gh.PackageVersionInfo{
		createTestVersionGitHubFormat(1, []string{"v1.0.0"}, "2025-01-01 10:30:45"),
		createTestVersionGitHubFormat(2, []string{}, "2025-01-02 14:20:10"),
		createTestVersionGitHubFormat(3, []string{"v2.0.0"}, "2025-01-03 09:15:22"),
	}

	filter := &VersionFilter{OnlyTagged: true}
	result := filter.Apply(versions)

	// Should return tagged versions even with GitHub date format
	assert.Equal(t, 2, len(result))
	assert.Equal(t, int64(1), result[0].ID)
	assert.Equal(t, int64(3), result[1].ID)
}

func TestVersionFilter_Apply_VersionID(t *testing.T) {
	versions := []gh.PackageVersionInfo{
		createTestVersion(12345, []string{"v1.0.0"}, "2025-01-01T00:00:00Z"),
		createTestVersion(12346, []string{"v2.0.0"}, "2025-01-02T00:00:00Z"),
		createTestVersion(12347, []string{"latest"}, "2025-01-03T00:00:00Z"),
	}

	filter := &VersionFilter{VersionID: 12346}
	result := filter.Apply(versions)

	assert.Equal(t, 1, len(result))
	assert.Equal(t, int64(12346), result[0].ID)
}

func TestVersionFilter_Apply_VersionID_NotFound(t *testing.T) {
	versions := []gh.PackageVersionInfo{
		createTestVersion(12345, []string{"v1.0.0"}, "2025-01-01T00:00:00Z"),
		createTestVersion(12346, []string{"v2.0.0"}, "2025-01-02T00:00:00Z"),
	}

	filter := &VersionFilter{VersionID: 99999}
	result := filter.Apply(versions)

	assert.Equal(t, 0, len(result))
}

func TestVersionFilter_Apply_Digest(t *testing.T) {
	versions := []gh.PackageVersionInfo{
		{ID: 1, Name: "sha256:abc123", Tags: []string{"v1.0.0"}, CreatedAt: "2025-01-01T00:00:00Z"},
		{ID: 2, Name: "sha256:def456", Tags: []string{"v2.0.0"}, CreatedAt: "2025-01-02T00:00:00Z"},
		{ID: 3, Name: "sha256:ghi789", Tags: []string{"latest"}, CreatedAt: "2025-01-03T00:00:00Z"},
	}

	filter := &VersionFilter{Digest: "sha256:def456"}
	result := filter.Apply(versions)

	assert.Equal(t, 1, len(result))
	assert.Equal(t, int64(2), result[0].ID)
	assert.Equal(t, "sha256:def456", result[0].Name)
}

func TestVersionFilter_Apply_Digest_NotFound(t *testing.T) {
	versions := []gh.PackageVersionInfo{
		{ID: 1, Name: "sha256:abc123", Tags: []string{"v1.0.0"}, CreatedAt: "2025-01-01T00:00:00Z"},
		{ID: 2, Name: "sha256:def456", Tags: []string{"v2.0.0"}, CreatedAt: "2025-01-02T00:00:00Z"},
	}

	filter := &VersionFilter{Digest: "sha256:notexist"}
	result := filter.Apply(versions)

	assert.Equal(t, 0, len(result))
}

func TestVersionFilter_Apply_Digest_ShortForm(t *testing.T) {
	versions := []gh.PackageVersionInfo{
		{ID: 1, Name: "sha256:abc123def456789012345678901234567890123456789012345678901234", Tags: []string{"v1.0.0"}, CreatedAt: "2025-01-01T00:00:00Z"},
		{ID: 2, Name: "sha256:def456abc789012345678901234567890123456789012345678901234567", Tags: []string{"v2.0.0"}, CreatedAt: "2025-01-02T00:00:00Z"},
	}

	// Short form should match the beginning of the digest
	filter := &VersionFilter{Digest: "sha256:abc123"}
	result := filter.Apply(versions)

	assert.Equal(t, 1, len(result))
	assert.Equal(t, int64(1), result[0].ID)
}
