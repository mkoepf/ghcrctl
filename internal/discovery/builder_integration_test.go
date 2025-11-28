package discovery

import (
	"context"
	"os"
	"testing"

	"github.com/mhk/ghcrctl/internal/gh"
	"github.com/mhk/ghcrctl/internal/oras"
)

// Integration tests for GraphBuilder that require GITHUB_TOKEN
// These tests will be skipped if GITHUB_TOKEN is not set

// TestFindParentDigestIntegration_PlatformManifest tests finding parent of a platform manifest
func TestFindParentDigestIntegration_PlatformManifest(t *testing.T) {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		t.Skip("Skipping integration test - GITHUB_TOKEN not set")
	}

	ctx := context.Background()
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

	// Resolve tag to get the root (index) digest
	rootDigest, err := oras.ResolveTag(ctx, fullImage, tag)
	if err != nil {
		t.Fatalf("Failed to resolve tag: %v", err)
	}

	t.Logf("Root digest: %s", rootDigest)

	// Get platform manifests from the root
	platforms, err := oras.GetPlatformManifests(ctx, fullImage, rootDigest)
	if err != nil {
		t.Fatalf("Failed to get platform manifests: %v", err)
	}

	if len(platforms) == 0 {
		t.Skip("No platform manifests found - cannot test FindParentDigest")
	}

	// Pick the first platform as child
	childDigest := platforms[0].Digest
	t.Logf("Child platform digest: %s (%s)", childDigest, platforms[0].Platform)

	// Create builder and cache
	builder := NewGraphBuilder(ctx, client, fullImage, owner, ownerType, imageName)
	cache, err := builder.GetVersionCache()
	if err != nil {
		t.Fatalf("Failed to get version cache: %v", err)
	}

	// Find parent of the platform manifest
	foundParent, err := builder.FindParentDigest(childDigest, cache)
	if err != nil {
		t.Fatalf("FindParentDigest() error = %v", err)
	}

	if foundParent == "" {
		t.Error("Expected to find parent for platform manifest, got empty string")
	}

	// Note: Platform manifests may be shared across multiple indices (e.g., different tags
	// pointing to different index versions that share the same platform manifest).
	// FindParentDigest finds *a* valid parent, which may differ from rootDigest if the
	// platform is shared. We verify the found parent actually contains the child.
	parentPlatforms, err := oras.GetPlatformManifests(ctx, fullImage, foundParent)
	if err != nil {
		t.Fatalf("Failed to verify parent contains child: %v", err)
	}

	foundChild := false
	for _, p := range parentPlatforms {
		if p.Digest == childDigest {
			foundChild = true
			break
		}
	}

	if !foundChild {
		t.Errorf("Found parent %s does not actually contain child %s", foundParent, childDigest)
	}

	t.Logf("✓ Successfully found valid parent: %s (may differ from rootDigest if platform is shared)", foundParent)
}

// TestFindParentDigestIntegration_Attestation tests finding parent of an attestation
func TestFindParentDigestIntegration_Attestation(t *testing.T) {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		t.Skip("Skipping integration test - GITHUB_TOKEN not set")
	}

	ctx := context.Background()
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

	// Resolve tag to get the root digest
	rootDigest, err := oras.ResolveTag(ctx, fullImage, tag)
	if err != nil {
		t.Fatalf("Failed to resolve tag: %v", err)
	}

	t.Logf("Root digest: %s", rootDigest)

	// Get referrers (attestations) from the root
	referrers, err := oras.DiscoverReferrers(ctx, fullImage, rootDigest)
	if err != nil {
		t.Fatalf("Failed to discover referrers: %v", err)
	}

	if len(referrers) == 0 {
		t.Skip("No referrers found - cannot test FindParentDigest for attestations")
	}

	// Pick the first referrer as child
	childDigest := referrers[0].Digest
	t.Logf("Child attestation digest: %s (%s)", childDigest, referrers[0].ArtifactType)

	// Create builder and cache
	builder := NewGraphBuilder(ctx, client, fullImage, owner, ownerType, imageName)
	cache, err := builder.GetVersionCache()
	if err != nil {
		t.Fatalf("Failed to get version cache: %v", err)
	}

	// Find parent of the attestation
	foundParent, err := builder.FindParentDigest(childDigest, cache)
	if err != nil {
		t.Fatalf("FindParentDigest() error = %v", err)
	}

	if foundParent == "" {
		t.Error("Expected to find parent for attestation, got empty string")
	}

	if foundParent != rootDigest {
		t.Errorf("FindParentDigest() = %s, want %s", foundParent, rootDigest)
	}

	t.Logf("✓ Successfully found parent: %s", foundParent)
}

// TestFindParentDigestIntegration_RootHasNoParent tests that root returns empty parent
func TestFindParentDigestIntegration_RootHasNoParent(t *testing.T) {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		t.Skip("Skipping integration test - GITHUB_TOKEN not set")
	}

	ctx := context.Background()
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

	// Resolve tag to get the root digest
	rootDigest, err := oras.ResolveTag(ctx, fullImage, tag)
	if err != nil {
		t.Fatalf("Failed to resolve tag: %v", err)
	}

	t.Logf("Root digest: %s", rootDigest)

	// Create builder and cache
	builder := NewGraphBuilder(ctx, client, fullImage, owner, ownerType, imageName)
	cache, err := builder.GetVersionCache()
	if err != nil {
		t.Fatalf("Failed to get version cache: %v", err)
	}

	// Try to find parent of the root (should not find any)
	foundParent, err := builder.FindParentDigest(rootDigest, cache)
	if err != nil {
		t.Fatalf("FindParentDigest() error = %v", err)
	}

	if foundParent != "" {
		t.Errorf("Expected empty string for root (no parent), got %s", foundParent)
	}

	t.Logf("✓ Correctly returned no parent for root")
}

// TestBuildGraphIntegration_MultiarchWithAttestations tests building graph for real multiarch image
func TestBuildGraphIntegration_MultiarchWithAttestations(t *testing.T) {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		t.Skip("Skipping integration test - GITHUB_TOKEN not set")
	}

	ctx := context.Background()
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

	// Resolve tag to get the root digest
	rootDigest, err := oras.ResolveTag(ctx, fullImage, tag)
	if err != nil {
		t.Fatalf("Failed to resolve tag: %v", err)
	}

	t.Logf("Root digest: %s", rootDigest)

	// Create builder and cache
	builder := NewGraphBuilder(ctx, client, fullImage, owner, ownerType, imageName)
	cache, err := builder.GetVersionCache()
	if err != nil {
		t.Fatalf("Failed to get version cache: %v", err)
	}

	// Build graph
	graph, err := builder.BuildGraph(rootDigest, cache)
	if err != nil {
		t.Fatalf("BuildGraph() error = %v", err)
	}

	if graph == nil {
		t.Fatal("Expected non-nil graph")
	}

	t.Logf("Graph type: %s", graph.Type)
	t.Logf("Root version ID: %d", graph.RootVersion.ID)
	t.Logf("Children count: %d", len(graph.Children))

	// Verify it's an index (multiarch)
	if graph.Type != "index" {
		t.Errorf("Expected graph type 'index', got '%s'", graph.Type)
	}

	// Verify root
	if graph.RootVersion.ID == 0 {
		t.Error("Expected non-zero root version ID")
	}
	if graph.RootVersion.Name != rootDigest {
		t.Errorf("Expected root digest %s, got %s", rootDigest, graph.RootVersion.Name)
	}

	// Count platforms and attestations
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

	t.Logf("Platforms: %d, Attestations: %d", platformCount, attestationCount)

	// Should have at least 2 platforms (amd64, arm64)
	if platformCount < 2 {
		t.Errorf("Expected at least 2 platforms, got %d", platformCount)
	}

	// Should have attestations (SBOM/provenance)
	if attestationCount < 1 {
		t.Errorf("Expected at least 1 attestation, got %d", attestationCount)
	}
}
