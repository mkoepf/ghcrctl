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

	// Date filtering (absolute time or relative via ParseDateOrDuration)
	OlderThan time.Time // Include versions created before this time
	NewerThan time.Time // Include versions created after this time

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

	// Apply filters
	result := []gh.PackageVersionInfo{}
	for _, ver := range versions {
		if !f.matchesVersion(ver, tagRegex) {
			continue
		}
		result = append(result, ver)
	}

	return result
}

// matchesVersion checks if a single version matches all filter criteria
func (f *VersionFilter) matchesVersion(ver gh.PackageVersionInfo, tagRegex *regexp.Regexp) bool {
	// Check version ID filter (exact match)
	if f.VersionID != 0 && ver.ID != f.VersionID {
		return false
	}

	// Check digest filter (prefix matching for short digests)
	// Supports both "sha256:abc123" and "abc123" (as shown in DIGEST column)
	if f.Digest != "" && !matchesDigest(ver.Digest, f.Digest) {
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
	needsDateCheck := !f.OlderThan.IsZero() || !f.NewerThan.IsZero()

	if needsDateCheck {
		// Parse creation time - try multiple formats
		createdAt, err := ParseDate(ver.CreatedAt)
		if err != nil {
			// Skip versions with invalid dates only when date filtering is active
			return false
		}

		// Check date filters
		if !f.OlderThan.IsZero() && !createdAt.Before(f.OlderThan) {
			return false
		}
		if !f.NewerThan.IsZero() && !createdAt.After(f.NewerThan) {
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

// ParseDateOrDuration parses a string as either a date or duration.
// Dates are detected by starting with 4 digits followed by '-' (e.g., "2025-01-15").
// Durations support Go's time.ParseDuration units (h, m, s, ms, us, ns) plus 'd' for days.
// For durations, returns the cutoff time relative to now (now - duration).
//
// Examples:
//   - "2025-01-15" -> parsed as date
//   - "2025-01-15T10:30:00Z" -> parsed as RFC3339 date
//   - "7d" -> 7 days ago
//   - "24h" -> 24 hours ago
//   - "30m" -> 30 minutes ago
//   - "1h30m" -> 1 hour 30 minutes ago
//   - "2d12h" -> 2 days and 12 hours ago
func ParseDateOrDuration(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, fmt.Errorf("date/duration string cannot be empty")
	}

	// Detect date format: starts with 4 digits followed by '-'
	if len(s) >= 5 && isDigit(s[0]) && isDigit(s[1]) && isDigit(s[2]) && isDigit(s[3]) && s[4] == '-' {
		return ParseDate(s)
	}

	// Parse as duration
	return parseDuration(s)
}

// parseDuration parses a duration string with support for 'd' (days).
// Returns the cutoff time (now - duration).
func parseDuration(s string) (time.Time, error) {
	now := time.Now()

	// Check for day component (e.g., "7d" or "2d12h")
	if idx := strings.Index(s, "d"); idx != -1 {
		// Extract days part
		daysStr := s[:idx]
		days, err := parsePositiveInt(daysStr)
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid duration %q: %w", s, err)
		}

		// Get remainder after 'd'
		remainder := s[idx+1:]
		result := now.AddDate(0, 0, -days)

		// If there's more after 'd', parse it as standard duration
		if remainder != "" {
			d, err := time.ParseDuration(remainder)
			if err != nil {
				return time.Time{}, fmt.Errorf("invalid duration %q: %w", s, err)
			}
			result = result.Add(-d)
		}

		return result, nil
	}

	// No 'd', parse as standard Go duration
	d, err := time.ParseDuration(s)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid duration %q (supported: Nd, Nh, Nm, Ns or combinations like 1h30m)", s)
	}

	return now.Add(-d), nil
}

// isDigit returns true if b is an ASCII digit.
func isDigit(b byte) bool {
	return b >= '0' && b <= '9'
}

// parsePositiveInt parses a string as a positive integer.
func parsePositiveInt(s string) (int, error) {
	if s == "" {
		return 0, fmt.Errorf("empty number")
	}
	var n int
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("invalid number %q", s)
		}
		n = n*10 + int(c-'0')
	}
	return n, nil
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
