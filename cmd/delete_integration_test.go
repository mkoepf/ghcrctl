package cmd

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/mkoepf/ghcrctl/internal/discover"
	"github.com/mkoepf/ghcrctl/internal/gh"
)

// Integration tests for delete command functionality
// These tests require GITHUB_TOKEN and will skip if not set

// TestCountIncomingRefsRootVersion tests counting incoming refs for a root version
func TestCountIncomingRefsRootVersion(t *testing.T) {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		t.Skip("Skipping integration test - GITHUB_TOKEN not set")
	}
	t.Parallel()

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

	// Resolve tag to get root digest
	rootDigest, err := discover.ResolveTag(ctx, fullImage, tag)
	if err != nil {
		t.Fatalf("Failed to resolve tag: %v", err)
	}

	// Get the version ID for the root
	rootVersionID, err := client.GetVersionIDByDigest(ctx, owner, ownerType, imageName, rootDigest)
	if err != nil {
		t.Fatalf("Failed to get version ID: %v", err)
	}

	t.Logf("Root version ID: %d, digest: %s", rootVersionID, rootDigest)

	// Count incoming refs (versions that reference this version)
	count := CountIncomingRefs(ctx, client, owner, ownerType, imageName, rootVersionID)

	t.Logf("Root version incoming ref count: %d", count)

	// Root version (index) should have 0 incoming refs - nothing references the root
	if count != 0 {
		t.Errorf("Expected root version to have 0 incoming refs, got %d", count)
	}
}

// TestCountIncomingRefsNonexistentVersion tests counting incoming refs for a nonexistent version
func TestCountIncomingRefsNonexistentVersion(t *testing.T) {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		t.Skip("Skipping integration test - GITHUB_TOKEN not set")
	}
	t.Parallel()

	ctx := context.Background()

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
	count := CountIncomingRefs(ctx, client, owner, ownerType, imageName, nonexistentVersionID)

	t.Logf("Nonexistent version membership count: %d", count)

	// Nonexistent version should be in 0 images
	if count != 0 {
		t.Errorf("Expected nonexistent version to be in 0 images, got %d", count)
	}
}

// TestExecuteSingleDeleteDryRunIntegration tests single version dry-run
func TestExecuteSingleDeleteDryRunIntegration(t *testing.T) {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		t.Skip("Skipping integration test - GITHUB_TOKEN not set")
	}
	t.Parallel()

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

	// Resolve tag to get a real version ID
	rootDigest, err := discover.ResolveTag(ctx, fullImage, tag)
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
	params := DeleteVersionParams{
		Owner:      owner,
		OwnerType:  ownerType,
		ImageName:  imageName,
		VersionID:  versionID,
		Tags:       tags,
		ImageCount: 1, // It's a root, so count is 1
		Force:      true,
		DryRun:     true, // DRY RUN
	}

	var buf strings.Builder
	confirmFn := func() (bool, error) {
		t.Error("Confirm should not be called in dry-run mode")
		return false, nil
	}

	err = ExecuteSingleDelete(ctx, mock, params, &buf, confirmFn)
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
	t.Parallel()

	ctx := context.Background()

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
	params := BulkDeleteParams{
		Owner:     owner,
		OwnerType: ownerType,
		ImageName: imageName,
		Versions:  testVersions,
		Force:     true,
		DryRun:    true, // DRY RUN
	}

	var buf strings.Builder
	confirmFn := func(count int) (bool, error) {
		t.Error("Confirm should not be called in dry-run mode")
		return false, nil
	}

	err = ExecuteBulkDelete(ctx, mock, params, &buf, confirmFn)
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
