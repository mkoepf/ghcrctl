//go:build mutating

package cmd

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/mkoepf/ghcrctl/internal/gh"
	"github.com/mkoepf/ghcrctl/internal/testutil"
)

func TestDeletePackage_Mutating(t *testing.T) {
	// Note: t.Parallel() is disabled due to race conditions in oras-go library
	// when multiple copies run concurrently (internal http2PusherState race)

	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		t.Fatal("GITHUB_TOKEN not set")
	}

	ctx := context.Background()

	// Generate unique ephemeral package name
	ephemeralName := testutil.GenerateEphemeralName("ghcrctl-ephemeral")
	ephemeralImage := fmt.Sprintf("ghcr.io/%s/%s", testOwner, ephemeralName)

	// Copy stable fixture to ephemeral package (no cleanup registered - we're testing delete)
	t.Logf("Copying %s:%s to %s:latest", stableFixture, stableFixtureTag, ephemeralImage)
	err := testutil.CopyImage(ctx, stableFixture, stableFixtureTag, ephemeralImage, "latest")
	if err != nil {
		t.Fatalf("Failed to copy image: %v", err)
	}

	// Delete the entire package
	t.Logf("Deleting package %s", ephemeralName)
	err = testutil.DeletePackage(ctx, testOwner, ephemeralName)
	if err != nil {
		t.Fatalf("Failed to delete package: %v", err)
	}

	// Verify package no longer exists by trying to list versions
	client, err := gh.NewClient(token)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	_, err = client.ListPackageVersions(ctx, testOwner, "user", ephemeralName)
	if err == nil {
		t.Error("Expected error when listing versions of deleted package, got none")
	}

	t.Logf("Successfully deleted package %s", ephemeralName)
}

func TestDeletePackageVersion_Mutating(t *testing.T) {
	// Note: t.Parallel() is disabled due to race conditions in oras-go library

	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		t.Fatal("GITHUB_TOKEN not set")
	}

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

	// Copy stable fixture to ephemeral package with two different tags
	// This creates two versions we can work with
	t.Logf("Creating ephemeral package with multiple tags")
	err := testutil.CopyImage(ctx, stableFixture, stableFixtureTag, ephemeralImage, "tag-v1")
	if err != nil {
		t.Fatalf("Failed to copy image with tag-v1: %v", err)
	}

	// Create a second tag pointing to the same digest
	err = testutil.CopyImage(ctx, stableFixture, stableFixtureTag, ephemeralImage, "tag-v2")
	if err != nil {
		t.Fatalf("Failed to copy image with tag-v2: %v", err)
	}

	// Get list of versions
	client, err := gh.NewClient(token)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	versions, err := client.ListPackageVersions(ctx, testOwner, "user", ephemeralName)
	if err != nil {
		t.Fatalf("Failed to list versions: %v", err)
	}

	if len(versions) == 0 {
		t.Fatal("Expected at least one version, got none")
	}

	t.Logf("Found %d version(s)", len(versions))

	// Try to delete a version
	// Note: GHCR doesn't allow deleting the last tagged version
	// Since both tags point to the same digest, we have only 1 version
	// Attempting to delete it should fail with "last tagged version" error
	versionToDelete := versions[0]
	t.Logf("Attempting to delete version %d (digest: %s)", versionToDelete.ID, versionToDelete.Name[:20])

	err = client.DeletePackageVersion(ctx, testOwner, "user", ephemeralName, versionToDelete.ID)
	if err == nil {
		t.Log("Version deleted successfully (package had multiple versions)")
	} else if gh.IsLastTaggedVersionError(err) {
		t.Log("Got expected 'last tagged version' error - this confirms the delete API works")
	} else {
		t.Fatalf("Unexpected error deleting version: %v", err)
	}
}
