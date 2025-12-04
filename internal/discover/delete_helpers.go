package discover

import "fmt"

// ClassifyGraphVersions separates graph versions into exclusive (to delete) and shared (to preserve).
// A version is shared if it has incoming refs from outside the graph being deleted.
func ClassifyGraphVersions(graphVersions []VersionInfo) (toDelete, shared []VersionInfo) {
	// Build set of digests in this graph
	graphDigests := make(map[string]bool)
	for _, v := range graphVersions {
		graphDigests[v.Digest] = true
	}

	for _, v := range graphVersions {
		// Check if any incoming ref is from outside this graph
		isShared := false
		for _, inRef := range v.IncomingRefs {
			if !graphDigests[inRef] {
				isShared = true
				break
			}
		}
		if isShared {
			shared = append(shared, v)
		} else {
			toDelete = append(toDelete, v)
		}
	}
	return
}

// FindGraphByDigest returns all versions reachable from a root digest via OutgoingRefs.
func FindGraphByDigest(versions map[string]VersionInfo, rootDigest string) []VersionInfo {
	visited := make(map[string]bool)
	var result []VersionInfo

	var collect func(digest string)
	collect = func(digest string) {
		if visited[digest] {
			return
		}
		visited[digest] = true

		if v, ok := versions[digest]; ok {
			result = append(result, v)
			for _, out := range v.OutgoingRefs {
				collect(out)
			}
		}
	}
	collect(rootDigest)
	return result
}

// ToMap converts a slice of VersionInfo to a map keyed by digest.
func ToMap(versions []VersionInfo) map[string]VersionInfo {
	m := make(map[string]VersionInfo, len(versions))
	for _, v := range versions {
		m[v.Digest] = v
	}
	return m
}

// FindGraphsContainingVersion returns all versions that belong to graphs containing the target digest.
// It finds the root(s) that can reach the target and returns all versions in those graphs.
func FindGraphsContainingVersion(versions map[string]VersionInfo, targetDigest string) []VersionInfo {
	// Find all roots that can reach the target
	roots := findRootsContaining(versions, targetDigest)
	if len(roots) == 0 {
		return nil
	}

	// Collect all versions from all graphs containing the target
	seen := make(map[string]bool)
	var result []VersionInfo
	for _, rootDigest := range roots {
		for _, v := range FindGraphByDigest(versions, rootDigest) {
			if !seen[v.Digest] {
				seen[v.Digest] = true
				result = append(result, v)
			}
		}
	}
	return result
}

// findRootsContaining finds all root digests whose graphs contain the target digest.
func findRootsContaining(versions map[string]VersionInfo, targetDigest string) []string {
	// If target doesn't exist, return empty
	if _, exists := versions[targetDigest]; !exists {
		return nil
	}

	// Find all roots (versions with no incoming refs or type "index")
	var roots []string
	for _, v := range versions {
		if v.IsRoot(versions) {
			roots = append(roots, v.Digest)
		}
	}

	// Check which roots can reach the target
	var containingRoots []string
	for _, rootDigest := range roots {
		if canReach(versions, rootDigest, targetDigest) {
			containingRoots = append(containingRoots, rootDigest)
		}
	}
	return containingRoots
}

// FindDigestByShortDigest finds a full digest from a short or full digest input.
// It supports full digests (sha256:abc...), short digests without prefix (abc123...),
// and short digests with prefix (sha256:abc...). Returns error if not found or ambiguous.
func FindDigestByShortDigest(versions map[string]VersionInfo, input string) (string, error) {
	// Check for exact match first
	if _, exists := versions[input]; exists {
		return input, nil
	}

	// Normalize: remove sha256: prefix for comparison
	shortInput := input
	if len(input) > 7 && input[:7] == "sha256:" {
		shortInput = input[7:]
	}

	// Find all matching digests
	var matches []string
	for digest := range versions {
		// Get the hash part without prefix
		hashPart := digest
		if len(digest) > 7 && digest[:7] == "sha256:" {
			hashPart = digest[7:]
		}

		// Check if hash starts with input
		if len(hashPart) >= len(shortInput) && hashPart[:len(shortInput)] == shortInput {
			matches = append(matches, digest)
		}
	}

	if len(matches) == 0 {
		return "", fmt.Errorf("digest %s not found", input)
	}
	if len(matches) > 1 {
		return "", fmt.Errorf("digest %s is ambiguous, matches %d versions", input, len(matches))
	}
	return matches[0], nil
}

// FindDigestByVersionID finds a full digest from a version ID.
func FindDigestByVersionID(versions map[string]VersionInfo, versionID int64) (string, error) {
	for _, v := range versions {
		if v.ID == versionID {
			return v.Digest, nil
		}
	}
	return "", fmt.Errorf("version ID %d not found", versionID)
}

// canReach checks if we can reach targetDigest starting from startDigest via OutgoingRefs.
func canReach(versions map[string]VersionInfo, startDigest, targetDigest string) bool {
	if startDigest == targetDigest {
		return true
	}

	visited := make(map[string]bool)
	var search func(digest string) bool
	search = func(digest string) bool {
		if visited[digest] {
			return false
		}
		visited[digest] = true

		if digest == targetDigest {
			return true
		}

		if v, ok := versions[digest]; ok {
			for _, out := range v.OutgoingRefs {
				if search(out) {
					return true
				}
			}
		}
		return false
	}
	return search(startDigest)
}
