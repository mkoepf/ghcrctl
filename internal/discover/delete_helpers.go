package discover

// ClassifyImageVersions separates image versions into exclusive (to delete) and shared (to preserve).
// A version is shared if it has incoming refs from outside the image being deleted.
func ClassifyImageVersions(imageVersions []VersionInfo) (toDelete, shared []VersionInfo) {
	// Build set of digests in this image
	imageDigests := make(map[string]bool)
	for _, v := range imageVersions {
		imageDigests[v.Digest] = true
	}

	for _, v := range imageVersions {
		// Check if any incoming ref is from outside this image
		isShared := false
		for _, inRef := range v.IncomingRefs {
			if !imageDigests[inRef] {
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

// FindImageByDigest returns all versions reachable from a root digest via OutgoingRefs.
func FindImageByDigest(versions map[string]VersionInfo, rootDigest string) []VersionInfo {
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
