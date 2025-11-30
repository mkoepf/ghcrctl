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

func TestCountImageMembershipByID(t *testing.T) {
	t.Parallel()

	versions := map[string]VersionInfo{
		"sha256:index1": {
			ID:           1,
			Digest:       "sha256:index1",
			Types:        []string{"index"},
			OutgoingRefs: []string{"sha256:platform1"},
		},
		"sha256:platform1": {
			ID:           2,
			Digest:       "sha256:platform1",
			Types:        []string{"linux/amd64"},
			IncomingRefs: []string{"sha256:index1"},
		},
	}

	// Version ID 1 (index) belongs to 1 image
	count := CountImageMembershipByID(versions, 1)
	if count != 1 {
		t.Errorf("Expected index to belong to 1 image, got %d", count)
	}

	// Version ID 2 (platform) belongs to 1 image
	count = CountImageMembershipByID(versions, 2)
	if count != 1 {
		t.Errorf("Expected platform to belong to 1 image, got %d", count)
	}

	// Version ID 999 (nonexistent) belongs to 0 images
	count = CountImageMembershipByID(versions, 999)
	if count != 0 {
		t.Errorf("Expected nonexistent version to belong to 0 images, got %d", count)
	}
}

func TestCountImageMembership(t *testing.T) {
	t.Parallel()

	// Create a version map with one index and its children
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
	}

	// Platform belongs to one image
	count := CountImageMembership(versions, "sha256:platform1")
	if count != 1 {
		t.Errorf("Expected platform to belong to 1 image, got %d", count)
	}

	// SBOM belongs to one image
	count = CountImageMembership(versions, "sha256:sbom1")
	if count != 1 {
		t.Errorf("Expected sbom to belong to 1 image, got %d", count)
	}

	// Index itself is the root of one image
	count = CountImageMembership(versions, "sha256:index1")
	if count != 1 {
		t.Errorf("Expected index to belong to 1 image, got %d", count)
	}
}

func TestCountImageMembership_Shared(t *testing.T) {
	t.Parallel()

	// Create two indexes that share a platform
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
			OutgoingRefs: []string{"sha256:shared-platform"},
		},
		"sha256:shared-platform": {
			ID:           3,
			Digest:       "sha256:shared-platform",
			Types:        []string{"linux/amd64"},
			IncomingRefs: []string{"sha256:index1", "sha256:index2"},
		},
	}

	// Shared platform belongs to two images
	count := CountImageMembership(versions, "sha256:shared-platform")
	if count != 2 {
		t.Errorf("Expected shared platform to belong to 2 images, got %d", count)
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

	// Find image by index digest
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

	// Find image by child digest should still find the full image
	image = FindImageByDigest(versions, "sha256:platform1")
	if len(image) != 3 {
		t.Errorf("Expected 3 versions when finding by child digest, got %d", len(image))
	}

	// Find standalone version (not connected to any image)
	image = FindImageByDigest(versions, "sha256:standalone")
	if len(image) != 1 {
		t.Errorf("Expected 1 version for standalone, got %d", len(image))
	}
}

func TestClassifyImageVersions(t *testing.T) {
	t.Parallel()

	versionMap := map[string]VersionInfo{
		"sha256:index1": {
			ID:           100,
			Digest:       "sha256:index1",
			Types:        []string{"index"},
			Tags:         []string{"latest"},
			OutgoingRefs: []string{"sha256:platform1", "sha256:sbom1"},
		},
		"sha256:platform1": {
			ID:           200,
			Digest:       "sha256:platform1",
			Types:        []string{"linux/amd64"},
			IncomingRefs: []string{"sha256:index1"},
		},
		"sha256:sbom1": {
			ID:           300,
			Digest:       "sha256:sbom1",
			Types:        []string{"sbom"},
			IncomingRefs: []string{"sha256:index1"},
		},
	}

	imageVersions := []VersionInfo{
		versionMap["sha256:index1"],
		versionMap["sha256:platform1"],
		versionMap["sha256:sbom1"],
	}

	toDelete, shared := ClassifyImageVersions(imageVersions, versionMap)

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

	// Two indexes share a platform
	versionMap := map[string]VersionInfo{
		"sha256:index1": {
			ID:           1,
			Digest:       "sha256:index1",
			Types:        []string{"index"},
			Tags:         []string{"v1.0"},
			OutgoingRefs: []string{"sha256:shared-platform", "sha256:exclusive-sbom"},
		},
		"sha256:index2": {
			ID:           2,
			Digest:       "sha256:index2",
			Types:        []string{"index"},
			OutgoingRefs: []string{"sha256:shared-platform"},
		},
		"sha256:shared-platform": {
			ID:           3,
			Digest:       "sha256:shared-platform",
			Types:        []string{"linux/amd64"},
			IncomingRefs: []string{"sha256:index1", "sha256:index2"},
		},
		"sha256:exclusive-sbom": {
			ID:           4,
			Digest:       "sha256:exclusive-sbom",
			Types:        []string{"sbom"},
			IncomingRefs: []string{"sha256:index1"},
		},
	}

	// Versions for index1's image
	imageVersions := []VersionInfo{
		versionMap["sha256:index1"],
		versionMap["sha256:shared-platform"],
		versionMap["sha256:exclusive-sbom"],
	}

	toDelete, shared := ClassifyImageVersions(imageVersions, versionMap)

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
