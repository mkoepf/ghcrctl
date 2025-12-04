//go:build !mutating

package discover

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Integration tests for discover package functions that call OCI registries.
// These tests require GITHUB_TOKEN and will skip if not set.

// =============================================================================
// ResolveVersionInfo Integration Tests
// =============================================================================

func TestResolveVersionInfo_Integration_Index(t *testing.T) {
	t.Parallel()
	if os.Getenv("GITHUB_TOKEN") == "" {
		t.Skip("Skipping integration test - GITHUB_TOKEN not set")
	}

	ctx := context.Background()
	resolver := newOrasResolver()

	// Use the test image with SBOM - it's a multi-arch index
	image := "ghcr.io/mkoepf/ghcrctl-test-with-sbom"

	// First resolve the "latest" tag to get the index digest
	digest, err := ResolveTag(ctx, image, "latest")
	require.NoError(t, err, "Failed to resolve tag")

	// Now resolve the version info for the index
	types, size, err := resolver.resolveVersionInfo(ctx, image, digest)
	require.NoError(t, err, "ResolveVersionInfo failed")

	// Multi-arch image should be an index
	require.NotEmpty(t, types, "Expected at least one type")
	assert.Contains(t, types, "index", "Expected 'index' type for multi-arch image")

	// Size should be reasonable (index manifests are typically small)
	assert.Greater(t, size, int64(0), "Expected positive size")
	t.Logf("Index digest: %s, types: %v, size: %d", digest, types, size)
}

func TestResolveVersionInfo_Integration_Platform(t *testing.T) {
	t.Parallel()
	if os.Getenv("GITHUB_TOKEN") == "" {
		t.Skip("Skipping integration test - GITHUB_TOKEN not set")
	}

	ctx := context.Background()
	resolver := newOrasResolver()

	// Use the single-platform test image (no SBOM)
	image := "ghcr.io/mkoepf/ghcrctl-test-no-sbom"

	// Resolve the "latest" tag
	digest, err := ResolveTag(ctx, image, "latest")
	require.NoError(t, err, "Failed to resolve tag")

	types, size, err := resolver.resolveVersionInfo(ctx, image, digest)
	require.NoError(t, err, "ResolveVersionInfo failed")

	require.NotEmpty(t, types, "Expected at least one type")

	// Single platform image should have os/arch type (e.g., "linux/amd64")
	typ := types[0]
	validType := strings.Contains(typ, "/") || typ == "index" || typ == "manifest"
	assert.True(t, validType, "Expected platform type (os/arch) or index/manifest, got: %s", typ)

	assert.Greater(t, size, int64(0), "Expected positive size")
	t.Logf("Platform digest: %s, types: %v, size: %d", digest, types, size)
}

func TestResolveVersionInfo_Integration_InvalidDigest(t *testing.T) {
	t.Parallel()
	if os.Getenv("GITHUB_TOKEN") == "" {
		t.Skip("Skipping integration test - GITHUB_TOKEN not set")
	}

	ctx := context.Background()
	resolver := newOrasResolver()

	image := "ghcr.io/mkoepf/ghcrctl-test-with-sbom"
	// Valid format but non-existent digest
	invalidDigest := "sha256:0000000000000000000000000000000000000000000000000000000000000000"

	_, _, err := resolver.resolveVersionInfo(ctx, image, invalidDigest)
	assert.Error(t, err, "Expected error for non-existent digest")
}

// =============================================================================
// GetArtifactContent Integration Tests
// =============================================================================

func TestGetArtifactContent_Integration_SBOM(t *testing.T) {
	t.Parallel()
	if os.Getenv("GITHUB_TOKEN") == "" {
		t.Skip("Skipping integration test - GITHUB_TOKEN not set")
	}

	ctx := context.Background()
	image := "ghcr.io/mkoepf/ghcrctl-test-with-sbom"

	// First, we need to find an SBOM artifact digest
	// Resolve the main image tag
	mainDigest, err := ResolveTag(ctx, image, "latest")
	require.NoError(t, err, "Failed to resolve tag")

	// Use the discoverer to find SBOM artifacts
	resolver := newOrasResolver()
	discoverer := &PackageDiscoverer{
		resolver:        resolver,
		childDiscoverer: &orasChildDiscoverer{resolver: resolver},
	}

	// We need version info to discover - create a minimal version list
	versions := []struct {
		ID        int64
		Name      string
		Tags      []string
		CreatedAt string
	}{
		{ID: 1, Name: mainDigest, Tags: []string{"latest"}, CreatedAt: "2025-01-01T00:00:00Z"},
	}

	// Convert to PackageVersionInfo format
	var pkgVersions []struct {
		ID        int64
		Name      string
		Tags      []string
		CreatedAt string
	}
	for _, v := range versions {
		pkgVersions = append(pkgVersions, v)
	}

	// Discover children of the main image
	children, err := discoverer.childDiscoverer.discoverChildren(ctx, image, mainDigest, nil)
	require.NoError(t, err, "Failed to discover children")

	// Find an SBOM among the children
	var sbomDigest string
	for _, childDigest := range children {
		types, _, err := resolver.resolveVersionInfo(ctx, image, childDigest)
		if err != nil {
			continue
		}
		for _, typ := range types {
			if typ == "sbom" {
				sbomDigest = childDigest
				break
			}
		}
		if sbomDigest != "" {
			break
		}
	}

	if sbomDigest == "" {
		t.Skip("No SBOM artifact found in test image - skipping content fetch test")
	}

	// Now fetch the SBOM content
	content, err := GetArtifactContent(ctx, image, sbomDigest)
	require.NoError(t, err, "GetArtifactContent failed")
	assert.NotNil(t, content, "Expected non-nil content")

	t.Logf("Successfully fetched SBOM content from digest: %s", sbomDigest[:19])
}

func TestGetArtifactContent_Integration_NonExistent(t *testing.T) {
	t.Parallel()
	if os.Getenv("GITHUB_TOKEN") == "" {
		t.Skip("Skipping integration test - GITHUB_TOKEN not set")
	}

	ctx := context.Background()
	image := "ghcr.io/mkoepf/ghcrctl-test-with-sbom"
	// Valid format but non-existent digest
	invalidDigest := "sha256:0000000000000000000000000000000000000000000000000000000000000000"

	_, err := GetArtifactContent(ctx, image, invalidDigest)
	assert.Error(t, err, "Expected error for non-existent digest")
}

// =============================================================================
// ResolveTag Integration Tests
// =============================================================================

func TestResolveTag_Integration(t *testing.T) {
	t.Parallel()
	if os.Getenv("GITHUB_TOKEN") == "" {
		t.Skip("Skipping integration test - GITHUB_TOKEN not set")
	}

	ctx := context.Background()
	image := "ghcr.io/mkoepf/ghcrctl-test-with-sbom"

	digest, err := ResolveTag(ctx, image, "latest")
	require.NoError(t, err, "ResolveTag failed")

	// Should return a valid sha256 digest
	assert.True(t, strings.HasPrefix(digest, "sha256:"), "Expected sha256 digest, got: %s", digest)
	assert.Len(t, digest, 71, "Expected digest length 71") // "sha256:" (7) + 64 hex chars

	t.Logf("Resolved 'latest' to: %s", digest)
}

func TestResolveTag_Integration_NonExistent(t *testing.T) {
	t.Parallel()
	if os.Getenv("GITHUB_TOKEN") == "" {
		t.Skip("Skipping integration test - GITHUB_TOKEN not set")
	}

	ctx := context.Background()
	image := "ghcr.io/mkoepf/ghcrctl-test-with-sbom"

	_, err := ResolveTag(ctx, image, "nonexistent-tag-12345")
	assert.Error(t, err, "Expected error for non-existent tag")
}

// =============================================================================
// GetImageConfig Integration Tests
// =============================================================================

func TestGetImageConfig_Integration_MultiArch(t *testing.T) {
	t.Parallel()
	if os.Getenv("GITHUB_TOKEN") == "" {
		t.Skip("Skipping integration test - GITHUB_TOKEN not set")
	}

	ctx := context.Background()
	image := "ghcr.io/mkoepf/ghcrctl-test-with-sbom"

	// Resolve the "latest" tag to get the index digest
	digest, err := ResolveTag(ctx, image, "latest")
	require.NoError(t, err, "Failed to resolve tag")

	// Fetch the image config - this exercises the Image Index code path
	config, err := GetImageConfig(ctx, image, digest)
	require.NoError(t, err, "GetImageConfig failed")
	require.NotNil(t, config, "Expected non-nil config")

	// Verify config has expected fields
	assert.NotEmpty(t, config.OS, "Expected config.OS to be set")
	assert.NotEmpty(t, config.Architecture, "Expected config.Architecture to be set")

	t.Logf("Multi-arch config: OS=%s, Arch=%s", config.OS, config.Architecture)
}

func TestGetImageConfig_Integration_SingleArch(t *testing.T) {
	t.Parallel()
	if os.Getenv("GITHUB_TOKEN") == "" {
		t.Skip("Skipping integration test - GITHUB_TOKEN not set")
	}

	ctx := context.Background()
	image := "ghcr.io/mkoepf/ghcrctl-test-no-sbom"

	// Resolve the "latest" tag
	digest, err := ResolveTag(ctx, image, "latest")
	require.NoError(t, err, "Failed to resolve tag")

	// Fetch the image config - this exercises the simple manifest code path
	config, err := GetImageConfig(ctx, image, digest)
	require.NoError(t, err, "GetImageConfig failed")
	require.NotNil(t, config, "Expected non-nil config")

	// Verify config has expected fields
	assert.NotEmpty(t, config.OS, "Expected config.OS to be set")
	assert.NotEmpty(t, config.Architecture, "Expected config.Architecture to be set")

	t.Logf("Single-arch config: OS=%s, Arch=%s", config.OS, config.Architecture)
}

func TestGetImageConfig_Integration_NonExistent(t *testing.T) {
	t.Parallel()
	if os.Getenv("GITHUB_TOKEN") == "" {
		t.Skip("Skipping integration test - GITHUB_TOKEN not set")
	}

	ctx := context.Background()
	image := "ghcr.io/mkoepf/ghcrctl-test-with-sbom"
	// Valid format but non-existent digest
	invalidDigest := "sha256:0000000000000000000000000000000000000000000000000000000000000000"

	_, err := GetImageConfig(ctx, image, invalidDigest)
	assert.Error(t, err, "Expected error for non-existent digest")
	assert.ErrorContains(t, err, "failed to resolve digest")
}
