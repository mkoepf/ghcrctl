package cmd

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/mkoepf/ghcrctl/internal/config"
	"github.com/mkoepf/ghcrctl/internal/gh"
	"github.com/mkoepf/ghcrctl/internal/oras"
)

// Integration tests for buildGraph and countGraphMembership
// These tests require GITHUB_TOKEN and will skip if not set

// TestBuildGraphMultiarchWithSBOMAndProvenance tests graph building with a complex image
func TestBuildGraphMultiarchWithSBOMAndProvenance(t *testing.T) {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		t.Skip("Skipping integration test - GITHUB_TOKEN not set")
	}

	ctx := context.Background()

	// Set up config
	cfg := config.New()
	err := cfg.SetOwner("mkoepf", "user")
	if err != nil {
		t.Fatalf("Failed to set owner: %v", err)
	}

	owner := "mkoepf"
	ownerType := "user"
	imageName := "ghcrctl-test-with-sbom"
	fullImage := "ghcr.io/" + owner + "/" + imageName
	tag := "latest"

	// Create GitHub client
	client, err := gh.NewClient(token)
	if err != nil {
		t.Fatalf("Failed to create GitHub client: %v", err)
	}

	// Resolve tag to digest
	rootDigest, err := oras.ResolveTag(ctx, fullImage, tag)
	if err != nil {
		t.Fatalf("Failed to resolve tag: %v", err)
	}

	// Build graph
	graph, err := buildGraph(ctx, client, fullImage, owner, ownerType, imageName, rootDigest, tag)
	if err != nil {
		t.Fatalf("buildGraph failed: %v", err)
	}

	if graph == nil {
		t.Fatal("Expected non-nil graph")
	}

	t.Logf("Graph root: %s (ID: %d)", graph.RootVersion.Name, graph.RootVersion.ID)
	t.Logf("Graph type: %s", graph.Type)
	t.Logf("Children count: %d", len(graph.Children))

	// Verify root version
	if graph.RootVersion.Name != rootDigest {
		t.Errorf("Expected root digest %s, got %s", rootDigest, graph.RootVersion.Name)
	}

	if graph.RootVersion.ID == 0 {
		t.Error("Expected non-zero root version ID")
	}

	// Multiarch image should be an index
	if graph.Type != "index" {
		t.Errorf("Expected type 'index' for multiarch image, got '%s'", graph.Type)
	}

	// Should have children (platforms + attestations)
	if len(graph.Children) == 0 {
		t.Error("Expected children (platforms/attestations) for multiarch image with SBOM")
	}

	// Verify we have platform children
	platformCount := 0
	attestationCount := 0
	for _, child := range graph.Children {
		t.Logf("  Child: %s (ID: %d, RefCount: %d)", child.Type.DisplayType(), child.Version.ID, child.RefCount)
		if child.Type.IsPlatform() {
			platformCount++
		} else {
			attestationCount++
		}
	}

	// Should have at least 2 platforms (amd64, arm64)
	if platformCount < 2 {
		t.Errorf("Expected at least 2 platform children, got %d", platformCount)
	}

	// Should have attestations (SBOM and provenance)
	if attestationCount < 1 {
		t.Errorf("Expected at least 1 attestation child, got %d", attestationCount)
	}
}

// TestBuildGraphNoSBOMImage tests graph building with an image without SBOM
// Note: Docker's defaults may add provenance even when not explicitly requested
func TestBuildGraphNoSBOMImage(t *testing.T) {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		t.Skip("Skipping integration test - GITHUB_TOKEN not set")
	}

	ctx := context.Background()

	// Set up config
	cfg := config.New()
	err := cfg.SetOwner("mkoepf", "user")
	if err != nil {
		t.Fatalf("Failed to set owner: %v", err)
	}

	owner := "mkoepf"
	ownerType := "user"
	imageName := "ghcrctl-test-no-sbom"
	fullImage := "ghcr.io/" + owner + "/" + imageName
	tag := "latest"

	// Create GitHub client
	client, err := gh.NewClient(token)
	if err != nil {
		t.Fatalf("Failed to create GitHub client: %v", err)
	}

	// Resolve tag to digest
	rootDigest, err := oras.ResolveTag(ctx, fullImage, tag)
	if err != nil {
		t.Fatalf("Failed to resolve tag: %v", err)
	}

	// Build graph
	graph, err := buildGraph(ctx, client, fullImage, owner, ownerType, imageName, rootDigest, tag)
	if err != nil {
		t.Fatalf("buildGraph failed: %v", err)
	}

	if graph == nil {
		t.Fatal("Expected non-nil graph")
	}

	t.Logf("Graph root: %s (ID: %d)", graph.RootVersion.Name, graph.RootVersion.ID)
	t.Logf("Graph type: %s", graph.Type)
	t.Logf("Children count: %d", len(graph.Children))

	// Verify root version
	if graph.RootVersion.Name != rootDigest {
		t.Errorf("Expected root digest %s, got %s", rootDigest, graph.RootVersion.Name)
	}

	if graph.RootVersion.ID == 0 {
		t.Error("Expected non-zero root version ID")
	}

	// Log children details
	hasSBOM := false
	for _, child := range graph.Children {
		t.Logf("  Child: %s (ID: %d)", child.Type.DisplayType(), child.Version.ID)
		if child.Type.Role == "sbom" {
			hasSBOM = true
		}
	}

	// This image should NOT have SBOM (that's the key distinction)
	if hasSBOM {
		t.Error("Expected no SBOM for ghcrctl-test-no-sbom image")
	}
}

// TestBuildGraphWithSBOMOnly tests graph building with SBOM but no provenance
func TestBuildGraphWithSBOMOnly(t *testing.T) {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		t.Skip("Skipping integration test - GITHUB_TOKEN not set")
	}

	ctx := context.Background()

	// Set up config
	cfg := config.New()
	err := cfg.SetOwner("mkoepf", "user")
	if err != nil {
		t.Fatalf("Failed to set owner: %v", err)
	}

	owner := "mkoepf"
	ownerType := "user"
	imageName := "ghcrctl-test-with-sbom-no-provenance"
	fullImage := "ghcr.io/" + owner + "/" + imageName
	tag := "latest"

	// Create GitHub client
	client, err := gh.NewClient(token)
	if err != nil {
		t.Fatalf("Failed to create GitHub client: %v", err)
	}

	// Resolve tag to digest
	rootDigest, err := oras.ResolveTag(ctx, fullImage, tag)
	if err != nil {
		t.Fatalf("Failed to resolve tag: %v", err)
	}

	// Build graph
	graph, err := buildGraph(ctx, client, fullImage, owner, ownerType, imageName, rootDigest, tag)
	if err != nil {
		t.Fatalf("buildGraph failed: %v", err)
	}

	if graph == nil {
		t.Fatal("Expected non-nil graph")
	}

	t.Logf("Graph root: %s (ID: %d)", graph.RootVersion.Name, graph.RootVersion.ID)
	t.Logf("Graph type: %s", graph.Type)
	t.Logf("Children count: %d", len(graph.Children))

	// Check for SBOM in children
	hasSBOM := false
	hasProvenance := false
	for _, child := range graph.Children {
		t.Logf("  Child: %s (ID: %d)", child.Type.DisplayType(), child.Version.ID)
		if child.Type.Role == "sbom" {
			hasSBOM = true
		}
		if child.Type.Role == "provenance" {
			hasProvenance = true
		}
	}

	if !hasSBOM {
		t.Error("Expected SBOM child in graph")
	}

	if hasProvenance {
		t.Error("Did not expect provenance child for SBOM-only image")
	}
}

// TestBuildGraphMultiarchNoAttestations tests graph building with multiarch but no attestations
func TestBuildGraphMultiarchNoAttestations(t *testing.T) {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		t.Skip("Skipping integration test - GITHUB_TOKEN not set")
	}

	ctx := context.Background()

	// Set up config
	cfg := config.New()
	err := cfg.SetOwner("mkoepf", "user")
	if err != nil {
		t.Fatalf("Failed to set owner: %v", err)
	}

	owner := "mkoepf"
	ownerType := "user"
	imageName := "ghcrctl-test-no-sbom-no-provenance"
	fullImage := "ghcr.io/" + owner + "/" + imageName
	tag := "latest"

	// Create GitHub client
	client, err := gh.NewClient(token)
	if err != nil {
		t.Fatalf("Failed to create GitHub client: %v", err)
	}

	// Resolve tag to digest
	rootDigest, err := oras.ResolveTag(ctx, fullImage, tag)
	if err != nil {
		t.Fatalf("Failed to resolve tag: %v", err)
	}

	// Build graph
	graph, err := buildGraph(ctx, client, fullImage, owner, ownerType, imageName, rootDigest, tag)
	if err != nil {
		t.Fatalf("buildGraph failed: %v", err)
	}

	if graph == nil {
		t.Fatal("Expected non-nil graph")
	}

	t.Logf("Graph root: %s (ID: %d)", graph.RootVersion.Name, graph.RootVersion.ID)
	t.Logf("Graph type: %s", graph.Type)
	t.Logf("Children count: %d", len(graph.Children))

	// Should be index type for multiarch
	if graph.Type != "index" {
		t.Errorf("Expected type 'index' for multiarch image, got '%s'", graph.Type)
	}

	// Should have platform children but no attestations
	platformCount := 0
	attestationCount := 0
	for _, child := range graph.Children {
		t.Logf("  Child: %s (ID: %d)", child.Type.DisplayType(), child.Version.ID)
		if child.Type.IsPlatform() {
			platformCount++
		} else {
			attestationCount++
		}
	}

	// Should have at least 2 platforms (amd64, arm64)
	if platformCount < 2 {
		t.Errorf("Expected at least 2 platform children, got %d", platformCount)
	}

	// Should have no attestations
	if attestationCount > 0 {
		t.Errorf("Expected no attestations for no-sbom-no-provenance image, got %d", attestationCount)
	}
}

// TestCountGraphMembershipRootVersion tests counting membership for a root version
func TestCountGraphMembershipRootVersion(t *testing.T) {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		t.Skip("Skipping integration test - GITHUB_TOKEN not set")
	}

	ctx := context.Background()

	// Set up config
	cfg := config.New()
	err := cfg.SetOwner("mkoepf", "user")
	if err != nil {
		t.Fatalf("Failed to set owner: %v", err)
	}

	owner := "mkoepf"
	ownerType := "user"
	imageName := "ghcrctl-test-with-sbom"
	fullImage := "ghcr.io/" + owner + "/" + imageName
	tag := "latest"

	// Create GitHub client
	client, err := gh.NewClient(token)
	if err != nil {
		t.Fatalf("Failed to create GitHub client: %v", err)
	}

	// Resolve tag to get root digest
	rootDigest, err := oras.ResolveTag(ctx, fullImage, tag)
	if err != nil {
		t.Fatalf("Failed to resolve tag: %v", err)
	}

	// Get the version ID for the root
	rootVersionID, err := client.GetVersionIDByDigest(ctx, owner, ownerType, imageName, rootDigest)
	if err != nil {
		t.Fatalf("Failed to get version ID: %v", err)
	}

	t.Logf("Root version ID: %d, digest: %s", rootVersionID, rootDigest)

	// Count membership
	count := countGraphMembership(ctx, client, owner, ownerType, imageName, rootVersionID)

	t.Logf("Root version membership count: %d", count)

	// Root version should be in exactly 1 graph (itself)
	if count != 1 {
		t.Errorf("Expected root version to be in exactly 1 graph, got %d", count)
	}
}

// TestCountGraphMembershipChildVersion tests counting membership for a child version
func TestCountGraphMembershipChildVersion(t *testing.T) {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		t.Skip("Skipping integration test - GITHUB_TOKEN not set")
	}

	ctx := context.Background()

	// Set up config
	cfg := config.New()
	err := cfg.SetOwner("mkoepf", "user")
	if err != nil {
		t.Fatalf("Failed to set owner: %v", err)
	}

	owner := "mkoepf"
	ownerType := "user"
	imageName := "ghcrctl-test-with-sbom"
	fullImage := "ghcr.io/" + owner + "/" + imageName
	tag := "latest"

	// Create GitHub client
	client, err := gh.NewClient(token)
	if err != nil {
		t.Fatalf("Failed to create GitHub client: %v", err)
	}

	// Build graph to get a child version
	rootDigest, err := oras.ResolveTag(ctx, fullImage, tag)
	if err != nil {
		t.Fatalf("Failed to resolve tag: %v", err)
	}

	graph, err := buildGraph(ctx, client, fullImage, owner, ownerType, imageName, rootDigest, tag)
	if err != nil {
		t.Fatalf("buildGraph failed: %v", err)
	}

	if graph == nil || len(graph.Children) == 0 {
		t.Skip("No children in graph, cannot test child membership")
	}

	// Get first child with a valid version ID
	var childVersionID int64
	for _, child := range graph.Children {
		if child.Version.ID != 0 {
			childVersionID = child.Version.ID
			t.Logf("Testing child: %s (ID: %d)", child.Type.DisplayType(), childVersionID)
			break
		}
	}

	if childVersionID == 0 {
		t.Skip("No child with valid version ID found")
	}

	// Count membership
	count := countGraphMembership(ctx, client, owner, ownerType, imageName, childVersionID)

	t.Logf("Child version membership count: %d", count)

	// Child should be in at least 1 graph
	if count < 1 {
		t.Errorf("Expected child version to be in at least 1 graph, got %d", count)
	}
}

// TestCountGraphMembershipNonexistentVersion tests counting membership for a nonexistent version
func TestCountGraphMembershipNonexistentVersion(t *testing.T) {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		t.Skip("Skipping integration test - GITHUB_TOKEN not set")
	}

	ctx := context.Background()

	// Set up config
	cfg := config.New()
	err := cfg.SetOwner("mkoepf", "user")
	if err != nil {
		t.Fatalf("Failed to set owner: %v", err)
	}

	owner := "mkoepf"
	ownerType := "user"
	imageName := "ghcrctl-test-with-sbom"

	// Create GitHub client
	client, err := gh.NewClient(token)
	if err != nil {
		t.Fatalf("Failed to create GitHub client: %v", err)
	}

	// Use a nonexistent version ID
	nonexistentVersionID := int64(999999999)

	// Count membership
	count := countGraphMembership(ctx, client, owner, ownerType, imageName, nonexistentVersionID)

	t.Logf("Nonexistent version membership count: %d", count)

	// Nonexistent version should be in 0 graphs
	if count != 0 {
		t.Errorf("Expected nonexistent version to be in 0 graphs, got %d", count)
	}
}

// TestBuildGraphRefCountCalculation tests that RefCount is properly calculated
func TestBuildGraphRefCountCalculation(t *testing.T) {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		t.Skip("Skipping integration test - GITHUB_TOKEN not set")
	}

	ctx := context.Background()

	// Set up config
	cfg := config.New()
	err := cfg.SetOwner("mkoepf", "user")
	if err != nil {
		t.Fatalf("Failed to set owner: %v", err)
	}

	owner := "mkoepf"
	ownerType := "user"
	imageName := "ghcrctl-test-with-sbom"
	fullImage := "ghcr.io/" + owner + "/" + imageName
	tag := "latest"

	// Create GitHub client
	client, err := gh.NewClient(token)
	if err != nil {
		t.Fatalf("Failed to create GitHub client: %v", err)
	}

	// Build graph
	rootDigest, err := oras.ResolveTag(ctx, fullImage, tag)
	if err != nil {
		t.Fatalf("Failed to resolve tag: %v", err)
	}

	graph, err := buildGraph(ctx, client, fullImage, owner, ownerType, imageName, rootDigest, tag)
	if err != nil {
		t.Fatalf("buildGraph failed: %v", err)
	}

	if graph == nil {
		t.Fatal("Expected non-nil graph")
	}

	// Verify RefCount is calculated for all children
	for _, child := range graph.Children {
		t.Logf("Child %s (ID: %d): RefCount = %d",
			child.Type.DisplayType(), child.Version.ID, child.RefCount)

		// RefCount should be at least 1 (the current graph references it)
		if child.RefCount < 1 {
			t.Errorf("Expected RefCount >= 1, got %d for child %s",
				child.RefCount, child.Type.DisplayType())
		}
	}
}

// TestCollectVersionIDsIntegration tests collectVersionIDs with a real graph
func TestCollectVersionIDsIntegration(t *testing.T) {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		t.Skip("Skipping integration test - GITHUB_TOKEN not set")
	}

	ctx := context.Background()

	// Set up config
	cfg := config.New()
	err := cfg.SetOwner("mkoepf", "user")
	if err != nil {
		t.Fatalf("Failed to set owner: %v", err)
	}

	owner := "mkoepf"
	ownerType := "user"
	imageName := "ghcrctl-test-with-sbom"
	fullImage := "ghcr.io/" + owner + "/" + imageName
	tag := "latest"

	// Create GitHub client
	client, err := gh.NewClient(token)
	if err != nil {
		t.Fatalf("Failed to create GitHub client: %v", err)
	}

	// Build graph
	rootDigest, err := oras.ResolveTag(ctx, fullImage, tag)
	if err != nil {
		t.Fatalf("Failed to resolve tag: %v", err)
	}

	graph, err := buildGraph(ctx, client, fullImage, owner, ownerType, imageName, rootDigest, tag)
	if err != nil {
		t.Fatalf("buildGraph failed: %v", err)
	}

	if graph == nil {
		t.Fatal("Expected non-nil graph")
	}

	// Collect version IDs
	ids := collectVersionIDs(graph)

	t.Logf("Collected %d version IDs for deletion", len(ids))
	for i, id := range ids {
		t.Logf("  [%d] ID: %d", i, id)
	}

	// Should have at least the root
	if len(ids) == 0 {
		t.Error("Expected at least the root ID in collected IDs")
	}

	// Root should be last (deletion order)
	if len(ids) > 0 && ids[len(ids)-1] != graph.RootVersion.ID {
		t.Errorf("Expected root ID %d to be last, got %d",
			graph.RootVersion.ID, ids[len(ids)-1])
	}

	// Verify no shared children are included
	for _, child := range graph.Children {
		if child.RefCount > 1 {
			// This child is shared, should NOT be in the list
			for _, id := range ids {
				if id == child.Version.ID {
					t.Errorf("Shared child %d (RefCount=%d) should NOT be in deletion list",
						child.Version.ID, child.RefCount)
				}
			}
		}
	}
}

// =============================================================================
// Dry-Run Integration Tests
// These tests verify the delete workflow against real images without deleting
// =============================================================================

// TestDeleteGraphDryRunIntegration tests that dry-run correctly identifies
// all versions that would be deleted without actually deleting them
func TestDeleteGraphDryRunIntegration(t *testing.T) {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		t.Skip("Skipping integration test - GITHUB_TOKEN not set")
	}

	ctx := context.Background()

	// Set up config
	cfg := config.New()
	err := cfg.SetOwner("mkoepf", "user")
	if err != nil {
		t.Fatalf("Failed to set owner: %v", err)
	}

	owner := "mkoepf"
	ownerType := "user"
	imageName := "ghcrctl-test-with-sbom"
	fullImage := "ghcr.io/" + owner + "/" + imageName
	tag := "latest"

	// Create GitHub client
	client, err := gh.NewClient(token)
	if err != nil {
		t.Fatalf("Failed to create GitHub client: %v", err)
	}

	// Resolve tag to digest
	rootDigest, err := oras.ResolveTag(ctx, fullImage, tag)
	if err != nil {
		t.Fatalf("Failed to resolve tag: %v", err)
	}

	// Build graph (this is what delete graph --dry-run does internally)
	graph, err := buildGraph(ctx, client, fullImage, owner, ownerType, imageName, rootDigest, tag)
	if err != nil {
		t.Fatalf("buildGraph failed: %v", err)
	}

	if graph == nil {
		t.Fatal("Expected non-nil graph")
	}

	// Simulate dry-run: collect what WOULD be deleted
	idsToDelete := collectVersionIDs(graph)

	t.Logf("Dry-run would delete %d versions:", len(idsToDelete))

	// Verify deletion order (children before root)
	if len(idsToDelete) > 0 {
		lastID := idsToDelete[len(idsToDelete)-1]
		if lastID != graph.RootVersion.ID {
			t.Errorf("Root should be deleted last, got %d, want %d", lastID, graph.RootVersion.ID)
		}
	}

	// Verify shared children are NOT in the list
	sharedCount := 0
	for _, child := range graph.Children {
		if child.RefCount > 1 {
			sharedCount++
			for _, id := range idsToDelete {
				if id == child.Version.ID {
					t.Errorf("Shared child %d (RefCount=%d) should NOT be in deletion list",
						child.Version.ID, child.RefCount)
				}
			}
		}
	}
	t.Logf("Protected %d shared children from deletion", sharedCount)

	// Verify all versions in the deletion list actually exist
	for _, id := range idsToDelete {
		tags, err := client.GetVersionTags(ctx, owner, ownerType, imageName, id)
		if err != nil {
			t.Errorf("Version %d should exist but got error: %v", id, err)
		}
		t.Logf("  Version %d exists (tags: %v)", id, tags)
	}

	// Verify versions are NOT deleted (they should still exist)
	t.Log("Verifying no versions were deleted (dry-run)...")
	for _, id := range idsToDelete {
		_, err := client.GetVersionTags(ctx, owner, ownerType, imageName, id)
		if err != nil {
			t.Errorf("Version %d should still exist after dry-run: %v", id, err)
		}
	}
}

// TestExecuteSingleDeleteDryRunIntegration tests single version dry-run
func TestExecuteSingleDeleteDryRunIntegration(t *testing.T) {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		t.Skip("Skipping integration test - GITHUB_TOKEN not set")
	}

	ctx := context.Background()

	// Set up config
	cfg := config.New()
	err := cfg.SetOwner("mkoepf", "user")
	if err != nil {
		t.Fatalf("Failed to set owner: %v", err)
	}

	owner := "mkoepf"
	ownerType := "user"
	imageName := "ghcrctl-test-with-sbom"
	fullImage := "ghcr.io/" + owner + "/" + imageName
	tag := "latest"

	// Create GitHub client
	client, err := gh.NewClient(token)
	if err != nil {
		t.Fatalf("Failed to create GitHub client: %v", err)
	}

	// Resolve tag to get a real version ID
	rootDigest, err := oras.ResolveTag(ctx, fullImage, tag)
	if err != nil {
		t.Fatalf("Failed to resolve tag: %v", err)
	}

	versionID, err := client.GetVersionIDByDigest(ctx, owner, ownerType, imageName, rootDigest)
	if err != nil {
		t.Fatalf("Failed to get version ID: %v", err)
	}

	tags, err := client.GetVersionTags(ctx, owner, ownerType, imageName, versionID)
	if err != nil {
		t.Fatalf("Failed to get tags: %v", err)
	}

	t.Logf("Testing dry-run for version %d (tags: %v)", versionID, tags)

	// Create a mock deleter that tracks calls
	mock := &mockPackageDeleterIntegration{
		deletedVersions: []int64{},
	}

	// Build params for dry-run
	params := deleteVersionParams{
		owner:      owner,
		ownerType:  ownerType,
		imageName:  imageName,
		versionID:  versionID,
		tags:       tags,
		graphCount: 1, // It's a root, so count is 1
		force:      true,
		dryRun:     true, // DRY RUN
	}

	var buf strings.Builder
	confirmFn := func() (bool, error) {
		t.Error("Confirm should not be called in dry-run mode")
		return false, nil
	}

	err = executeSingleDelete(ctx, mock, params, &buf, confirmFn)
	if err != nil {
		t.Fatalf("executeSingleDelete failed: %v", err)
	}

	output := buf.String()

	// Verify dry-run message is shown
	if !strings.Contains(output, "DRY RUN") {
		t.Error("Expected 'DRY RUN' in output")
	}

	// Verify nothing was deleted
	if len(mock.deletedVersions) > 0 {
		t.Errorf("Expected no deletions in dry-run, got %v", mock.deletedVersions)
	}

	// Verify version still exists
	_, err = client.GetVersionTags(ctx, owner, ownerType, imageName, versionID)
	if err != nil {
		t.Errorf("Version %d should still exist after dry-run: %v", versionID, err)
	}
}

// mockPackageDeleterIntegration is a mock for integration tests
type mockPackageDeleterIntegration struct {
	deletedVersions []int64
}

func (m *mockPackageDeleterIntegration) DeletePackageVersion(ctx context.Context, owner, ownerType, packageName string, versionID int64) error {
	m.deletedVersions = append(m.deletedVersions, versionID)
	return nil
}

// TestExecuteBulkDeleteDryRunIntegration tests bulk deletion dry-run
func TestExecuteBulkDeleteDryRunIntegration(t *testing.T) {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		t.Skip("Skipping integration test - GITHUB_TOKEN not set")
	}

	ctx := context.Background()

	// Set up config
	cfg := config.New()
	err := cfg.SetOwner("mkoepf", "user")
	if err != nil {
		t.Fatalf("Failed to set owner: %v", err)
	}

	owner := "mkoepf"
	ownerType := "user"
	imageName := "ghcrctl-test-with-sbom"

	// Create GitHub client
	client, err := gh.NewClient(token)
	if err != nil {
		t.Fatalf("Failed to create GitHub client: %v", err)
	}

	// Get some real versions to test with
	allVersions, err := client.ListPackageVersions(ctx, owner, ownerType, imageName)
	if err != nil {
		t.Fatalf("Failed to list versions: %v", err)
	}

	if len(allVersions) < 2 {
		t.Skip("Need at least 2 versions for bulk delete test")
	}

	// Take first 3 versions for the test
	testVersions := allVersions
	if len(testVersions) > 3 {
		testVersions = testVersions[:3]
	}

	t.Logf("Testing dry-run bulk delete for %d versions", len(testVersions))

	// Create a mock deleter
	mock := &mockPackageDeleterIntegration{
		deletedVersions: []int64{},
	}

	// Build params for dry-run
	params := bulkDeleteParams{
		owner:     owner,
		ownerType: ownerType,
		imageName: imageName,
		versions:  testVersions,
		force:     true,
		dryRun:    true, // DRY RUN
	}

	var buf strings.Builder
	confirmFn := func(count int) (bool, error) {
		t.Error("Confirm should not be called in dry-run mode")
		return false, nil
	}

	err = executeBulkDelete(ctx, mock, params, &buf, confirmFn)
	if err != nil {
		t.Fatalf("executeBulkDelete failed: %v", err)
	}

	output := buf.String()

	// Verify dry-run message is shown
	if !strings.Contains(output, "DRY RUN") {
		t.Error("Expected 'DRY RUN' in output")
	}

	// Verify nothing was deleted
	if len(mock.deletedVersions) > 0 {
		t.Errorf("Expected no deletions in dry-run, got %v", mock.deletedVersions)
	}

	// Verify all versions still exist
	for _, ver := range testVersions {
		_, err := client.GetVersionTags(ctx, owner, ownerType, imageName, ver.ID)
		if err != nil {
			t.Errorf("Version %d should still exist after dry-run: %v", ver.ID, err)
		}
	}
}
