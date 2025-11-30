package discover

import (
	"testing"
)

func TestToMap(t *testing.T) {
	t.Parallel()

	versions := []VersionInfo{
		{ID: 1, Digest: "sha256:abc"},
		{ID: 2, Digest: "sha256:def"},
		{ID: 3, Digest: "sha256:ghi"},
	}

	m := ToMap(versions)

	if len(m) != 3 {
		t.Errorf("Expected 3 entries in map, got %d", len(m))
	}

	if m["sha256:abc"].ID != 1 {
		t.Error("Expected ID 1 for sha256:abc")
	}
	if m["sha256:def"].ID != 2 {
		t.Error("Expected ID 2 for sha256:def")
	}
}

func TestFindImageByDigest(t *testing.T) {
	t.Parallel()

	versions := map[string]VersionInfo{
		"sha256:index1": {
			ID:           1,
			Digest:       "sha256:index1",
			Types:        []string{"index"},
			OutgoingRefs: []string{"sha256:platform1", "sha256:sbom1"},
		},
		"sha256:platform1": {
			ID:           2,
			Digest:       "sha256:platform1",
			Types:        []string{"linux/amd64"},
			IncomingRefs: []string{"sha256:index1"},
		},
		"sha256:sbom1": {
			ID:           3,
			Digest:       "sha256:sbom1",
			Types:        []string{"sbom"},
			IncomingRefs: []string{"sha256:index1"},
		},
		"sha256:standalone": {
			ID:     4,
			Digest: "sha256:standalone",
			Types:  []string{"manifest"},
		},
	}

	// Find image by root digest
	image := FindImageByDigest(versions, "sha256:index1")
	if len(image) != 3 {
		t.Errorf("Expected 3 versions in image, got %d", len(image))
	}

	// Verify all expected digests are present
	digestSet := make(map[string]bool)
	for _, v := range image {
		digestSet[v.Digest] = true
	}
	if !digestSet["sha256:index1"] || !digestSet["sha256:platform1"] || !digestSet["sha256:sbom1"] {
		t.Errorf("Missing expected digests in image: %v", image)
	}

	// Find standalone version
	image = FindImageByDigest(versions, "sha256:standalone")
	if len(image) != 1 {
		t.Errorf("Expected 1 version for standalone, got %d", len(image))
	}

	// Find nonexistent digest returns empty
	image = FindImageByDigest(versions, "sha256:nonexistent")
	if len(image) != 0 {
		t.Errorf("Expected 0 versions for nonexistent digest, got %d", len(image))
	}
}

func TestClassifyImageVersions(t *testing.T) {
	t.Parallel()

	// All versions in one image, no external refs
	imageVersions := []VersionInfo{
		{
			ID:           100,
			Digest:       "sha256:index1",
			Types:        []string{"index"},
			Tags:         []string{"latest"},
			OutgoingRefs: []string{"sha256:platform1", "sha256:sbom1"},
		},
		{
			ID:           200,
			Digest:       "sha256:platform1",
			Types:        []string{"linux/amd64"},
			IncomingRefs: []string{"sha256:index1"},
		},
		{
			ID:           300,
			Digest:       "sha256:sbom1",
			Types:        []string{"sbom"},
			IncomingRefs: []string{"sha256:index1"},
		},
	}

	toDelete, shared := ClassifyImageVersions(imageVersions)

	// All versions should be exclusive (no sharing)
	if len(toDelete) != 3 {
		t.Errorf("Expected 3 versions to delete, got %d", len(toDelete))
	}
	if len(shared) != 0 {
		t.Errorf("Expected 0 shared versions, got %d", len(shared))
	}
}

func TestClassifyImageVersions_WithShared(t *testing.T) {
	t.Parallel()

	// Versions for index1's image, but shared-platform has external ref to index2
	imageVersions := []VersionInfo{
		{
			ID:           1,
			Digest:       "sha256:index1",
			Types:        []string{"index"},
			Tags:         []string{"v1.0"},
			OutgoingRefs: []string{"sha256:shared-platform", "sha256:exclusive-sbom"},
		},
		{
			ID:           3,
			Digest:       "sha256:shared-platform",
			Types:        []string{"linux/amd64"},
			IncomingRefs: []string{"sha256:index1", "sha256:index2"}, // index2 is external
		},
		{
			ID:           4,
			Digest:       "sha256:exclusive-sbom",
			Types:        []string{"sbom"},
			IncomingRefs: []string{"sha256:index1"},
		},
	}

	toDelete, shared := ClassifyImageVersions(imageVersions)

	// 2 exclusive (index1 + exclusive-sbom), 1 shared (shared-platform)
	if len(toDelete) != 2 {
		t.Errorf("Expected 2 versions to delete, got %d", len(toDelete))
	}
	if len(shared) != 1 {
		t.Errorf("Expected 1 shared version, got %d", len(shared))
	}

	// Verify shared version is the platform
	if len(shared) > 0 && shared[0].ID != 3 {
		t.Errorf("Expected shared version to be ID 3, got %d", shared[0].ID)
	}
}

func TestClassifyImageVersions_ExternalRefCount(t *testing.T) {
	t.Parallel()

	// Version with multiple external refs
	imageVersions := []VersionInfo{
		{
			ID:           1,
			Digest:       "sha256:index1",
			Types:        []string{"index"},
			OutgoingRefs: []string{"sha256:shared"},
		},
		{
			ID:           2,
			Digest:       "sha256:shared",
			Types:        []string{"linux/amd64"},
			IncomingRefs: []string{"sha256:index1", "sha256:ext1", "sha256:ext2", "sha256:ext3"},
		},
	}

	toDelete, shared := ClassifyImageVersions(imageVersions)

	if len(toDelete) != 1 {
		t.Errorf("Expected 1 version to delete, got %d", len(toDelete))
	}
	if len(shared) != 1 {
		t.Errorf("Expected 1 shared version, got %d", len(shared))
	}

	// Verify we can count external refs from the shared version
	if len(shared) > 0 {
		externalCount := 0
		imageDigests := map[string]bool{"sha256:index1": true, "sha256:shared": true}
		for _, inRef := range shared[0].IncomingRefs {
			if !imageDigests[inRef] {
				externalCount++
			}
		}
		if externalCount != 3 {
			t.Errorf("Expected 3 external refs, got %d", externalCount)
		}
	}
}

func TestFindImagesContainingVersion(t *testing.T) {
	t.Parallel()

	// Two images sharing a platform
	versions := map[string]VersionInfo{
		"sha256:index1": {
			ID:           1,
			Digest:       "sha256:index1",
			Types:        []string{"index"},
			OutgoingRefs: []string{"sha256:shared-platform"},
		},
		"sha256:index2": {
			ID:           2,
			Digest:       "sha256:index2",
			Types:        []string{"index"},
			OutgoingRefs: []string{"sha256:shared-platform", "sha256:exclusive"},
		},
		"sha256:shared-platform": {
			ID:           3,
			Digest:       "sha256:shared-platform",
			Types:        []string{"linux/amd64"},
			IncomingRefs: []string{"sha256:index1", "sha256:index2"},
		},
		"sha256:exclusive": {
			ID:           4,
			Digest:       "sha256:exclusive",
			Types:        []string{"linux/arm64"},
			IncomingRefs: []string{"sha256:index2"},
		},
	}

	// Find images containing shared platform - should return both images
	result := FindImagesContainingVersion(versions, "sha256:shared-platform")
	if len(result) != 4 {
		t.Errorf("Expected 4 versions (both images), got %d", len(result))
	}

	// Find images containing exclusive platform - should return only index2's image
	result = FindImagesContainingVersion(versions, "sha256:exclusive")
	if len(result) != 3 {
		t.Errorf("Expected 3 versions (index2 image only), got %d", len(result))
	}

	// Verify index2's image contains the right versions
	digestSet := make(map[string]bool)
	for _, v := range result {
		digestSet[v.Digest] = true
	}
	if !digestSet["sha256:index2"] || !digestSet["sha256:shared-platform"] || !digestSet["sha256:exclusive"] {
		t.Errorf("Missing expected digests: %v", digestSet)
	}
	if digestSet["sha256:index1"] {
		t.Errorf("Should not contain index1")
	}

	// Find images containing a root - should return just that image
	result = FindImagesContainingVersion(versions, "sha256:index1")
	if len(result) != 2 {
		t.Errorf("Expected 2 versions (index1 + shared-platform), got %d", len(result))
	}

	// Find nonexistent digest returns nil
	result = FindImagesContainingVersion(versions, "sha256:nonexistent")
	if len(result) != 0 {
		t.Errorf("Expected 0 versions for nonexistent, got %d", len(result))
	}
}

func TestCanReach(t *testing.T) {
	t.Parallel()

	versions := map[string]VersionInfo{
		"sha256:root": {
			Digest:       "sha256:root",
			Types:        []string{"index"},
			OutgoingRefs: []string{"sha256:child1", "sha256:child2"},
		},
		"sha256:child1": {
			Digest:       "sha256:child1",
			Types:        []string{"linux/amd64"},
			IncomingRefs: []string{"sha256:root"},
			OutgoingRefs: []string{"sha256:grandchild"},
		},
		"sha256:child2": {
			Digest:       "sha256:child2",
			Types:        []string{"linux/arm64"},
			IncomingRefs: []string{"sha256:root"},
		},
		"sha256:grandchild": {
			Digest:       "sha256:grandchild",
			Types:        []string{"sbom"},
			IncomingRefs: []string{"sha256:child1"},
		},
	}

	// Can reach self
	if !canReach(versions, "sha256:root", "sha256:root") {
		t.Error("Should be able to reach self")
	}

	// Can reach direct child
	if !canReach(versions, "sha256:root", "sha256:child1") {
		t.Error("Should be able to reach direct child")
	}

	// Can reach grandchild
	if !canReach(versions, "sha256:root", "sha256:grandchild") {
		t.Error("Should be able to reach grandchild")
	}

	// Cannot reach nonexistent
	if canReach(versions, "sha256:root", "sha256:nonexistent") {
		t.Error("Should not be able to reach nonexistent")
	}

	// Child cannot reach sibling (no path)
	if canReach(versions, "sha256:child1", "sha256:child2") {
		t.Error("Child1 should not be able to reach child2")
	}
}

func TestFindDigestByShortDigest(t *testing.T) {
	t.Parallel()

	versions := map[string]VersionInfo{
		"sha256:abcdef123456789012345678901234567890123456789012345678901234": {
			ID:     1,
			Digest: "sha256:abcdef123456789012345678901234567890123456789012345678901234",
		},
		"sha256:123456abcdef789012345678901234567890123456789012345678901234": {
			ID:     2,
			Digest: "sha256:123456abcdef789012345678901234567890123456789012345678901234",
		},
	}

	// Find by full digest
	result, err := FindDigestByShortDigest(versions, "sha256:abcdef123456789012345678901234567890123456789012345678901234")
	if err != nil {
		t.Errorf("Expected no error for full digest, got %v", err)
	}
	if result != "sha256:abcdef123456789012345678901234567890123456789012345678901234" {
		t.Errorf("Expected full digest match, got %s", result)
	}

	// Find by short digest (12 chars)
	result, err = FindDigestByShortDigest(versions, "abcdef123456")
	if err != nil {
		t.Errorf("Expected no error for short digest, got %v", err)
	}
	if result != "sha256:abcdef123456789012345678901234567890123456789012345678901234" {
		t.Errorf("Expected first digest, got %s", result)
	}

	// Find by short digest with sha256: prefix
	result, err = FindDigestByShortDigest(versions, "sha256:123456ab")
	if err != nil {
		t.Errorf("Expected no error for short digest with prefix, got %v", err)
	}
	if result != "sha256:123456abcdef789012345678901234567890123456789012345678901234" {
		t.Errorf("Expected second digest, got %s", result)
	}

	// Error for nonexistent digest
	_, err = FindDigestByShortDigest(versions, "zzz999")
	if err == nil {
		t.Error("Expected error for nonexistent digest")
	}

	// Error for ambiguous short digest (both start with similar prefixes)
	versionsAmbiguous := map[string]VersionInfo{
		"sha256:abc123456789012345678901234567890123456789012345678901234567": {
			ID:     1,
			Digest: "sha256:abc123456789012345678901234567890123456789012345678901234567",
		},
		"sha256:abc123999999012345678901234567890123456789012345678901234567": {
			ID:     2,
			Digest: "sha256:abc123999999012345678901234567890123456789012345678901234567",
		},
	}
	_, err = FindDigestByShortDigest(versionsAmbiguous, "abc123")
	if err == nil {
		t.Error("Expected error for ambiguous short digest")
	}
}
