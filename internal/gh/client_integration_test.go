package gh

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/google/go-github/v58/github"
)

// Integration tests for GitHub client that require GITHUB_TOKEN
// These tests will be skipped if GITHUB_TOKEN is not set

// ===[ Version ID Mapping Tests ]===

// TestGetVersionIDByDigestIntegration verifies mapping digest to GitHub package version ID
func TestGetVersionIDByDigestIntegration(t *testing.T) {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		t.Skip("Skipping integration test - GITHUB_TOKEN not set")
	}

	// Create client
	client, err := NewClient(token)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	ctx := context.Background()

	// We need a real digest from a real image
	// This test assumes the integration test workflow has created these images
	owner := "mkoepf"
	ownerType := "user"
	packageName := "ghcrctl-test-with-sbom"

	// First, we need to get a known digest
	// We'll use the ORAS resolver to get a digest for a known tag
	// Import is not possible here since it would create a circular dependency
	// So we'll use a known digest or skip if we can't determine it

	// For now, we'll test with a digest that should exist
	// The actual digest will vary, so we'll list versions and pick the first one
	// to verify the mapping works

	testDigest := os.Getenv("TEST_DIGEST")
	if testDigest == "" {
		t.Skip("Skipping test - TEST_DIGEST not set (run resolver tests first to get a digest)")
	}

	// Map digest to version ID
	versionID, err := client.GetVersionIDByDigest(ctx, owner, ownerType, packageName, testDigest)
	if err != nil {
		t.Fatalf("Failed to map digest to version ID: %v", err)
	}

	// Verify we got a non-zero version ID
	if versionID == 0 {
		t.Error("Expected non-zero version ID")
	}

	t.Logf("✓ Successfully mapped digest to version ID: %d", versionID)
}

// TestGetVersionIDByDigestWithRealImage uses a complete workflow
func TestGetVersionIDByDigestWithRealImage(t *testing.T) {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		t.Skip("Skipping integration test - GITHUB_TOKEN not set")
	}

	// This test needs access to ORAS to resolve a tag first
	// Since we can't import due to circular dependencies, we'll use a hardcoded digest
	// that we know exists from the workflow

	// Create client
	client, err := NewClient(token)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	ctx := context.Background()
	owner := "mkoepf"
	ownerType := "user"
	packageName := "ghcrctl-test-with-sbom"

	// List all versions to find one we can test with
	opts := &github.PackageListOptions{
		PackageType: github.String("container"),
		State:       github.String("active"),
		ListOptions: github.ListOptions{PerPage: 10},
	}

	// This is a workaround - we'll need to use the go-github import
	// Let me check what's already imported
	versions, _, err := client.client.Users.PackageGetAllVersions(ctx, owner, "container", packageName, opts)
	if err != nil {
		t.Fatalf("Failed to list package versions: %v", err)
	}

	if len(versions) == 0 {
		t.Skip("No package versions found - integration test images may not be created yet")
	}

	// Use the first version
	firstVersion := versions[0]
	if firstVersion.Name == nil || firstVersion.ID == nil {
		t.Fatal("Version has nil name or ID")
	}

	digest := *firstVersion.Name
	expectedVersionID := *firstVersion.ID

	t.Logf("Testing with digest: %s", digest)
	t.Logf("Expected version ID: %d", expectedVersionID)

	// Verify it's a valid digest format
	if !strings.HasPrefix(digest, "sha256:") {
		t.Skipf("Version name is not a digest (got %s), skipping test", digest)
	}

	// Now call our function to map it
	actualVersionID, err := client.GetVersionIDByDigest(ctx, owner, ownerType, packageName, digest)
	if err != nil {
		t.Fatalf("Failed to map digest to version ID: %v", err)
	}

	// Verify the IDs match
	if actualVersionID != expectedVersionID {
		t.Errorf("Version ID mismatch: expected %d, got %d", expectedVersionID, actualVersionID)
	}

	t.Logf("✓ Successfully verified digest->version ID mapping")
}

// TestVersionIDNotFound verifies error handling for non-existent digest
func TestVersionIDNotFound(t *testing.T) {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		t.Skip("Skipping integration test - GITHUB_TOKEN not set")
	}

	// Create client
	client, err := NewClient(token)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	ctx := context.Background()
	owner := "mkoepf"
	ownerType := "user"
	packageName := "ghcrctl-test-with-sbom"

	// Use a fake digest that definitely doesn't exist
	fakeDigest := "sha256:0000000000000000000000000000000000000000000000000000000000000000"

	// This should return an error
	versionID, err := client.GetVersionIDByDigest(ctx, owner, ownerType, packageName, fakeDigest)
	if err == nil {
		t.Error("Expected error for non-existent digest, but got none")
	}

	if versionID != 0 {
		t.Errorf("Expected version ID 0 for error case, got %d", versionID)
	}

	if !strings.Contains(err.Error(), "no version found") {
		t.Errorf("Expected error message to contain 'no version found', got: %v", err)
	}

	t.Logf("✓ Correctly handled non-existent digest")
}
