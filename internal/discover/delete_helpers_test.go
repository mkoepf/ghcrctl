package discover

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToMap(t *testing.T) {
	t.Parallel()

	versions := []VersionInfo{
		{ID: 1, Digest: "sha256:abc"},
		{ID: 2, Digest: "sha256:def"},
		{ID: 3, Digest: "sha256:ghi"},
	}

	m := ToMap(versions)

	assert.Len(t, m, 3)
	assert.Equal(t, int64(1), m["sha256:abc"].ID)
	assert.Equal(t, int64(2), m["sha256:def"].ID)
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
	assert.Len(t, image, 3)

	// Verify all expected digests are present
	digestSet := make(map[string]bool)
	for _, v := range image {
		digestSet[v.Digest] = true
	}
	assert.True(t, digestSet["sha256:index1"], "missing sha256:index1")
	assert.True(t, digestSet["sha256:platform1"], "missing sha256:platform1")
	assert.True(t, digestSet["sha256:sbom1"], "missing sha256:sbom1")

	// Find standalone version
	image = FindImageByDigest(versions, "sha256:standalone")
	assert.Len(t, image, 1)

	// Find nonexistent digest returns empty
	image = FindImageByDigest(versions, "sha256:nonexistent")
	assert.Empty(t, image)
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
	assert.Len(t, toDelete, 3)
	assert.Empty(t, shared)
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
	assert.Len(t, toDelete, 2)
	assert.Len(t, shared, 1)

	// Verify shared version is the platform
	require.NotEmpty(t, shared)
	assert.Equal(t, int64(3), shared[0].ID)
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

	assert.Len(t, toDelete, 1)
	assert.Len(t, shared, 1)

	// Verify we can count external refs from the shared version
	require.NotEmpty(t, shared)
	externalCount := 0
	imageDigests := map[string]bool{"sha256:index1": true, "sha256:shared": true}
	for _, inRef := range shared[0].IncomingRefs {
		if !imageDigests[inRef] {
			externalCount++
		}
	}
	assert.Equal(t, 3, externalCount)
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
	assert.Len(t, result, 4)

	// Find images containing exclusive platform - should return only index2's image
	result = FindImagesContainingVersion(versions, "sha256:exclusive")
	assert.Len(t, result, 3)

	// Verify index2's image contains the right versions
	digestSet := make(map[string]bool)
	for _, v := range result {
		digestSet[v.Digest] = true
	}
	assert.True(t, digestSet["sha256:index2"], "missing sha256:index2")
	assert.True(t, digestSet["sha256:shared-platform"], "missing sha256:shared-platform")
	assert.True(t, digestSet["sha256:exclusive"], "missing sha256:exclusive")
	assert.False(t, digestSet["sha256:index1"], "should not contain index1")

	// Find images containing a root - should return just that image
	result = FindImagesContainingVersion(versions, "sha256:index1")
	assert.Len(t, result, 2)

	// Find nonexistent digest returns nil
	result = FindImagesContainingVersion(versions, "sha256:nonexistent")
	assert.Empty(t, result)
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
	assert.True(t, canReach(versions, "sha256:root", "sha256:root"), "should reach self")

	// Can reach direct child
	assert.True(t, canReach(versions, "sha256:root", "sha256:child1"), "should reach direct child")

	// Can reach grandchild
	assert.True(t, canReach(versions, "sha256:root", "sha256:grandchild"), "should reach grandchild")

	// Cannot reach nonexistent
	assert.False(t, canReach(versions, "sha256:root", "sha256:nonexistent"), "should not reach nonexistent")

	// Child cannot reach sibling (no path)
	assert.False(t, canReach(versions, "sha256:child1", "sha256:child2"), "child1 should not reach child2")
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
	require.NoError(t, err)
	assert.Equal(t, "sha256:abcdef123456789012345678901234567890123456789012345678901234", result)

	// Find by short digest (12 chars)
	result, err = FindDigestByShortDigest(versions, "abcdef123456")
	require.NoError(t, err)
	assert.Equal(t, "sha256:abcdef123456789012345678901234567890123456789012345678901234", result)

	// Find by short digest with sha256: prefix
	result, err = FindDigestByShortDigest(versions, "sha256:123456ab")
	require.NoError(t, err)
	assert.Equal(t, "sha256:123456abcdef789012345678901234567890123456789012345678901234", result)

	// Error for nonexistent digest
	_, err = FindDigestByShortDigest(versions, "zzz999")
	assert.Error(t, err)

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
	assert.Error(t, err)
}

func TestFindDigestByVersionID(t *testing.T) {
	t.Parallel()

	versions := map[string]VersionInfo{
		"sha256:abcdef123456789012345678901234567890123456789012345678901234": {
			ID:     12345,
			Digest: "sha256:abcdef123456789012345678901234567890123456789012345678901234",
		},
		"sha256:123456abcdef789012345678901234567890123456789012345678901234": {
			ID:     67890,
			Digest: "sha256:123456abcdef789012345678901234567890123456789012345678901234",
		},
	}

	// Find by existing version ID
	result, err := FindDigestByVersionID(versions, 12345)
	require.NoError(t, err)
	assert.Equal(t, "sha256:abcdef123456789012345678901234567890123456789012345678901234", result)

	// Find second version ID
	result, err = FindDigestByVersionID(versions, 67890)
	require.NoError(t, err)
	assert.Equal(t, "sha256:123456abcdef789012345678901234567890123456789012345678901234", result)

	// Error for nonexistent version ID
	_, err = FindDigestByVersionID(versions, 99999)
	assert.Error(t, err)
}
