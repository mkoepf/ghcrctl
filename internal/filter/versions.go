package filter

import (
	"regexp"
	"time"

	"github.com/mhk/ghcrctl/internal/gh"
)

// VersionFilter defines filtering criteria for package versions
type VersionFilter struct {
	// Tag filtering
	Tags       []string // Exact tag matches (OR logic)
	TagPattern string   // Regex pattern for tag matching

	// Tagged/Untagged filtering
	OnlyTagged   bool
	OnlyUntagged bool

	// Date filtering
	OlderThan time.Time // Include versions created before this time
	NewerThan time.Time // Include versions created after this time

	// Age-based filtering (relative to current time)
	OlderThanDays int // Include versions older than N days
	NewerThanDays int // Include versions newer than N days
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
	// Check tagged/untagged filter
	hasTag := len(ver.Tags) > 0
	if f.OnlyTagged && !hasTag {
		return false
	}
	if f.OnlyUntagged && hasTag {
		return false
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
		createdAt, err := parseDate(ver.CreatedAt)
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

// parseDate attempts to parse a date string in multiple formats
// GitHub API returns dates in format "2006-01-02 15:04:05"
func parseDate(dateStr string) (time.Time, error) {
	// Try GitHub format first (most common)
	t, err := time.Parse("2006-01-02 15:04:05", dateStr)
	if err == nil {
		return t, nil
	}

	// Try RFC3339 format (for tests and potential API changes)
	t, err = time.Parse(time.RFC3339, dateStr)
	if err == nil {
		return t, nil
	}

	// Try RFC3339 with fractional seconds
	t, err = time.Parse(time.RFC3339Nano, dateStr)
	if err == nil {
		return t, nil
	}

	return time.Time{}, err
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
