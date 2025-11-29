package discover

// CountImageMembership returns how many images (root artifacts) contain the given digest.
// An image is defined as a root version (index, standalone manifest) plus its children.
func CountImageMembership(versions map[string]VersionInfo, digest string) int {
	// Build list of all roots
	roots := findRoots(versions)

	count := 0
	for _, rootDigest := range roots {
		// Check if the digest is the root itself or a child of this root
		if rootDigest == digest {
			count++
			continue
		}

		// Check if digest is reachable from this root
		if isReachableFrom(versions, rootDigest, digest) {
			count++
		}
	}

	return count
}

// FindImageByDigest returns all versions that belong to the same image as the given digest.
// If the digest is a child, it finds the root first, then collects all connected versions.
func FindImageByDigest(versions map[string]VersionInfo, digest string) []VersionInfo {
	// Find the root for this digest
	rootDigest := findRootFor(versions, digest)
	if rootDigest == "" {
		// The digest itself might be a standalone version
		if v, ok := versions[digest]; ok {
			return []VersionInfo{v}
		}
		return nil
	}

	// Collect all versions reachable from the root
	return collectImageVersions(versions, rootDigest)
}

// CollectDeletionOrder returns version IDs in the order they should be deleted.
// Children are deleted before the root. Shared versions (referenced by multiple roots) are skipped.
func CollectDeletionOrder(versions map[string]VersionInfo, rootDigest string) []int64 {
	root, ok := versions[rootDigest]
	if !ok {
		return nil
	}

	// Find all children
	children := collectChildren(versions, rootDigest)

	// Identify shared children (referenced by multiple roots)
	sharedDigests := make(map[string]bool)
	for _, child := range children {
		if CountImageMembership(versions, child.Digest) > 1 {
			sharedDigests[child.Digest] = true
		}
	}

	// Collect IDs: attestations first, then platforms, then root
	// Skip shared versions
	var attestationIDs, platformIDs []int64
	for _, child := range children {
		if sharedDigests[child.Digest] {
			continue // Skip shared
		}
		if child.IsReferrer() {
			attestationIDs = append(attestationIDs, child.ID)
		} else {
			platformIDs = append(platformIDs, child.ID)
		}
	}

	ids := make([]int64, 0, len(attestationIDs)+len(platformIDs)+1)
	ids = append(ids, attestationIDs...)
	ids = append(ids, platformIDs...)
	ids = append(ids, root.ID)

	return ids
}

// findRoots returns all root digests in the version map.
func findRoots(versions map[string]VersionInfo) []string {
	var roots []string
	for digest, v := range versions {
		if v.IsRoot(versions) {
			roots = append(roots, digest)
		}
	}
	return roots
}

// findRootFor finds the root digest for a given version.
// Returns the digest itself if it's a root, or the parent root otherwise.
func findRootFor(versions map[string]VersionInfo, digest string) string {
	v, ok := versions[digest]
	if !ok {
		return ""
	}

	// If this is a root, return it
	if v.IsRoot(versions) {
		return digest
	}

	// Follow incoming refs to find the root
	for _, inDigest := range v.IncomingRefs {
		if parent, ok := versions[inDigest]; ok {
			if parent.IsRoot(versions) {
				return inDigest
			}
			// Recursively search for root
			if root := findRootFor(versions, inDigest); root != "" {
				return root
			}
		}
	}

	return digest // No parent found, treat as standalone
}

// isReachableFrom checks if targetDigest is reachable from sourceDigest via outgoing refs.
func isReachableFrom(versions map[string]VersionInfo, sourceDigest, targetDigest string) bool {
	source, ok := versions[sourceDigest]
	if !ok {
		return false
	}

	visited := make(map[string]bool)
	return isReachableHelper(versions, source.OutgoingRefs, targetDigest, visited)
}

func isReachableHelper(versions map[string]VersionInfo, outRefs []string, target string, visited map[string]bool) bool {
	for _, ref := range outRefs {
		if ref == target {
			return true
		}
		if visited[ref] {
			continue
		}
		visited[ref] = true
		if v, ok := versions[ref]; ok {
			if isReachableHelper(versions, v.OutgoingRefs, target, visited) {
				return true
			}
		}
	}
	return false
}

// collectImageVersions collects all versions reachable from a root.
func collectImageVersions(versions map[string]VersionInfo, rootDigest string) []VersionInfo {
	visited := make(map[string]bool)
	var result []VersionInfo

	var collect func(digest string)
	collect = func(digest string) {
		if visited[digest] {
			return
		}
		visited[digest] = true

		v, ok := versions[digest]
		if !ok {
			return
		}
		result = append(result, v)

		// Follow outgoing refs
		for _, outDigest := range v.OutgoingRefs {
			collect(outDigest)
		}
	}

	collect(rootDigest)
	return result
}

// collectChildren collects all child versions of a root.
func collectChildren(versions map[string]VersionInfo, rootDigest string) []VersionInfo {
	all := collectImageVersions(versions, rootDigest)
	var children []VersionInfo
	for _, v := range all {
		if v.Digest != rootDigest {
			children = append(children, v)
		}
	}
	return children
}

// ToMap converts a slice of VersionInfo to a map keyed by digest.
func ToMap(versions []VersionInfo) map[string]VersionInfo {
	m := make(map[string]VersionInfo, len(versions))
	for _, v := range versions {
		m[v.Digest] = v
	}
	return m
}

// CountImageMembershipByID returns how many images contain the given version ID.
// This is a convenience wrapper around CountImageMembership that looks up the digest first.
func CountImageMembershipByID(versions map[string]VersionInfo, versionID int64) int {
	// Find the digest for this version ID
	var digest string
	for d, v := range versions {
		if v.ID == versionID {
			digest = d
			break
		}
	}
	if digest == "" {
		return 0
	}
	return CountImageMembership(versions, digest)
}
