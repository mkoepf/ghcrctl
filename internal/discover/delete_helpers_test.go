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
