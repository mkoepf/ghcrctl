// Package filter provides version filtering capabilities for GHCR package versions.
// It supports filtering by tags (exact match and regex), date ranges, age,
// version IDs, digests, and tagged/untagged status with graph-aware filtering.
package filter

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/mkoepf/ghcrctl/internal/gh"
)

// VersionFilter defines filtering criteria for package versions
type VersionFilter struct {
	// Tag filtering
	Tags       []string // Exact tag matches (OR logic)
	TagPattern string   // Regex pattern for tag matching

	// Tagged/Untagged filtering
	OnlyTagged   bool
	OnlyUntagged bool

	// Graph-aware filtering for OnlyUntagged
	// When OnlyUntagged is true, versions in this set are excluded
	// (they belong to tagged graphs and shouldn't be shown as "untagged")
	TaggedGraphMembers map[int64]bool

	// Date filtering
	OlderThan time.Time // Include versions created before this time
	NewerThan time.Time // Include versions created after this time

	// Age-based filtering (relative to current time)
	OlderThanDays int // Include versions older than N days
	NewerThanDays int // Include versions newer than N days

	// Direct version filtering
	VersionID int64  // Filter by exact version ID (0 means no filter)
	Digest    string // Filter by digest (supports prefix matching)
}

// Apply applies all configured filters to the provided versions
// Filters are combined with AND logic (all must match)
// Returns a new slice with filtered versions
func (f *VersionFilter) Apply(versions []gh.PackageVersionInfo) []gh.PackageVersionInfo {
	if f == nil {
		return versions
	}

	// Compile regex pattern if provided
	var tagRegex *regexp.Regexp
	if f.TagPattern != "" {
		var err error
		tagRegex, err = regexp.Compile(f.TagPattern)
		if err != nil {
			// Return empty on invalid pattern
			return []gh.PackageVersionInfo{}
		}
	}

	// Calculate cutoff times for age-based filters
	now := time.Now()
	var olderThanTime, newerThanTime time.Time

	if f.OlderThanDays > 0 {
		olderThanTime = now.AddDate(0, 0, -f.OlderThanDays)
	}
	if f.NewerThanDays > 0 {
		newerThanTime = now.AddDate(0, 0, -f.NewerThanDays)
	}

	// Apply filters
	result := []gh.PackageVersionInfo{}
	for _, ver := range versions {
		if !f.matchesVersion(ver, tagRegex, olderThanTime, newerThanTime) {
			continue
		}
		result = append(result, ver)
	}

	return result
}

// matchesVersion checks if a single version matches all filter criteria
func (f *VersionFilter) matchesVersion(ver gh.PackageVersionInfo, tagRegex *regexp.Regexp, olderThanTime, newerThanTime time.Time) bool {
	// Check version ID filter (exact match)
	if f.VersionID != 0 && ver.ID != f.VersionID {
		return false
	}

	// Check digest filter (prefix matching for short digests)
	// Supports both "sha256:abc123" and "abc123" (as shown in DIGEST column)
	if f.Digest != "" && !matchesDigest(ver.Name, f.Digest) {
		return false
	}

	// Check tagged/untagged filter
	hasTag := len(ver.Tags) > 0
	if f.OnlyTagged && !hasTag {
		return false
	}
	if f.OnlyUntagged {
		// Exclude versions that have tags
		if hasTag {
			return false
		}
		// Exclude versions that belong to tagged graphs (children of tagged versions)
		if f.TaggedGraphMembers != nil && f.TaggedGraphMembers[ver.ID] {
			return false
		}
	}

	// Check exact tag match (OR logic for multiple tags)
	if len(f.Tags) > 0 {
		if !hasMatchingTag(ver.Tags, f.Tags) {
			return false
		}
	}

	// Check tag pattern match
	if tagRegex != nil {
		if !hasMatchingTagPattern(ver.Tags, tagRegex) {
			return false
		}
	}

	// Only parse dates if we actually need to check date filters
	needsDateCheck := !f.OlderThan.IsZero() || !f.NewerThan.IsZero() ||
		!olderThanTime.IsZero() || !newerThanTime.IsZero()

	if needsDateCheck {
		// Parse creation time - try multiple formats
		createdAt, err := ParseDate(ver.CreatedAt)
		if err != nil {
			// Skip versions with invalid dates only when date filtering is active
			return false
		}

		// Check absolute date filters
		if !f.OlderThan.IsZero() && !createdAt.Before(f.OlderThan) {
			return false
		}
		if !f.NewerThan.IsZero() && !createdAt.After(f.NewerThan) {
			return false
		}

		// Check age-based filters
		if !olderThanTime.IsZero() && !createdAt.Before(olderThanTime) {
			return false
		}
		if !newerThanTime.IsZero() && !createdAt.After(newerThanTime) {
			return false
		}
	}

	return true
}

// ParseDate attempts to parse a date string in multiple formats.
// Supported formats:
//   - "2006-01-02" (date only, most convenient for CLI)
//   - "2006-01-02T15:04:05Z07:00" (RFC3339)
//   - "2006-01-02T15:04:05.999999999Z07:00" (RFC3339Nano)
//   - "2006-01-02 15:04:05" (GitHub API format)
//   - "2006-01-02T15:04:05" (datetime without timezone)
func ParseDate(dateStr string) (time.Time, error) {
	if dateStr == "" {
		return time.Time{}, fmt.Errorf("date string cannot be empty")
	}

	formats := []string{
		"2006-01-02",          // Date only (most convenient for CLI)
		time.RFC3339,          // 2006-01-02T15:04:05Z07:00
		time.RFC3339Nano,      // With fractional seconds
		"2006-01-02 15:04:05", // GitHub API format
		"2006-01-02T15:04:05", // Without timezone
	}

	for _, format := range formats {
		t, err := time.Parse(format, dateStr)
		if err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse date %q (supported formats: YYYY-MM-DD, RFC3339)", dateStr)
}

// hasMatchingTag checks if any version tag matches any filter tag (exact match)
func hasMatchingTag(versionTags []string, filterTags []string) bool {
	for _, vTag := range versionTags {
		for _, fTag := range filterTags {
			if vTag == fTag {
				return true
			}
		}
	}
	return false
}

// hasMatchingTagPattern checks if any version tag matches the regex pattern
func hasMatchingTagPattern(versionTags []string, pattern *regexp.Regexp) bool {
	for _, tag := range versionTags {
		if pattern.MatchString(tag) {
			return true
		}
	}
	return false
}

// matchesDigest checks if a version digest matches the filter digest.
// Supports both full format "sha256:abc123..." and short format "abc123..."
// (as displayed in the DIGEST column without the sha256: prefix).
func matchesDigest(versionDigest, filterDigest string) bool {
	// Try direct prefix match first (handles "sha256:abc123" format)
	if strings.HasPrefix(versionDigest, filterDigest) {
		return true
	}

	// If filter doesn't have sha256: prefix, try matching against the hash part
	if !strings.HasPrefix(filterDigest, "sha256:") {
		hashPart := strings.TrimPrefix(versionDigest, "sha256:")
		return strings.HasPrefix(hashPart, filterDigest)
	}

	return false
}
