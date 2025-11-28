package oras

import (
	"context"
	"os"
	"strings"
	"testing"
)

// Integration tests that require GITHUB_TOKEN and access to real GHCR images
// These tests will be skipped if GITHUB_TOKEN is not set

// TestAuthWithValidToken verifies that a valid GITHUB_TOKEN allows access to test images
func TestAuthWithValidToken(t *testing.T) {
	t.Parallel()
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		t.Skip("Skipping integration test - GITHUB_TOKEN not set")
	}

	ctx := context.Background()

	// Use a known test image from the ghcrctl repository
	// This assumes the integration test workflow has created this image
	testImage := os.Getenv("TEST_IMAGE")
	if testImage == "" {
		// Default to the test image that should be created by the workflow
		testImage = "ghcr.io/mkoepf/ghcrctl-test-with-sbom"
	}

	// Test basic tag resolution - this requires authentication to work
	digest, err := ResolveTag(ctx, testImage, "latest")
	if err != nil {
		t.Fatalf("Failed to resolve tag with valid token: %v", err)
	}

	// Verify we got a valid digest back
	if !strings.HasPrefix(digest, "sha256:") {
		t.Errorf("Expected digest to start with 'sha256:', got '%s'", digest)
	}

	if len(digest) != 71 {
		t.Errorf("Expected digest length 71, got %d", len(digest))
	}

	t.Logf("Successfully resolved tag 'latest' to digest: %s", digest)
}

// TestAuthRegistryAccess verifies that ORAS can authenticate to ghcr.io
// and access the referrers API
func TestAuthRegistryAccess(t *testing.T) {
	t.Parallel()
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		t.Skip("Skipping integration test - GITHUB_TOKEN not set")
	}

	ctx := context.Background()

	// Use the test image
	testImage := os.Getenv("TEST_IMAGE")
	if testImage == "" {
		testImage = "ghcr.io/mkoepf/ghcrctl-test-with-sbom"
	}

	// First resolve the tag to get a digest
	digest, err := ResolveTag(ctx, testImage, "latest")
	if err != nil {
		t.Fatalf("Failed to resolve tag: %v", err)
	}

	// Now try to discover children - this tests the full authentication flow
	// including access to the manifest API
	children, err := DiscoverChildren(ctx, testImage, digest, nil)
	if err != nil {
		t.Fatalf("Failed to discover children with valid token: %v", err)
	}

	// We should get a non-nil slice (may be empty if image has no children)
	if children == nil {
		t.Error("Expected non-nil children slice")
	}

	t.Logf("Successfully accessed manifest API, found %d children", len(children))

	// Log what we found for debugging
	for _, child := range children {
		t.Logf("  Found child: type=%s, digest=%s", child.Type.DisplayType(), child.Digest)
	}
}

// TestAuthWithoutToken verifies graceful handling when no token is provided
func TestAuthWithoutToken(t *testing.T) {
	// Save original token
	originalToken := os.Getenv("GITHUB_TOKEN")
	defer func() {
		if originalToken != "" {
			os.Setenv("GITHUB_TOKEN", originalToken)
		}
	}()

	// Unset the token
	os.Unsetenv("GITHUB_TOKEN")

	ctx := context.Background()

	// Try to access a public image (if any exist)
	// Note: ghcr.io typically requires authentication even for public images
	testImage := "ghcr.io/mkoepf/ghcrctl-test-with-sbom"

	// This may fail or succeed depending on whether the image is public
	// We're mainly testing that the code doesn't panic
	_, err := ResolveTag(ctx, testImage, "latest")

	// We expect this to either work (public image) or fail gracefully
	if err != nil {
		t.Logf("Expected behavior: tag resolution failed without token: %v", err)
		// This is acceptable - the test passes as long as we don't panic
	} else {
		t.Logf("Tag resolution succeeded without token (image may be public)")
	}
}

// ===[ Tag Resolution Integration Tests ]===

// TestResolveTagLatest verifies resolving "latest" tag returns valid sha256 digest
func TestResolveTagLatest(t *testing.T) {
	t.Parallel()
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		t.Skip("Skipping integration test - GITHUB_TOKEN not set")
	}

	ctx := context.Background()
	testImage := "ghcr.io/mkoepf/ghcrctl-test-with-sbom"

	digest, err := ResolveTag(ctx, testImage, "latest")
	if err != nil {
		t.Fatalf("Failed to resolve 'latest' tag: %v", err)
	}

	if !strings.HasPrefix(digest, "sha256:") {
		t.Errorf("Expected digest to start with 'sha256:', got '%s'", digest)
	}

	if len(digest) != 71 {
		t.Errorf("Expected digest length 71, got %d", len(digest))
	}

	t.Logf("Resolved 'latest' to: %s", digest)
}

// TestResolveTagSemanticVersion verifies resolving semantic version tag
func TestResolveTagSemanticVersion(t *testing.T) {
	t.Parallel()
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		t.Skip("Skipping integration test - GITHUB_TOKEN not set")
	}

	ctx := context.Background()
	testImage := "ghcr.io/mkoepf/ghcrctl-test-with-sbom"

	// Resolve v1.0 tag
	digestV1, err := ResolveTag(ctx, testImage, "v1.0")
	if err != nil {
		t.Fatalf("Failed to resolve 'v1.0' tag: %v", err)
	}

	if !strings.HasPrefix(digestV1, "sha256:") {
		t.Errorf("Expected digest to start with 'sha256:', got '%s'", digestV1)
	}

	// Resolve latest tag
	digestLatest, err := ResolveTag(ctx, testImage, "latest")
	if err != nil {
		t.Fatalf("Failed to resolve 'latest' tag: %v", err)
	}

	// They should both be valid digests (may or may not be the same)
	t.Logf("Resolved 'v1.0' to: %s", digestV1)
	t.Logf("Resolved 'latest' to: %s", digestLatest)

	if digestV1 == digestLatest {
		t.Logf("Tags 'v1.0' and 'latest' point to the same digest")
	} else {
		t.Logf("Tags 'v1.0' and 'latest' point to different digests")
	}
}

// TestResolveMultipleTagsSameDigest verifies multiple tags can point to same digest
func TestResolveMultipleTagsSameDigest(t *testing.T) {
	t.Parallel()
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		t.Skip("Skipping integration test - GITHUB_TOKEN not set")
	}

	ctx := context.Background()
	testImage := "ghcr.io/mkoepf/ghcrctl-test-with-sbom"

	// Resolve multiple tags that should point to the same image
	tags := []string{"latest", "stable"}
	digests := make(map[string]string)

	for _, tag := range tags {
		digest, err := ResolveTag(ctx, testImage, tag)
		if err != nil {
			t.Logf("Tag '%s' could not be resolved: %v", tag, err)
			continue
		}
		digests[tag] = digest
		t.Logf("Tag '%s' -> %s", tag, digest)
	}

	// Verify we resolved at least one tag
	if len(digests) == 0 {
		t.Fatal("Failed to resolve any tags")
	}

	// If we have both tags, check if they're consistent
	if latestDigest, hasLatest := digests["latest"]; hasLatest {
		if stableDigest, hasStable := digests["stable"]; hasStable {
			if latestDigest == stableDigest {
				t.Logf("✓ Tags 'latest' and 'stable' correctly point to same digest")
			} else {
				t.Logf("Tags 'latest' and 'stable' point to different digests (this is acceptable)")
			}
		}
	}
}

// ===[ SBOM Discovery Integration Tests ]===

// TestDiscoverSBOMPresent verifies SBOM is discovered for image with attestations
func TestDiscoverSBOMPresent(t *testing.T) {
	t.Parallel()
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		t.Skip("Skipping integration test - GITHUB_TOKEN not set")
	}

	ctx := context.Background()
	testImage := "ghcr.io/mkoepf/ghcrctl-test-with-sbom"

	// Resolve tag to digest
	digest, err := ResolveTag(ctx, testImage, "latest")
	if err != nil {
		t.Fatalf("Failed to resolve tag: %v", err)
	}

	// Discover children
	children, err := DiscoverChildren(ctx, testImage, digest, nil)
	if err != nil {
		t.Fatalf("Failed to discover children: %v", err)
	}

	// Look for SBOM
	var foundSBOM bool
	for _, child := range children {
		if child.Type.Role == "sbom" {
			foundSBOM = true
			t.Logf("✓ Found SBOM: digest=%s", child.Digest)
			break
		}
	}

	if !foundSBOM {
		t.Error("Expected to find SBOM artifact, but none was found")
	}
}

// TestDiscoverSBOMAbsent verifies no SBOM found for image without attestations
func TestDiscoverSBOMAbsent(t *testing.T) {
	t.Parallel()
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		t.Skip("Skipping integration test - GITHUB_TOKEN not set")
	}

	ctx := context.Background()
	testImage := "ghcr.io/mkoepf/ghcrctl-test-no-sbom"

	// Resolve tag to digest
	digest, err := ResolveTag(ctx, testImage, "latest")
	if err != nil {
		t.Fatalf("Failed to resolve tag: %v", err)
	}

	// Discover children
	children, err := DiscoverChildren(ctx, testImage, digest, nil)
	if err != nil {
		t.Fatalf("Failed to discover children: %v", err)
	}

	// Verify no SBOM
	for _, child := range children {
		if child.Type.Role == "sbom" {
			t.Errorf("Expected no SBOM, but found one: digest=%s", child.Digest)
		}
	}

	t.Logf("✓ Correctly found no SBOM for image without attestations")
}

// TestDiscoverMultiLayerAttestations verifies SBOM and provenance from same manifest
func TestDiscoverMultiLayerAttestations(t *testing.T) {
	t.Parallel()
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		t.Skip("Skipping integration test - GITHUB_TOKEN not set")
	}

	ctx := context.Background()
	testImage := "ghcr.io/mkoepf/ghcrctl-test-with-sbom"

	// Resolve tag to digest
	digest, err := ResolveTag(ctx, testImage, "latest")
	if err != nil {
		t.Fatalf("Failed to resolve tag: %v", err)
	}

	// Discover children
	children, err := DiscoverChildren(ctx, testImage, digest, nil)
	if err != nil {
		t.Fatalf("Failed to discover children: %v", err)
	}

	// Look for both SBOM and provenance
	var sbomDigest, provenanceDigest string
	for _, child := range children {
		if child.Type.Role == "sbom" {
			sbomDigest = child.Digest
			t.Logf("Found SBOM: digest=%s", child.Digest)
		}
		if child.Type.Role == "provenance" {
			provenanceDigest = child.Digest
			t.Logf("Found provenance: digest=%s", child.Digest)
		}
	}

	if sbomDigest == "" {
		t.Error("Expected to find SBOM")
	}
	if provenanceDigest == "" {
		t.Error("Expected to find provenance")
	}

	// Docker buildx stores both in the same manifest
	if sbomDigest != "" && provenanceDigest != "" {
		if sbomDigest == provenanceDigest {
			t.Logf("✓ SBOM and provenance share same digest (multi-layer attestation manifest)")
		} else {
			t.Logf("SBOM and provenance have different digests (separate manifests)")
		}
	}
}

// ===[ Provenance Discovery Integration Tests ]====

// TestDiscoverProvenancePresent verifies provenance attestation is discovered
func TestDiscoverProvenancePresent(t *testing.T) {
	t.Parallel()
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		t.Skip("Skipping integration test - GITHUB_TOKEN not set")
	}

	ctx := context.Background()
	testImage := "ghcr.io/mkoepf/ghcrctl-test-with-sbom"

	// Resolve tag to digest
	digest, err := ResolveTag(ctx, testImage, "latest")
	if err != nil {
		t.Fatalf("Failed to resolve tag: %v", err)
	}

	// Discover children
	children, err := DiscoverChildren(ctx, testImage, digest, nil)
	if err != nil {
		t.Fatalf("Failed to discover children: %v", err)
	}

	// Look for provenance
	var foundProvenance bool
	for _, child := range children {
		if child.Type.Role == "provenance" {
			foundProvenance = true
			t.Logf("✓ Found provenance: digest=%s", child.Digest)
			break
		}
	}

	if !foundProvenance {
		t.Error("Expected to find provenance artifact, but none was found")
	}
}

// ===[] Edge Cases & Robustness Tests ]===

// TestMultipleTagsSameImage verifies digest consistency across tags
func TestMultipleTagsSameImage(t *testing.T) {
	t.Parallel()
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		t.Skip("Skipping integration test - GITHUB_TOKEN not set")
	}

	ctx := context.Background()
	testImage := "ghcr.io/mkoepf/ghcrctl-test-with-sbom"

	// Resolve the same tag multiple times
	digest1, err := ResolveTag(ctx, testImage, "latest")
	if err != nil {
		t.Fatalf("Failed to resolve tag (first attempt): %v", err)
	}

	digest2, err := ResolveTag(ctx, testImage, "latest")
	if err != nil {
		t.Fatalf("Failed to resolve tag (second attempt): %v", err)
	}

	if digest1 != digest2 {
		t.Errorf("Expected same digest for same tag, got '%s' and '%s'", digest1, digest2)
	}

	t.Logf("✓ Digest consistency verified: %s", digest1)
}
