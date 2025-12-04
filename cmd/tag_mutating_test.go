//go:build mutating

package cmd

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/mkoepf/ghcrctl/internal/discover"
	"github.com/mkoepf/ghcrctl/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testOwner        = "mkoepf"
	stableFixture    = "ghcr.io/mkoepf/ghcrctl-test-no-sbom"
	stableFixtureTag = "latest"
)

func TestAddTag_Mutating(t *testing.T) {
	t.Parallel()

	token := os.Getenv("GITHUB_TOKEN")
	require.NotEmpty(t, token, "GITHUB_TOKEN not set")

	ctx := context.Background()

	// Generate unique ephemeral package name
	ephemeralName := testutil.GenerateEphemeralName("ghcrctl-ephemeral")
	ephemeralImage := fmt.Sprintf("ghcr.io/%s/%s", testOwner, ephemeralName)

	// Register cleanup BEFORE any operations
	t.Cleanup(func() {
		cleanupCtx := context.Background()
		err := testutil.DeletePackage(cleanupCtx, testOwner, ephemeralName)
		if err != nil {
			t.Logf("cleanup warning: failed to delete package %s: %v", ephemeralName, err)
		}
	})

	// Copy stable fixture to ephemeral package
	t.Logf("Copying %s:%s to %s:latest", stableFixture, stableFixtureTag, ephemeralImage)
	err := testutil.CopyImage(ctx, stableFixture, stableFixtureTag, ephemeralImage, "latest")
	require.NoError(t, err, "Failed to copy image")

	// Get the digest of the copied image
	digest, err := discover.ResolveTag(ctx, ephemeralImage, "latest")
	require.NoError(t, err, "Failed to resolve tag")
	t.Logf("Ephemeral image digest: %s", digest)

	// Add a new tag using AddTagByDigest
	newTag := "test-tag-v1"
	t.Logf("Adding tag %s to %s", newTag, ephemeralImage)
	err = discover.AddTagByDigest(ctx, ephemeralImage, digest, newTag)
	require.NoError(t, err, "Failed to add tag")

	// Verify the new tag resolves to the same digest
	resolvedDigest, err := discover.ResolveTag(ctx, ephemeralImage, newTag)
	require.NoError(t, err, "Failed to resolve new tag")

	assert.Equal(t, digest, resolvedDigest, "New tag resolved to different digest")

	t.Logf("Successfully added tag %s pointing to %s", newTag, digest[:20])
}
