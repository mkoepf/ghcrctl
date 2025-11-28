package oras

import (
	"context"
	"os"
	"testing"
)

// Cosign Discovery Integration Tests
//
// These tests require the cosign-signed test image to be available.
// The image is created by running the prepare_integration_test.yml workflow.

// TestDiscoverCosignSignature verifies cosign signature discovery via tag pattern
func TestDiscoverCosignSignature(t *testing.T) {
	t.Parallel()
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		t.Skip("Skipping integration test - GITHUB_TOKEN not set")
	}

	ctx := context.Background()
	testImage := "ghcr.io/mkoepf/ghcrctl-test-cosign-signed"

	// Resolve tag to digest
	digest, err := ResolveTag(ctx, testImage, "latest")
	if err != nil {
		t.Fatalf("Failed to resolve tag - cosign-signed test image must exist: %v", err)
	}

	// List all tags for this image to enable cosign discovery
	// For now, we'll construct the expected cosign tags
	sigTag := digestToTagPrefix(digest) + ".sig"
	allTags := []string{"latest", "v1.0", sigTag}

	// Discover children with cosign tag discovery enabled
	children, err := DiscoverChildren(ctx, testImage, digest, allTags)
	if err != nil {
		t.Fatalf("Failed to discover children: %v", err)
	}

	// Look for signature
	var foundSig bool
	for _, child := range children {
		if child.Type.Role == "signature" {
			foundSig = true
			t.Logf("✓ Found cosign signature: digest=%s, tag=%s", child.Digest, child.Tag)
			if child.Tag == "" {
				t.Error("Expected non-empty tag for cosign signature")
			}
			break
		}
	}

	if !foundSig {
		t.Error("Expected to find cosign signature artifact")
	}
}

// TestDiscoverCosignAttestation verifies cosign attestation discovery with type resolution
func TestDiscoverCosignAttestation(t *testing.T) {
	t.Parallel()
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		t.Skip("Skipping integration test - GITHUB_TOKEN not set")
	}

	ctx := context.Background()
	testImage := "ghcr.io/mkoepf/ghcrctl-test-cosign-signed"

	// Resolve tag to digest
	digest, err := ResolveTag(ctx, testImage, "latest")
	if err != nil {
		t.Fatalf("Failed to resolve tag - cosign-signed test image must exist: %v", err)
	}

	// List all tags including cosign attestation tag
	attTag := digestToTagPrefix(digest) + ".att"
	allTags := []string{"latest", "v1.0", attTag}

	// Discover children with cosign tag discovery enabled
	children, err := DiscoverChildren(ctx, testImage, digest, allTags)
	if err != nil {
		t.Fatalf("Failed to discover children: %v", err)
	}

	// Look for attestation - should be resolved to specific type (sbom), not generic "attestation"
	var foundSBOM bool
	for _, child := range children {
		if child.Type.Role == "sbom" && child.Tag != "" {
			foundSBOM = true
			t.Logf("✓ Found cosign SBOM attestation: digest=%s, tag=%s", child.Digest, child.Tag)
			break
		}
	}

	if !foundSBOM {
		// Check if we found any attestation at all
		for _, child := range children {
			if child.Type.IsAttestation() && child.Tag != "" {
				t.Logf("Found attestation with role=%s (expected sbom)", child.Type.Role)
			}
		}
		t.Error("Expected to find cosign SBOM attestation with specific role")
	}
}

// TestDiscoverCosignBothArtifacts verifies both signature and attestation are discovered
func TestDiscoverCosignBothArtifacts(t *testing.T) {
	t.Parallel()
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		t.Skip("Skipping integration test - GITHUB_TOKEN not set")
	}

	ctx := context.Background()
	testImage := "ghcr.io/mkoepf/ghcrctl-test-cosign-signed"

	// Resolve tag to digest
	digest, err := ResolveTag(ctx, testImage, "latest")
	if err != nil {
		t.Fatalf("Failed to resolve tag - cosign-signed test image must exist: %v", err)
	}

	// Construct all expected tags
	sigTag := digestToTagPrefix(digest) + ".sig"
	attTag := digestToTagPrefix(digest) + ".att"
	allTags := []string{"latest", "v1.0", sigTag, attTag}

	// Discover children with cosign tag discovery enabled
	children, err := DiscoverChildren(ctx, testImage, digest, allTags)
	if err != nil {
		t.Fatalf("Failed to discover children: %v", err)
	}

	// Count cosign artifacts (identified by non-empty Tag field)
	var sigCount, attCount int
	for _, child := range children {
		if child.Tag != "" {
			if child.Type.Role == "signature" {
				sigCount++
				t.Logf("Found signature: tag=%s", child.Tag)
			} else if child.Type.IsAttestation() {
				attCount++
				t.Logf("Found attestation: role=%s, tag=%s", child.Type.Role, child.Tag)
			}
		}
	}

	if sigCount == 0 {
		t.Error("Expected at least 1 cosign signature")
	}
	if attCount == 0 {
		t.Error("Expected at least 1 cosign attestation")
	}

	t.Logf("✓ Found %d signature(s) and %d attestation(s)", sigCount, attCount)
}

// TestCosignAttestationTypeNotGeneric verifies cosign attestations are resolved to specific types
func TestCosignAttestationTypeNotGeneric(t *testing.T) {
	t.Parallel()
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		t.Skip("Skipping integration test - GITHUB_TOKEN not set")
	}

	ctx := context.Background()
	testImage := "ghcr.io/mkoepf/ghcrctl-test-cosign-signed"

	// Resolve tag to digest
	digest, err := ResolveTag(ctx, testImage, "latest")
	if err != nil {
		t.Fatalf("Failed to resolve tag - cosign-signed test image must exist: %v", err)
	}

	// Construct cosign attestation tag
	attTag := digestToTagPrefix(digest) + ".att"
	allTags := []string{"latest", attTag}

	// Discover children
	children, err := DiscoverChildren(ctx, testImage, digest, allTags)
	if err != nil {
		t.Fatalf("Failed to discover children: %v", err)
	}

	// Check that cosign attestations have specific types, not generic "attestation"
	for _, child := range children {
		if child.Tag != "" && child.Type.IsAttestation() {
			if child.Type.Role == "attestation" {
				t.Errorf("Cosign attestation should have specific role (sbom, provenance, etc.), got generic 'attestation'")
			} else {
				t.Logf("✓ Cosign attestation has specific role: %s", child.Type.Role)
			}
		}
	}
}
