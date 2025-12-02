//go:build mutating

package cmd

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/mkoepf/ghcrctl/internal/discover"
	"github.com/mkoepf/ghcrctl/internal/testutil"
)

const (
	testOwner        = "mkoepf"
	stableFixture    = "ghcr.io/mkoepf/ghcrctl-test-no-sbom"
	stableFixtureTag = "latest"
)

func TestAddTag_Mutating(t *testing.T) {
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
	if err != nil {
		t.Fatalf("Failed to copy image: %v", err)
	}

	// Get the digest of the copied image
	digest, err := discover.ResolveTag(ctx, ephemeralImage, "latest")
	if err != nil {
		t.Fatalf("Failed to resolve tag: %v", err)
	}
	t.Logf("Ephemeral image digest: %s", digest)

	// Add a new tag using AddTagByDigest
	newTag := "test-tag-v1"
	t.Logf("Adding tag %s to %s", newTag, ephemeralImage)
	err = discover.AddTagByDigest(ctx, ephemeralImage, digest, newTag)
	if err != nil {
		t.Fatalf("Failed to add tag: %v", err)
	}

	// Verify the new tag resolves to the same digest
	resolvedDigest, err := discover.ResolveTag(ctx, ephemeralImage, newTag)
	if err != nil {
		t.Fatalf("Failed to resolve new tag: %v", err)
	}

	if resolvedDigest != digest {
		t.Errorf("New tag resolved to different digest: got %s, want %s", resolvedDigest, digest)
	}

	t.Logf("Successfully added tag %s pointing to %s", newTag, digest[:20])
}

func TestAddTagFromTag_Mutating(t *testing.T) {
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

	// Copy stable fixture to ephemeral package
	err := testutil.CopyImage(ctx, stableFixture, stableFixtureTag, ephemeralImage, "source-tag")
	if err != nil {
		t.Fatalf("Failed to copy image: %v", err)
	}

	// Get the digest of the source tag
	sourceDigest, err := discover.ResolveTag(ctx, ephemeralImage, "source-tag")
	if err != nil {
		t.Fatalf("Failed to resolve source tag: %v", err)
	}

	// Add a new tag using AddTag (tag-to-tag)
	destTag := "dest-tag"
	err = discover.AddTag(ctx, ephemeralImage, "source-tag", destTag)
	if err != nil {
		t.Fatalf("Failed to add tag from tag: %v", err)
	}

	// Verify the destination tag resolves to the same digest
	destDigest, err := discover.ResolveTag(ctx, ephemeralImage, destTag)
	if err != nil {
		t.Fatalf("Failed to resolve dest tag: %v", err)
	}

	if destDigest != sourceDigest {
		t.Errorf("Dest tag resolved to different digest: got %s, want %s", destDigest, sourceDigest)
	}

	t.Logf("Successfully copied tag source-tag -> %s", destTag)
}
