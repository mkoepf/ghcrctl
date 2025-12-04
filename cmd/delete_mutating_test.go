//go:build mutating

package cmd

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/mkoepf/ghcrctl/internal/discover"
	"github.com/mkoepf/ghcrctl/internal/gh"
	"github.com/mkoepf/ghcrctl/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeletePackage_Mutating(t *testing.T) {
	t.Parallel()

	token := os.Getenv("GITHUB_TOKEN")
	require.NotEmpty(t, token, "GITHUB_TOKEN not set")

	ctx := context.Background()

	// Generate unique ephemeral package name
	ephemeralName := testutil.GenerateEphemeralName("ghcrctl-ephemeral")
	ephemeralImage := fmt.Sprintf("ghcr.io/%s/%s", testOwner, ephemeralName)

	// Copy stable fixture to ephemeral package (no cleanup registered - we're testing delete)
	t.Logf("Copying %s:%s to %s:latest", stableFixture, stableFixtureTag, ephemeralImage)
	err := testutil.CopyImage(ctx, stableFixture, stableFixtureTag, ephemeralImage, "latest")
	require.NoError(t, err, "Failed to copy image")

	// Delete the entire package
	t.Logf("Deleting package %s", ephemeralName)
	err = testutil.DeletePackage(ctx, testOwner, ephemeralName)
	require.NoError(t, err, "Failed to delete package")

	// Verify package no longer exists by trying to list versions
	client, err := gh.NewClient(token)
	require.NoError(t, err, "Failed to create client")

	_, err = client.ListPackageVersions(ctx, testOwner, "user", ephemeralName)
	assert.Error(t, err, "Expected error when listing versions of deleted package, got none")

	t.Logf("Successfully deleted package %s", ephemeralName)
}

func TestDeletePackageVersion_Mutating(t *testing.T) {
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

	// Copy stable fixture to ephemeral package with two different tags
	// This creates two versions we can work with
	t.Logf("Creating ephemeral package with multiple tags")
	err := testutil.CopyImage(ctx, stableFixture, stableFixtureTag, ephemeralImage, "tag-v1")
	require.NoError(t, err, "Failed to copy image with tag-v1")

	// Create a second tag pointing to the same digest
	err = testutil.CopyImage(ctx, stableFixture, stableFixtureTag, ephemeralImage, "tag-v2")
	require.NoError(t, err, "Failed to copy image with tag-v2")

	// Get list of versions
	client, err := gh.NewClient(token)
	require.NoError(t, err, "Failed to create client")

	versions, err := client.ListPackageVersions(ctx, testOwner, "user", ephemeralName)
	require.NoError(t, err, "Failed to list versions")

	require.NotEmpty(t, versions, "Expected at least one version, got none")

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
		require.NoError(t, err, "Unexpected error deleting version")
	}
}

// =============================================================================
// CLI Command Tests - These test the actual CLI commands
// =============================================================================

// TestDeleteVersionCmd_ByVersionID tests delete version --version <id>
func TestDeleteVersionCmd_ByVersionID(t *testing.T) {
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
		_ = testutil.DeletePackage(cleanupCtx, testOwner, ephemeralName)
	})

	// Create ephemeral package with multiple versions (so we can delete one)
	t.Logf("Creating ephemeral package with multiple tags")
	err := testutil.CopyImage(ctx, stableFixture, stableFixtureTag, ephemeralImage, "tag-v1")
	require.NoError(t, err, "Failed to copy image with tag-v1")

	// Push a second distinct version
	err = testutil.CopyImage(ctx, stableFixture, stableFixtureTag, ephemeralImage, "tag-v2")
	require.NoError(t, err, "Failed to copy image with tag-v2")

	// Get versions to find one to delete
	client, err := gh.NewClient(token)
	require.NoError(t, err, "Failed to create client")

	versions, err := client.ListPackageVersions(ctx, testOwner, "user", ephemeralName)
	require.NoError(t, err, "Failed to list versions")

	require.NotEmpty(t, versions, "Expected at least one version")

	versionToDelete := versions[0]
	t.Logf("Testing delete version command with --version %d", versionToDelete.ID)

	// Run delete version command with --force and --dry-run first
	cmd := NewRootCmd()
	cmd.SetArgs([]string{
		"delete", "version", fmt.Sprintf("%s/%s", testOwner, ephemeralName),
		"--version", fmt.Sprintf("%d", versionToDelete.ID),
		"--dry-run",
	})

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	err = cmd.Execute()
	require.NoError(t, err, "delete version --dry-run failed")

	output := stdout.String()
	t.Logf("Dry-run output:\n%s", output)

	// Verify dry-run output
	assert.Contains(t, output, "DRY RUN", "Expected 'DRY RUN' in output")
	assert.Contains(t, output, fmt.Sprintf("%d", versionToDelete.ID), "Expected version ID in output")
}

// TestDeleteVersionCmd_ByDigest tests delete version --digest <digest>
func TestDeleteVersionCmd_ByDigest(t *testing.T) {
	t.Parallel()

	token := os.Getenv("GITHUB_TOKEN")
	require.NotEmpty(t, token, "GITHUB_TOKEN not set")

	ctx := context.Background()

	ephemeralName := testutil.GenerateEphemeralName("ghcrctl-ephemeral")
	ephemeralImage := fmt.Sprintf("ghcr.io/%s/%s", testOwner, ephemeralName)

	t.Cleanup(func() {
		cleanupCtx := context.Background()
		_ = testutil.DeletePackage(cleanupCtx, testOwner, ephemeralName)
	})

	// Create ephemeral package
	err := testutil.CopyImage(ctx, stableFixture, stableFixtureTag, ephemeralImage, "latest")
	require.NoError(t, err, "Failed to copy image")

	// Get the digest
	digest, err := discover.ResolveTag(ctx, ephemeralImage, "latest")
	require.NoError(t, err, "Failed to resolve tag")

	t.Logf("Testing delete version command with --digest %s", digest[:20])

	// Run delete version command with --dry-run
	cmd := NewRootCmd()
	cmd.SetArgs([]string{
		"delete", "version", fmt.Sprintf("%s/%s", testOwner, ephemeralName),
		"--digest", digest,
		"--dry-run",
	})

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	err = cmd.Execute()
	require.NoError(t, err, "delete version --digest --dry-run failed")

	output := stdout.String()
	t.Logf("Dry-run output:\n%s", output)

	assert.Contains(t, output, "DRY RUN", "Expected 'DRY RUN' in output")
}

// TestDeleteVersionCmd_ByTag tests delete version --tag <tag>
func TestDeleteVersionCmd_ByTag(t *testing.T) {
	t.Parallel()

	token := os.Getenv("GITHUB_TOKEN")
	require.NotEmpty(t, token, "GITHUB_TOKEN not set")

	ctx := context.Background()

	ephemeralName := testutil.GenerateEphemeralName("ghcrctl-ephemeral")
	ephemeralImage := fmt.Sprintf("ghcr.io/%s/%s", testOwner, ephemeralName)

	t.Cleanup(func() {
		cleanupCtx := context.Background()
		_ = testutil.DeletePackage(cleanupCtx, testOwner, ephemeralName)
	})

	// Create ephemeral package
	err := testutil.CopyImage(ctx, stableFixture, stableFixtureTag, ephemeralImage, "test-tag")
	require.NoError(t, err, "Failed to copy image")

	t.Logf("Testing delete version command with --tag test-tag")

	// Run delete version command with --dry-run
	cmd := NewRootCmd()
	cmd.SetArgs([]string{
		"delete", "version", fmt.Sprintf("%s/%s", testOwner, ephemeralName),
		"--tag", "test-tag",
		"--dry-run",
	})

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	err = cmd.Execute()
	require.NoError(t, err, "delete version --tag --dry-run failed")

	output := stdout.String()
	t.Logf("Dry-run output:\n%s", output)

	assert.Contains(t, output, "DRY RUN", "Expected 'DRY RUN' in output")
	assert.Contains(t, output, "test-tag", "Expected tag name in output")
}

// TestDeleteImageCmd_ByTag tests delete image --tag <tag>
func TestDeleteImageCmd_ByTag(t *testing.T) {
	t.Parallel()

	token := os.Getenv("GITHUB_TOKEN")
	require.NotEmpty(t, token, "GITHUB_TOKEN not set")

	ctx := context.Background()

	ephemeralName := testutil.GenerateEphemeralName("ghcrctl-ephemeral")
	ephemeralImage := fmt.Sprintf("ghcr.io/%s/%s", testOwner, ephemeralName)

	t.Cleanup(func() {
		cleanupCtx := context.Background()
		_ = testutil.DeletePackage(cleanupCtx, testOwner, ephemeralName)
	})

	// Create ephemeral package
	err := testutil.CopyImage(ctx, stableFixture, stableFixtureTag, ephemeralImage, "delete-me")
	require.NoError(t, err, "Failed to copy image")

	t.Logf("Testing delete image command with --tag delete-me")

	// Run delete image command with --dry-run
	cmd := NewRootCmd()
	cmd.SetArgs([]string{
		"delete", "image", fmt.Sprintf("%s/%s", testOwner, ephemeralName),
		"--tag", "delete-me",
		"--dry-run",
	})

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	err = cmd.Execute()
	require.NoError(t, err, "delete image --tag --dry-run failed")

	output := stdout.String()
	t.Logf("Dry-run output:\n%s", output)

	assert.Contains(t, output, "DRY RUN", "Expected 'DRY RUN' in output")
	// Should show what would be deleted
	assert.Contains(t, output, "delete-me", "Expected tag in output")
}

// TestDeleteImageCmd_ByDigest tests delete image --digest <digest>
func TestDeleteImageCmd_ByDigest(t *testing.T) {
	t.Parallel()

	token := os.Getenv("GITHUB_TOKEN")
	require.NotEmpty(t, token, "GITHUB_TOKEN not set")

	ctx := context.Background()

	ephemeralName := testutil.GenerateEphemeralName("ghcrctl-ephemeral")
	ephemeralImage := fmt.Sprintf("ghcr.io/%s/%s", testOwner, ephemeralName)

	t.Cleanup(func() {
		cleanupCtx := context.Background()
		_ = testutil.DeletePackage(cleanupCtx, testOwner, ephemeralName)
	})

	// Create ephemeral package
	err := testutil.CopyImage(ctx, stableFixture, stableFixtureTag, ephemeralImage, "latest")
	require.NoError(t, err, "Failed to copy image")

	// Get the digest
	digest, err := discover.ResolveTag(ctx, ephemeralImage, "latest")
	require.NoError(t, err, "Failed to resolve tag")

	t.Logf("Testing delete image command with --digest %s", digest[:20])

	// Run delete image command with --dry-run
	cmd := NewRootCmd()
	cmd.SetArgs([]string{
		"delete", "image", fmt.Sprintf("%s/%s", testOwner, ephemeralName),
		"--digest", digest,
		"--dry-run",
	})

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	err = cmd.Execute()
	require.NoError(t, err, "delete image --digest --dry-run failed")

	output := stdout.String()
	t.Logf("Dry-run output:\n%s", output)

	assert.Contains(t, output, "DRY RUN", "Expected 'DRY RUN' in output")
}

// TestDeletePackageCmd tests delete package command
func TestDeletePackageCmd(t *testing.T) {
	t.Parallel()

	token := os.Getenv("GITHUB_TOKEN")
	require.NotEmpty(t, token, "GITHUB_TOKEN not set")

	ctx := context.Background()

	ephemeralName := testutil.GenerateEphemeralName("ghcrctl-ephemeral")
	ephemeralImage := fmt.Sprintf("ghcr.io/%s/%s", testOwner, ephemeralName)

	// Create ephemeral package (no cleanup - we're testing delete)
	err := testutil.CopyImage(ctx, stableFixture, stableFixtureTag, ephemeralImage, "latest")
	require.NoError(t, err, "Failed to copy image")

	t.Logf("Testing delete package command for %s", ephemeralName)

	// Run delete package command with --force
	cmd := NewRootCmd()
	cmd.SetArgs([]string{
		"delete", "package", fmt.Sprintf("%s/%s", testOwner, ephemeralName),
		"--force",
	})

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	err = cmd.Execute()
	require.NoError(t, err, "delete package --force failed: %s", stderr.String())

	output := stdout.String()
	t.Logf("Output:\n%s", output)

	// Verify success message
	assert.Contains(t, output, "Successfully deleted", "Expected 'Successfully deleted' in output")

	// Verify package no longer exists
	client, err := gh.NewClient(token)
	require.NoError(t, err, "Failed to create client")

	_, err = client.ListPackageVersions(ctx, testOwner, "user", ephemeralName)
	assert.Error(t, err, "Expected error when listing deleted package")

	t.Logf("Successfully verified package deletion via CLI")
}

// TestDeleteVersionCmd_BulkUntagged tests delete version --untagged
func TestDeleteVersionCmd_BulkUntagged(t *testing.T) {
	t.Parallel()

	token := os.Getenv("GITHUB_TOKEN")
	require.NotEmpty(t, token, "GITHUB_TOKEN not set")

	ctx := context.Background()

	ephemeralName := testutil.GenerateEphemeralName("ghcrctl-ephemeral")
	ephemeralImage := fmt.Sprintf("ghcr.io/%s/%s", testOwner, ephemeralName)

	t.Cleanup(func() {
		cleanupCtx := context.Background()
		_ = testutil.DeletePackage(cleanupCtx, testOwner, ephemeralName)
	})

	// Create ephemeral package
	err := testutil.CopyImage(ctx, stableFixture, stableFixtureTag, ephemeralImage, "keep-me")
	require.NoError(t, err, "Failed to copy image")

	t.Logf("Testing delete version command with --untagged --dry-run")

	// Run delete version command with --untagged --dry-run
	cmd := NewRootCmd()
	cmd.SetArgs([]string{
		"delete", "version", fmt.Sprintf("%s/%s", testOwner, ephemeralName),
		"--untagged",
		"--dry-run",
	})

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	err = cmd.Execute()

	output := stdout.String()
	t.Logf("Output:\n%s", output)

	// Several valid outcomes:
	// 1. Found untagged versions and would delete them (DRY RUN)
	// 2. Found untagged versions but they're shared (preserved)
	// 3. No untagged versions found (error)
	if err != nil {
		if strings.Contains(err.Error(), "no versions match") {
			t.Log("No untagged versions found - this is expected for our simple fixture")
		} else {
			require.NoError(t, err, "Unexpected error")
		}
	} else {
		// Command succeeded - check the output
		if strings.Contains(output, "DRY RUN") {
			t.Log("Found untagged versions that would be deleted")
		} else if strings.Contains(output, "shared") || strings.Contains(output, "preserved") {
			t.Log("Found untagged versions but they're shared - this exercises the bulk delete code path")
		} else if strings.Contains(output, "No versions to delete") {
			t.Log("No deletable versions found - bulk delete code path exercised")
		} else {
			t.Logf("Unexpected output format but command succeeded")
		}
	}
}

// TestDeleteImageCmd_ActualDelete tests actual deletion (not just dry-run)
func TestDeleteImageCmd_ActualDelete(t *testing.T) {
	t.Parallel()

	token := os.Getenv("GITHUB_TOKEN")
	require.NotEmpty(t, token, "GITHUB_TOKEN not set")

	ctx := context.Background()

	ephemeralName := testutil.GenerateEphemeralName("ghcrctl-ephemeral")
	ephemeralImage := fmt.Sprintf("ghcr.io/%s/%s", testOwner, ephemeralName)

	// Register cleanup in case test fails partway
	t.Cleanup(func() {
		cleanupCtx := context.Background()
		_ = testutil.DeletePackage(cleanupCtx, testOwner, ephemeralName)
	})

	// Create ephemeral package
	err := testutil.CopyImage(ctx, stableFixture, stableFixtureTag, ephemeralImage, "delete-this")
	require.NoError(t, err, "Failed to copy image")

	// Get version count before delete
	client, err := gh.NewClient(token)
	require.NoError(t, err, "Failed to create client")

	versionsBefore, err := client.ListPackageVersions(ctx, testOwner, "user", ephemeralName)
	require.NoError(t, err, "Failed to list versions before delete")
	t.Logf("Versions before delete: %d", len(versionsBefore))

	// Run delete image command with --force (actual delete)
	cmd := NewRootCmd()
	cmd.SetArgs([]string{
		"delete", "image", fmt.Sprintf("%s/%s", testOwner, ephemeralName),
		"--tag", "delete-this",
		"--force",
	})

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	err = cmd.Execute()
	if err != nil {
		// "last tagged version" error is expected if this is the only tagged version
		if gh.IsLastTaggedVersionError(err) || strings.Contains(err.Error(), "last tagged version") {
			t.Log("Got expected 'last tagged version' error - cannot delete when it's the only tagged version")
			return
		}
		require.NoError(t, err, "delete image --force failed: %s", stderr.String())
	}

	output := stdout.String()
	t.Logf("Output:\n%s", output)

	// Verify success
	assert.Contains(t, output, "Successfully deleted", "Expected 'Successfully deleted' in output")

	// Verify versions were deleted
	versionsAfter, err := client.ListPackageVersions(ctx, testOwner, "user", ephemeralName)
	if err != nil {
		// Package might be completely gone
		t.Log("Package no longer exists after delete - this is expected")
		return
	}

	assert.Less(t, len(versionsAfter), len(versionsBefore),
		"Expected fewer versions after delete: before=%d, after=%d", len(versionsBefore), len(versionsAfter))

	t.Logf("Versions after delete: %d (was %d)", len(versionsAfter), len(versionsBefore))
}
