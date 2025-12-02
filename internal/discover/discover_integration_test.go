package discover

import (
	"context"
	"os"
	"strings"
	"testing"
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
	resolver := NewOrasResolver()

	// Use the test image with SBOM - it's a multi-arch index
	image := "ghcr.io/mkoepf/ghcrctl-test-with-sbom"

	// First resolve the "latest" tag to get the index digest
	digest, err := ResolveTag(ctx, image, "latest")
	if err != nil {
		t.Fatalf("Failed to resolve tag: %v", err)
	}

	// Now resolve the version info for the index
	types, size, err := resolver.ResolveVersionInfo(ctx, image, digest)
	if err != nil {
		t.Fatalf("ResolveVersionInfo failed: %v", err)
	}

	// Multi-arch image should be an index
	if len(types) == 0 {
		t.Error("Expected at least one type")
	}

	found := false
	for _, typ := range types {
		if typ == "index" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected 'index' type for multi-arch image, got: %v", types)
	}

	// Size should be reasonable (index manifests are typically small)
	if size <= 0 {
		t.Errorf("Expected positive size, got: %d", size)
	}
	t.Logf("Index digest: %s, types: %v, size: %d", digest, types, size)
}

func TestResolveVersionInfo_Integration_Platform(t *testing.T) {
	t.Parallel()
	if os.Getenv("GITHUB_TOKEN") == "" {
		t.Skip("Skipping integration test - GITHUB_TOKEN not set")
	}

	ctx := context.Background()
	resolver := NewOrasResolver()

	// Use the single-platform test image (no SBOM)
	image := "ghcr.io/mkoepf/ghcrctl-test-no-sbom"

	// Resolve the "latest" tag
	digest, err := ResolveTag(ctx, image, "latest")
	if err != nil {
		t.Fatalf("Failed to resolve tag: %v", err)
	}

	types, size, err := resolver.ResolveVersionInfo(ctx, image, digest)
	if err != nil {
		t.Fatalf("ResolveVersionInfo failed: %v", err)
	}

	if len(types) == 0 {
		t.Error("Expected at least one type")
	}

	// Single platform image should have os/arch type (e.g., "linux/amd64")
	typ := types[0]
	if !strings.Contains(typ, "/") && typ != "index" && typ != "manifest" {
		t.Errorf("Expected platform type (os/arch) or index/manifest, got: %s", typ)
	}

	if size <= 0 {
		t.Errorf("Expected positive size, got: %d", size)
	}
	t.Logf("Platform digest: %s, types: %v, size: %d", digest, types, size)
}

func TestResolveVersionInfo_Integration_InvalidDigest(t *testing.T) {
	t.Parallel()
	if os.Getenv("GITHUB_TOKEN") == "" {
		t.Skip("Skipping integration test - GITHUB_TOKEN not set")
	}

	ctx := context.Background()
	resolver := NewOrasResolver()

	image := "ghcr.io/mkoepf/ghcrctl-test-with-sbom"
	// Valid format but non-existent digest
	invalidDigest := "sha256:0000000000000000000000000000000000000000000000000000000000000000"

	_, _, err := resolver.ResolveVersionInfo(ctx, image, invalidDigest)
	if err == nil {
		t.Error("Expected error for non-existent digest")
	}
}

// =============================================================================
// FetchArtifactContent Integration Tests
// =============================================================================

func TestFetchArtifactContent_Integration_SBOM(t *testing.T) {
	t.Parallel()
	if os.Getenv("GITHUB_TOKEN") == "" {
		t.Skip("Skipping integration test - GITHUB_TOKEN not set")
	}

	ctx := context.Background()
	image := "ghcr.io/mkoepf/ghcrctl-test-with-sbom"

	// First, we need to find an SBOM artifact digest
	// Resolve the main image tag
	mainDigest, err := ResolveTag(ctx, image, "latest")
	if err != nil {
		t.Fatalf("Failed to resolve tag: %v", err)
	}

	// Use the discoverer to find SBOM artifacts
	resolver := NewOrasResolver()
	discoverer := &PackageDiscoverer{
		Resolver:        resolver,
		ChildDiscoverer: &OrasChildDiscoverer{resolver: resolver},
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
	children, err := discoverer.ChildDiscoverer.DiscoverChildren(ctx, image, mainDigest, nil)
	if err != nil {
		t.Fatalf("Failed to discover children: %v", err)
	}

	// Find an SBOM among the children
	var sbomDigest string
	for _, childDigest := range children {
		types, _, err := resolver.ResolveVersionInfo(ctx, image, childDigest)
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
	content, err := FetchArtifactContent(ctx, image, sbomDigest)
	if err != nil {
		t.Fatalf("FetchArtifactContent failed: %v", err)
	}

	if content == nil {
		t.Error("Expected non-nil content")
	}

	t.Logf("Successfully fetched SBOM content from digest: %s", sbomDigest[:19])
}

func TestFetchArtifactContent_Integration_NonExistent(t *testing.T) {
	t.Parallel()
	if os.Getenv("GITHUB_TOKEN") == "" {
		t.Skip("Skipping integration test - GITHUB_TOKEN not set")
	}

	ctx := context.Background()
	image := "ghcr.io/mkoepf/ghcrctl-test-with-sbom"
	// Valid format but non-existent digest
	invalidDigest := "sha256:0000000000000000000000000000000000000000000000000000000000000000"

	_, err := FetchArtifactContent(ctx, image, invalidDigest)
	if err == nil {
		t.Error("Expected error for non-existent digest")
	}
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
	if err != nil {
		t.Fatalf("ResolveTag failed: %v", err)
	}

	// Should return a valid sha256 digest
	if !strings.HasPrefix(digest, "sha256:") {
		t.Errorf("Expected sha256 digest, got: %s", digest)
	}

	if len(digest) != 71 { // "sha256:" (7) + 64 hex chars
		t.Errorf("Expected digest length 71, got: %d", len(digest))
	}

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
	if err == nil {
		t.Error("Expected error for non-existent tag")
	}
}
