package discover

import (
	"testing"
)

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

func TestCollectDeletionOrder(t *testing.T) {
	t.Parallel()

	versions := map[string]VersionInfo{
		"sha256:index1": {
			ID:           100,
			Digest:       "sha256:index1",
			Types:        []string{"index"},
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

	ids := CollectDeletionOrder(versions, "sha256:index1")

	// Should have 3 IDs
	if len(ids) != 3 {
		t.Fatalf("Expected 3 version IDs, got %d", len(ids))
	}

	// Root should be last
	if ids[len(ids)-1] != 100 {
		t.Errorf("Expected root (ID 100) to be last, got %d", ids[len(ids)-1])
	}

	// Children should be before root
	childIDs := make(map[int64]bool)
	for _, id := range ids[:len(ids)-1] {
		childIDs[id] = true
	}
	if !childIDs[200] || !childIDs[300] {
		t.Errorf("Expected child IDs 200 and 300 before root, got %v", ids)
	}
}

func TestCollectDeletionOrder_SkipsShared(t *testing.T) {
	t.Parallel()

	// Two indexes share a platform
	versions := map[string]VersionInfo{
		"sha256:index1": {
			ID:           1,
			Digest:       "sha256:index1",
			Types:        []string{"index"},
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

	ids := CollectDeletionOrder(versions, "sha256:index1")

	// Should have 2 IDs: exclusive-sbom + root (shared-platform skipped)
	if len(ids) != 2 {
		t.Fatalf("Expected 2 version IDs (shared skipped), got %d: %v", len(ids), ids)
	}

	// Should NOT contain the shared platform
	for _, id := range ids {
		if id == 3 {
			t.Error("Shared platform (ID 3) should not be in deletion order")
		}
	}

	// Should contain root and exclusive sbom
	idSet := make(map[int64]bool)
	for _, id := range ids {
		idSet[id] = true
	}
	if !idSet[1] || !idSet[4] {
		t.Errorf("Expected IDs 1 (root) and 4 (exclusive sbom), got %v", ids)
	}
}
