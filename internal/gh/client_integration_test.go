//go:build !mutating

package gh

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/google/go-github/v58/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ===[ ListPackageVersions Tests ]===

// TestListPackageVersionsWithRealImage tests listing versions for a real image
func TestListPackageVersionsWithRealImage(t *testing.T) {
	t.Parallel()
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		t.Skip("Skipping integration test - GITHUB_TOKEN not set")
	}

	client, err := NewClient(token)
	require.NoError(t, err, "Failed to create client")

	ctx := context.Background()
	owner := "mkoepf"
	ownerType := "user"
	packageName := "ghcrctl-test-with-sbom"

	versions, err := client.ListPackageVersions(ctx, owner, ownerType, packageName)
	require.NoError(t, err)

	// Should have multiple versions (root + platforms + attestations)
	if len(versions) == 0 {
		t.Skip("No package versions found - integration test images may not be created yet")
	}

	t.Logf("Found %d versions", len(versions))

	// Verify structure of returned versions
	for i, ver := range versions {
		assert.NotZero(t, ver.ID, "Version %d has zero ID", i)
		assert.NotEmpty(t, ver.Name, "Version %d has empty name (digest)", i)
		assert.True(t, strings.HasPrefix(ver.Name, "sha256:"), "Version %d name should be a digest, got: %s", i, ver.Name)
		assert.NotEmpty(t, ver.CreatedAt, "Version %d has empty CreatedAt", i)

		// Log some details for debugging
		if i < 5 {
			t.Logf("  Version %d: ID=%d, Tags=%v, Digest=%s...", i, ver.ID, ver.Tags, ver.Name[:20])
		}
	}

	// Check for tagged versions
	taggedCount := 0
	for _, ver := range versions {
		if len(ver.Tags) > 0 {
			taggedCount++
		}
	}
	t.Logf("Tagged versions: %d, Untagged: %d", taggedCount, len(versions)-taggedCount)

	assert.Greater(t, taggedCount, 0, "Expected at least one tagged version")
}

// TestListPackageVersionsMultipleImages tests listing versions for different test images
func TestListPackageVersionsMultipleImages(t *testing.T) {
	t.Parallel()
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		t.Skip("Skipping integration test - GITHUB_TOKEN not set")
	}

	client, err := NewClient(token)
	require.NoError(t, err, "Failed to create client")

	ctx := context.Background()
	owner := "mkoepf"
	ownerType := "user"

	testImages := []struct {
		name          string
		expectMinVers int
		description   string
	}{
		{"ghcrctl-test-with-sbom", 3, "multiarch with SBOM and provenance"},
		{"ghcrctl-test-no-sbom", 1, "single platform with no SBOM"},
		{"ghcrctl-test-with-sbom-no-provenance", 2, "with SBOM, no provenance"},
		{"ghcrctl-test-no-sbom-no-provenance", 2, "multiarch, no attestations"},
	}

	for _, img := range testImages {
		t.Run(img.name, func(t *testing.T) {
			versions, err := client.ListPackageVersions(ctx, owner, ownerType, img.name)
			require.NoError(t, err)

			t.Logf("%s (%s): %d versions", img.name, img.description, len(versions))
			assert.GreaterOrEqual(t, len(versions), img.expectMinVers)
		})
	}
}

// TestListPackageVersionsInputValidation tests input validation
func TestListPackageVersionsInputValidation(t *testing.T) {
	t.Parallel()
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		t.Skip("Skipping integration test - GITHUB_TOKEN not set")
	}

	client, err := NewClient(token)
	require.NoError(t, err, "Failed to create client")

	ctx := context.Background()

	tests := []struct {
		name        string
		owner       string
		ownerType   string
		packageName string
		wantErr     bool
		errContains string
	}{
		{
			name:        "empty owner",
			owner:       "",
			ownerType:   "user",
			packageName: "test",
			wantErr:     true,
			errContains: "owner cannot be empty",
		},
		{
			name:        "invalid owner type",
			owner:       "test",
			ownerType:   "invalid",
			packageName: "test",
			wantErr:     true,
			errContains: "owner type must be",
		},
		{
			name:        "empty package name",
			owner:       "test",
			ownerType:   "user",
			packageName: "",
			wantErr:     true,
			errContains: "package name cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := client.ListPackageVersions(ctx, tt.owner, tt.ownerType, tt.packageName)
			if tt.wantErr {
				require.Error(t, err)
				assert.ErrorContains(t, err, tt.errContains)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestListPackageVersionsNonexistentPackage tests error handling for non-existent package
func TestListPackageVersionsNonexistentPackage(t *testing.T) {
	t.Parallel()
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		t.Skip("Skipping integration test - GITHUB_TOKEN not set")
	}

	client, err := NewClient(token)
	require.NoError(t, err, "Failed to create client")

	ctx := context.Background()
	owner := "mkoepf"
	ownerType := "user"
	packageName := "this-package-definitely-does-not-exist-12345"

	_, err = client.ListPackageVersions(ctx, owner, ownerType, packageName)
	assert.Error(t, err, "Expected error for non-existent package")

	t.Logf("Got expected error for non-existent package: %v", err)
}

// ===[ GetOwnerType Tests ]===

// TestGetOwnerTypeUser tests that a known user returns "user"
func TestGetOwnerTypeUser(t *testing.T) {
	t.Parallel()
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		t.Skip("Skipping integration test - GITHUB_TOKEN not set")
	}

	client, err := NewClient(token)
	require.NoError(t, err, "Failed to create client")

	ctx := context.Background()
	ownerType, err := client.GetOwnerType(ctx, "mkoepf")
	require.NoError(t, err)
	assert.Equal(t, "user", ownerType)
}

// TestGetOwnerTypeOrg tests that a known organization returns "org"
func TestGetOwnerTypeOrg(t *testing.T) {
	t.Parallel()
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		t.Skip("Skipping integration test - GITHUB_TOKEN not set")
	}

	client, err := NewClient(token)
	require.NoError(t, err, "Failed to create client")

	ctx := context.Background()
	ownerType, err := client.GetOwnerType(ctx, "golang")
	require.NoError(t, err)
	assert.Equal(t, "org", ownerType)
}

// ===[ Version ID Mapping Tests ]===

// TestGetVersionIDByDigestWithRealImage uses a complete workflow
func TestGetVersionIDByDigestWithRealImage(t *testing.T) {
	t.Parallel()
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		t.Skip("Skipping integration test - GITHUB_TOKEN not set")
	}

	// This test needs access to ORAS to resolve a tag first
	// Since we can't import due to circular dependencies, we'll use a hardcoded digest
	// that we know exists from the workflow

	// Create client
	client, err := NewClient(token)
	require.NoError(t, err, "Failed to create client")

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
	require.NoError(t, err, "Failed to list package versions")

	if len(versions) == 0 {
		t.Skip("No package versions found - integration test images may not be created yet")
	}

	// Use the first version
	firstVersion := versions[0]
	require.NotNil(t, firstVersion.Name, "Version has nil name")
	require.NotNil(t, firstVersion.ID, "Version has nil ID")

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
	require.NoError(t, err, "Failed to map digest to version ID")

	// Verify the IDs match
	assert.Equal(t, expectedVersionID, actualVersionID)

	t.Logf("✓ Successfully verified digest->version ID mapping")
}

// TestVersionIDNotFound verifies error handling for non-existent digest
func TestVersionIDNotFound(t *testing.T) {
	t.Parallel()
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		t.Skip("Skipping integration test - GITHUB_TOKEN not set")
	}

	// Create client
	client, err := NewClient(token)
	require.NoError(t, err, "Failed to create client")

	ctx := context.Background()
	owner := "mkoepf"
	ownerType := "user"
	packageName := "ghcrctl-test-with-sbom"

	// Use a fake digest that definitely doesn't exist
	fakeDigest := "sha256:0000000000000000000000000000000000000000000000000000000000000000"

	// This should return an error
	versionID, err := client.GetVersionIDByDigest(ctx, owner, ownerType, packageName, fakeDigest)
	require.Error(t, err, "Expected error for non-existent digest")
	assert.Equal(t, int64(0), versionID)
	assert.ErrorContains(t, err, "no version found")

	t.Logf("✓ Correctly handled non-existent digest")
}
