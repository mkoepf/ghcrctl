package cmd

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/mkoepf/ghcrctl/internal/discover"
	"github.com/mkoepf/ghcrctl/internal/gh"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDeleteCommandStructure verifies the delete command is properly set up
func TestDeleteCommandStructure(t *testing.T) {
	t.Parallel()
	cmd := NewRootCmd()
	deleteCmd, _, err := cmd.Find([]string{"delete"})
	require.NoError(t, err, "Failed to find delete command")

	assert.NotEmpty(t, deleteCmd.Short, "Short description should not be empty")

	// Check for subcommands
	subcommands := deleteCmd.Commands()
	assert.GreaterOrEqual(t, len(subcommands), 3, "Expected at least 3 subcommands (version, image, package)")
}

// TestDeleteVersionCommandStructure verifies the delete version subcommand
func TestDeleteVersionCommandStructure(t *testing.T) {
	t.Parallel()
	cmd := NewRootCmd()
	deleteVersionCmd, _, err := cmd.Find([]string{"delete", "version"})
	require.NoError(t, err, "Failed to find delete version command")

	assert.Equal(t, "version <owner/package>", deleteVersionCmd.Use)
	assert.NotEmpty(t, deleteVersionCmd.Short, "Short description should not be empty")
}

// TestDeleteVersionCommandArguments verifies argument validation
func TestDeleteVersionCommandArguments(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		args        []string
		expectUsage bool
	}{
		{
			name:        "missing all arguments",
			args:        []string{"delete", "version"},
			expectUsage: true,
		},
		{
			name:        "missing version-id argument",
			args:        []string{"delete", "version", "mkoepf/myimage"},
			expectUsage: true,
		},
		{
			name:        "too many arguments",
			args:        []string{"delete", "version", "mkoepf/myimage", "12345", "extra"},
			expectUsage: true,
		},
	}

	for _, tt := range tests {
		tt := tt // capture range variable
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cmd := NewRootCmd()
			cmd.SetArgs(tt.args)
			err := cmd.Execute()

			// Should fail with usage error
			assert.Error(t, err, "Expected error but got none")
		})
	}
}

// TestDeleteVersionCommandHasFlags verifies required flags exist
func TestDeleteVersionCommandHasFlags(t *testing.T) {
	t.Parallel()
	cmd := NewRootCmd()
	deleteVersionCmd, _, err := cmd.Find([]string{"delete", "version"})
	require.NoError(t, err, "Failed to find delete version command")

	requiredFlags := []string{
		"force",
		"yes",
		"dry-run",
		"digest",
		"tag-pattern",
		"tagged",
		"untagged",
		"older-than",
		"newer-than",
		"older-than-days",
		"newer-than-days",
	}

	for _, flagName := range requiredFlags {
		flag := deleteVersionCmd.Flags().Lookup(flagName)
		assert.NotNil(t, flag, "delete version command should have --%s flag", flagName)
	}
}

// TestDeletePackageCommandStructure verifies the delete package subcommand
func TestDeletePackageCommandStructure(t *testing.T) {
	t.Parallel()
	cmd := NewRootCmd()
	deletePackageCmd, _, err := cmd.Find([]string{"delete", "package"})
	require.NoError(t, err, "Failed to find delete package command")

	assert.Equal(t, "package <owner/package>", deletePackageCmd.Use)
	assert.NotEmpty(t, deletePackageCmd.Short, "Short description should not be empty")
}

// TestDeletePackageCommandHasFlags verifies required flags exist
func TestDeletePackageCommandHasFlags(t *testing.T) {
	t.Parallel()
	cmd := NewRootCmd()
	deletePackageCmd, _, err := cmd.Find([]string{"delete", "package"})
	require.NoError(t, err, "Failed to find delete package command")

	requiredFlags := []string{"force", "yes"}
	for _, flagName := range requiredFlags {
		flag := deletePackageCmd.Flags().Lookup(flagName)
		assert.NotNil(t, flag, "delete package command should have --%s flag", flagName)
	}
}

// TestDeleteImageCommandStructure verifies the delete image subcommand
func TestDeleteImageCommandStructure(t *testing.T) {
	t.Parallel()
	cmd := NewRootCmd()
	deleteImageCmd, _, err := cmd.Find([]string{"delete", "image"})
	require.NoError(t, err, "Failed to find delete image command")

	assert.Equal(t, "image <owner/package>", deleteImageCmd.Use)
	assert.NotEmpty(t, deleteImageCmd.Short, "Short description should not be empty")
}

// TestDeleteImageCommandArguments verifies argument validation
func TestDeleteImageCommandArguments(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		args        []string
		expectUsage bool
	}{
		{
			name:        "missing all arguments",
			args:        []string{"delete", "image"},
			expectUsage: true,
		},
		{
			name:        "too many arguments",
			args:        []string{"delete", "image", "mkoepf/myimage:v1.0.0", "extra"},
			expectUsage: true,
		},
	}

	for _, tt := range tests {
		tt := tt // capture range variable
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cmd := NewRootCmd()
			cmd.SetArgs(tt.args)
			err := cmd.Execute()

			// Should fail with usage error
			assert.Error(t, err, "Expected error but got none")
		})
	}
}

// TestDeleteImageCommandHasFlags verifies required flags exist
func TestDeleteImageCommandHasFlags(t *testing.T) {
	t.Parallel()
	cmd := NewRootCmd()
	deleteImageCmd, _, err := cmd.Find([]string{"delete", "image"})
	require.NoError(t, err, "Failed to find delete image command")

	// Check for --force flag
	forceFlag := deleteImageCmd.Flags().Lookup("force")
	assert.NotNil(t, forceFlag, "Expected --force flag to exist")

	// Check for --yes flag (alias for --force)
	yesFlag := deleteImageCmd.Flags().Lookup("yes")
	assert.NotNil(t, yesFlag, "Expected --yes flag to exist")

	// Check for -y shorthand
	yShorthand := deleteImageCmd.Flags().ShorthandLookup("y")
	assert.NotNil(t, yShorthand, "Expected -y shorthand for --yes flag")

	// Check for --dry-run flag
	dryRunFlag := deleteImageCmd.Flags().Lookup("dry-run")
	assert.NotNil(t, dryRunFlag, "Expected --dry-run flag to exist")

	// Check for --digest flag
	digestFlag := deleteImageCmd.Flags().Lookup("digest")
	assert.NotNil(t, digestFlag, "Expected --digest flag to exist")

	// Check for --version flag
	versionFlag := deleteImageCmd.Flags().Lookup("version")
	assert.NotNil(t, versionFlag, "Expected --version flag to exist")
}

// TestDeleteImageCommandFlagExclusivity verifies mutually exclusive flags
func TestDeleteImageCommandFlagExclusivity(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		args      []string
		expectErr bool
	}{
		{
			name:      "digest and version flags both set",
			args:      []string{"delete", "image", "mkoepf/myimage", "--digest", "sha256:abc", "--version", "12345"},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt // capture range variable
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cmd := NewRootCmd()
			cmd.SetArgs(tt.args)
			err := cmd.Execute()

			if tt.expectErr {
				assert.Error(t, err, "Expected error for mutually exclusive flags, got none")
			}
		})
	}
}

// TestBuildDeleteFilter verifies that the filter is built correctly from flags
func TestBuildDeleteFilter(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		tagPattern    string
		onlyTagged    bool
		onlyUntagged  bool
		olderThan     string
		newerThan     string
		olderThanDays int
		newerThanDays int
		wantErr       bool
		errContains   string
	}{
		{
			name:    "no filters",
			wantErr: false,
		},
		{
			name:         "conflicting tagged/untagged flags",
			onlyTagged:   true,
			onlyUntagged: true,
			wantErr:      true,
			errContains:  "cannot use --tagged and --untagged together",
		},
		{
			name:        "invalid older-than date",
			olderThan:   "invalid-date",
			wantErr:     true,
			errContains: "invalid --older-than date format",
		},
		{
			name:      "valid older-than date RFC3339",
			olderThan: "2025-01-01T00:00:00Z",
			wantErr:   false,
		},
		{
			name:      "valid older-than date (date only)",
			olderThan: "2025-01-01",
			wantErr:   false,
		},
		{
			name:      "valid newer-than date (date only)",
			newerThan: "2025-11-01",
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter, err := buildDeleteVersionFilter(tt.tagPattern, tt.onlyTagged, tt.onlyUntagged,
				tt.olderThan, tt.newerThan, tt.olderThanDays, tt.newerThanDays)

			if tt.wantErr {
				require.Error(t, err, "Expected error but got none")
				if tt.errContains != "" {
					assert.ErrorContains(t, err, tt.errContains)
				}
			} else {
				require.NoError(t, err, "Unexpected error")
				assert.NotNil(t, filter, "Expected non-nil filter")
			}
		})
	}
}

// TestDeleteVersionBulkModeArgsValidation tests that bulk mode accepts only image name
func TestDeleteVersionBulkModeArgsValidation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		args      []string
		expectErr bool
	}{
		{
			name:      "bulk mode with untagged flag - correct args",
			args:      []string{"delete", "version", "mkoepf/myimage", "--untagged"},
			expectErr: false,
		},
		{
			name:      "bulk mode with untagged flag - too many args",
			args:      []string{"delete", "version", "mkoepf/myimage", "12345", "--untagged"},
			expectErr: true,
		},
		{
			name:      "bulk mode with tag pattern - correct args",
			args:      []string{"delete", "version", "mkoepf/myimage", "--tag-pattern", ".*-rc.*"},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test args validation
			cmd := NewRootCmd()
			cmd.SetArgs(tt.args)
			err := cmd.Execute()

			// We expect configuration errors since we're not providing real tokens/config
			// But we should not get args validation errors if expectErr is false
			if !tt.expectErr && err != nil {
				// Check if error is about args validation (not config/auth errors)
				errStr := err.Error()
				isArgsError := strings.Contains(errStr, "accepts") || strings.Contains(errStr, "arg")
				assert.False(t, isArgsError, "Unexpected args validation error: %v", err)
			}
		})
	}
}

// TestFormatTagsForDisplay tests the tags formatting function
func TestFormatTagsForDisplay(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		tags []string
		want string
	}{
		{
			name: "no tags should show []",
			tags: []string{},
			want: "[]",
		},
		{
			name: "nil tags should show []",
			tags: nil,
			want: "[]",
		},
		{
			name: "single tag",
			tags: []string{"v1.0"},
			want: "v1.0",
		},
		{
			name: "multiple tags",
			tags: []string{"v1.0", "latest"},
			want: "v1.0, latest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatTagsForDisplay(tt.tags)
			assert.Equal(t, tt.want, got, "formatTagsForDisplay()")
		})
	}
}

// =============================================================================
// Tests for displayImageSummary using discover.VersionInfo
// =============================================================================

func TestDisplayImageVersions(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		toDelete       []discover.VersionInfo
		shared         []discover.VersionInfo
		imageVersions  []discover.VersionInfo
		wantContains   []string
		wantNotContain []string
	}{
		{
			name:     "single version to delete",
			toDelete: []discover.VersionInfo{{ID: 100, Digest: "sha256:rootdigest123", Tags: []string{"v1.0.0"}, Types: []string{"index"}}},
			shared:   nil,
			imageVersions: []discover.VersionInfo{
				{ID: 100, Digest: "sha256:rootdigest123", Tags: []string{"v1.0.0"}, Types: []string{"index"}},
			},
			wantContains: []string{
				"Versions to delete (1)",
				"index (version 100) [v1.0.0]",
			},
			wantNotContain: []string{
				"Shared versions",
			},
		},
		{
			name: "multiple exclusive versions",
			toDelete: []discover.VersionInfo{
				{ID: 100, Digest: "sha256:indexdigest", Tags: []string{"latest"}, Types: []string{"index"}},
				{ID: 101, Digest: "sha256:amd64", Types: []string{"linux/amd64"}},
				{ID: 102, Digest: "sha256:arm64", Types: []string{"linux/arm64"}},
			},
			shared: nil,
			imageVersions: []discover.VersionInfo{
				{ID: 100, Digest: "sha256:indexdigest", Tags: []string{"latest"}, Types: []string{"index"}},
				{ID: 101, Digest: "sha256:amd64", Types: []string{"linux/amd64"}},
				{ID: 102, Digest: "sha256:arm64", Types: []string{"linux/arm64"}},
			},
			wantContains: []string{
				"Versions to delete (3)",
				"index (version 100) [latest]",
				"linux/amd64 (version 101)",
				"linux/arm64 (version 102)",
			},
			wantNotContain: []string{
				"Shared versions",
			},
		},
		{
			name: "versions with attestations",
			toDelete: []discover.VersionInfo{
				{ID: 200, Digest: "sha256:manifestdigest", Types: []string{"manifest"}},
				{ID: 201, Digest: "sha256:sbomdigest", Types: []string{"sbom"}},
				{ID: 202, Digest: "sha256:provdigest", Types: []string{"provenance"}},
			},
			shared: nil,
			imageVersions: []discover.VersionInfo{
				{ID: 200, Digest: "sha256:manifestdigest", Types: []string{"manifest"}},
				{ID: 201, Digest: "sha256:sbomdigest", Types: []string{"sbom"}},
				{ID: 202, Digest: "sha256:provdigest", Types: []string{"provenance"}},
			},
			wantContains: []string{
				"Versions to delete (3)",
				"manifest (version 200)",
				"sbom (version 201)",
				"provenance (version 202)",
			},
			wantNotContain: []string{
				"Shared versions",
			},
		},
		{
			name: "image with shared platforms (preserved)",
			toDelete: []discover.VersionInfo{
				{ID: 300, Digest: "sha256:rootwithshared", Tags: []string{"v2.0"}, Types: []string{"index"}},
				{ID: 301, Digest: "sha256:exclusive", Types: []string{"linux/amd64"}},
			},
			shared: []discover.VersionInfo{
				{ID: 302, Digest: "sha256:shared", Types: []string{"linux/arm64"}, IncomingRefs: []string{"sha256:rootwithshared", "sha256:otherroot1", "sha256:otherroot2"}},
			},
			imageVersions: []discover.VersionInfo{
				{ID: 300, Digest: "sha256:rootwithshared", Tags: []string{"v2.0"}, Types: []string{"index"}, OutgoingRefs: []string{"sha256:exclusive", "sha256:shared"}},
				{ID: 301, Digest: "sha256:exclusive", Types: []string{"linux/amd64"}, IncomingRefs: []string{"sha256:rootwithshared"}},
				{ID: 302, Digest: "sha256:shared", Types: []string{"linux/arm64"}, IncomingRefs: []string{"sha256:rootwithshared", "sha256:otherroot1", "sha256:otherroot2"}},
			},
			wantContains: []string{
				"Versions to delete (2)",
				"index (version 300) [v2.0]",
				"linux/amd64 (version 301)",
				"Shared versions (preserved)",
				"linux/arm64 (version 302, referenced by 2 versions outside this delete)",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf strings.Builder
			displayDeleteImageVersions(&buf, tt.toDelete, tt.shared, tt.imageVersions)
			output := buf.String()

			for _, want := range tt.wantContains {
				assert.Contains(t, output, want, "Expected output to contain %q", want)
			}

			for _, notWant := range tt.wantNotContain {
				assert.NotContains(t, output, notWant, "Expected output NOT to contain %q", notWant)
			}
		})
	}
}

// =============================================================================
// Mock implementations for testing
// =============================================================================

// mockPackageDeleter implements gh.PackageDeleter for testing
type mockPackageDeleter struct {
	deletedVersions []int64
	deleteErrors    map[int64]error // Map of versionID -> error to return
	callCount       int
}

func newMockPackageDeleter() *mockPackageDeleter {
	return &mockPackageDeleter{
		deletedVersions: []int64{},
		deleteErrors:    make(map[int64]error),
	}
}

func (m *mockPackageDeleter) DeletePackageVersion(ctx context.Context, owner, ownerType, packageName string, versionID int64) error {
	m.callCount++
	if err, ok := m.deleteErrors[versionID]; ok {
		return err
	}
	m.deletedVersions = append(m.deletedVersions, versionID)
	return nil
}

// =============================================================================
// Tests for deleteGraphWithDeleter
// =============================================================================

func TestDeleteGraphWithDeleter(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		versionIDs     []int64
		deleteErrors   map[int64]error
		wantDeleted    []int64
		wantErr        bool
		wantErrContain string
		wantOutput     []string
	}{
		{
			name:        "successful deletion of all versions",
			versionIDs:  []int64{201, 202, 100}, // attestations, platform, root
			wantDeleted: []int64{201, 202, 100},
			wantErr:     false,
			wantOutput: []string{
				"Deleting version 1/3 (ID: 201)",
				"Deleting version 2/3 (ID: 202)",
				"Deleting version 3/3 (ID: 100)",
			},
		},
		{
			name:        "single version deletion",
			versionIDs:  []int64{12345},
			wantDeleted: []int64{12345},
			wantErr:     false,
			wantOutput:  []string{"Deleting version 1/1 (ID: 12345)"},
		},
		{
			name:        "empty version list succeeds without action",
			versionIDs:  []int64{},
			wantDeleted: []int64{},
			wantErr:     false,
		},
		{
			name:           "stops on first error",
			versionIDs:     []int64{201, 202, 100},
			deleteErrors:   map[int64]error{202: fmt.Errorf("permission denied")},
			wantDeleted:    []int64{201}, // only first one succeeds before error
			wantErr:        true,
			wantErrContain: "failed to delete version 202",
			wantOutput:     []string{"Deleting version 1/3", "Deleting version 2/3"},
		},
		{
			name:           "error on first version",
			versionIDs:     []int64{100, 200},
			deleteErrors:   map[int64]error{100: fmt.Errorf("not found")},
			wantDeleted:    []int64{},
			wantErr:        true,
			wantErrContain: "failed to delete version 100",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := newMockPackageDeleter()
			mock.deleteErrors = tt.deleteErrors

			var buf strings.Builder
			ctx := context.Background()

			err := DeleteGraphWithDeleter(ctx, mock, "owner", "user", "image", tt.versionIDs, &buf)

			// Check error
			if tt.wantErr {
				require.Error(t, err)
				if tt.wantErrContain != "" {
					assert.ErrorContains(t, err, tt.wantErrContain)
				}
			} else {
				require.NoError(t, err)
			}

			// Check deleted versions
			assert.Len(t, mock.deletedVersions, len(tt.wantDeleted), "deleted versions count mismatch")
			for i, id := range tt.wantDeleted {
				if i < len(mock.deletedVersions) {
					assert.Equal(t, id, mock.deletedVersions[i], "deletedVersions[%d] mismatch", i)
				}
			}

			// Check output
			output := buf.String()
			for _, want := range tt.wantOutput {
				assert.Contains(t, output, want, "output missing expected string")
			}
		})
	}
}

// =============================================================================
// Tests for executeSingleDelete
// =============================================================================

func TestExecuteSingleDelete(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name            string
		params          DeleteVersionParams
		confirmResponse bool
		confirmErr      error
		deleteErr       error
		wantDeleted     bool
		wantErr         bool
		wantOutput      []string
		wantNotOutput   []string
	}{
		{
			name: "successful deletion with force",
			params: DeleteVersionParams{
				Owner:      "testowner",
				OwnerType:  "user",
				ImageName:  "testimage",
				VersionID:  12345,
				Tags:       []string{"v1.0", "latest"},
				ImageCount: 0,
				Force:      true,
				DryRun:     false,
			},
			wantDeleted: true,
			wantErr:     false,
			wantOutput: []string{
				"Preparing to delete package version",
				"Image:      testimage",
				"Owner:      testowner (user)",
				"Version ID: 12345",
				"Tags:       v1.0, latest",
				"Successfully deleted version 12345",
			},
		},
		{
			name: "dry run does not delete",
			params: DeleteVersionParams{
				Owner:     "testowner",
				OwnerType: "org",
				ImageName: "testimage",
				VersionID: 67890,
				Tags:      []string{},
				Force:     false,
				DryRun:    true,
			},
			wantDeleted: false,
			wantErr:     false,
			wantOutput: []string{
				"DRY RUN: No changes made",
				"Tags:       []",
			},
			wantNotOutput: []string{
				"Successfully deleted",
			},
		},
		{
			name: "confirmed deletion",
			params: DeleteVersionParams{
				Owner:     "testowner",
				OwnerType: "user",
				ImageName: "testimage",
				VersionID: 11111,
				Tags:      []string{"test"},
				Force:     false,
				DryRun:    false,
			},
			confirmResponse: true,
			wantDeleted:     true,
			wantErr:         false,
			wantOutput:      []string{"Successfully deleted version 11111"},
		},
		{
			name: "cancelled by user",
			params: DeleteVersionParams{
				Owner:     "testowner",
				OwnerType: "user",
				ImageName: "testimage",
				VersionID: 22222,
				Tags:      []string{},
				Force:     false,
				DryRun:    false,
			},
			confirmResponse: false,
			wantDeleted:     false,
			wantErr:         false,
			wantOutput:      []string{"Deletion cancelled"},
			wantNotOutput:   []string{"Successfully deleted"},
		},
		{
			name: "confirmation error",
			params: DeleteVersionParams{
				Owner:     "testowner",
				OwnerType: "user",
				ImageName: "testimage",
				VersionID: 33333,
				Tags:      []string{},
				Force:     false,
				DryRun:    false,
			},
			confirmErr:  fmt.Errorf("stdin closed"),
			wantDeleted: false,
			wantErr:     true,
		},
		{
			name: "delete API error",
			params: DeleteVersionParams{
				Owner:     "testowner",
				OwnerType: "user",
				ImageName: "testimage",
				VersionID: 44444,
				Tags:      []string{},
				Force:     true,
				DryRun:    false,
			},
			deleteErr:   fmt.Errorf("permission denied"),
			wantDeleted: false,
			wantErr:     true,
		},
		{
			name: "shows ref count when present",
			params: DeleteVersionParams{
				Owner:      "testowner",
				OwnerType:  "user",
				ImageName:  "testimage",
				VersionID:  55555,
				Tags:       []string{},
				ImageCount: 2,
				Force:      true,
				DryRun:     false,
			},
			wantDeleted: true,
			wantErr:     false,
			wantOutput:  []string{"Referenced:", "by 2 other versions"},
		},
		{
			name: "shows singular version when count is 1",
			params: DeleteVersionParams{
				Owner:      "testowner",
				OwnerType:  "user",
				ImageName:  "testimage",
				VersionID:  66666,
				Tags:       []string{},
				ImageCount: 1,
				Force:      true,
				DryRun:     false,
			},
			wantDeleted: true,
			wantErr:     false,
			wantOutput:  []string{"by 1 other version"},
		},
		{
			name: "last tagged version error shows helpful message",
			params: DeleteVersionParams{
				Owner:     "testowner",
				OwnerType: "user",
				ImageName: "testimage",
				VersionID: 77777,
				Tags:      []string{"latest"},
				Force:     true,
				DryRun:    false,
			},
			deleteErr:   fmt.Errorf("You cannot delete the last tagged version of a package"),
			wantDeleted: false,
			wantErr:     true,
			wantOutput:  []string{"GHCR does not allow", "delete package", "testowner/testimage"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := newMockPackageDeleter()
			if tt.deleteErr != nil {
				mock.deleteErrors[tt.params.VersionID] = tt.deleteErr
			}

			var buf strings.Builder
			ctx := context.Background()

			confirmFn := func() (bool, error) {
				return tt.confirmResponse, tt.confirmErr
			}

			err := ExecuteSingleDelete(ctx, mock, tt.params, &buf, confirmFn)

			// Check error
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			// Check deletion
			deleted := len(mock.deletedVersions) > 0
			assert.Equal(t, tt.wantDeleted, deleted, "deletion state mismatch")

			// Check output
			output := buf.String()
			for _, want := range tt.wantOutput {
				assert.Contains(t, output, want, "output missing expected string")
			}
			for _, notWant := range tt.wantNotOutput {
				assert.NotContains(t, output, notWant, "output should NOT contain")
			}
		})
	}
}

// =============================================================================
// Tests for executeBulkDelete
// =============================================================================

func TestExecuteBulkDelete(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name            string
		params          BulkDeleteParams
		confirmResponse bool
		confirmErr      error
		deleteErrors    map[int64]error
		wantDeleted     []int64
		wantErr         bool
		wantOutput      []string
		wantNotOutput   []string
	}{
		{
			name: "successful bulk deletion with force",
			params: BulkDeleteParams{
				Owner:     "testowner",
				OwnerType: "user",
				ImageName: "testimage",
				Versions: []gh.PackageVersionInfo{
					{ID: 100, Tags: []string{"v1.0"}, CreatedAt: "2025-01-01"},
					{ID: 101, Tags: []string{}, CreatedAt: "2025-01-02"},
					{ID: 102, Tags: []string{"v2.0"}, CreatedAt: "2025-01-03"},
				},
				Force:  true,
				DryRun: false,
			},
			wantDeleted: []int64{100, 101, 102},
			wantErr:     false,
			wantOutput: []string{
				"Preparing to delete 3 package version(s)",
				"Image: testimage",
				"Owner: testowner (user)",
				"ID: 100, Tags: v1.0",
				"ID: 101, Tags: []",
				"ID: 102, Tags: v2.0",
				"Deleting version 1/3",
				"Deleting version 2/3",
				"Deleting version 3/3",
				"3 succeeded",
			},
		},
		{
			name: "dry run does not delete",
			params: BulkDeleteParams{
				Owner:     "testowner",
				OwnerType: "org",
				ImageName: "testimage",
				Versions: []gh.PackageVersionInfo{
					{ID: 200, Tags: []string{"test"}, CreatedAt: "2025-01-01"},
				},
				Force:  false,
				DryRun: true,
			},
			wantDeleted: []int64{},
			wantErr:     false,
			wantOutput:  []string{"DRY RUN: No changes made"},
			wantNotOutput: []string{
				"Deleting version",
				"succeeded",
			},
		},
		{
			name: "cancelled by user",
			params: BulkDeleteParams{
				Owner:     "testowner",
				OwnerType: "user",
				ImageName: "testimage",
				Versions: []gh.PackageVersionInfo{
					{ID: 300, Tags: []string{}, CreatedAt: "2025-01-01"},
				},
				Force:  false,
				DryRun: false,
			},
			confirmResponse: false,
			wantDeleted:     []int64{},
			wantErr:         false,
			wantOutput:      []string{"Deletion cancelled"},
		},
		{
			name: "confirmed bulk deletion",
			params: BulkDeleteParams{
				Owner:     "testowner",
				OwnerType: "user",
				ImageName: "testimage",
				Versions: []gh.PackageVersionInfo{
					{ID: 400, Tags: []string{}, CreatedAt: "2025-01-01"},
					{ID: 401, Tags: []string{}, CreatedAt: "2025-01-02"},
				},
				Force:  false,
				DryRun: false,
			},
			confirmResponse: true,
			wantDeleted:     []int64{400, 401},
			wantErr:         false,
			wantOutput:      []string{"2 succeeded"},
		},
		{
			name: "partial failure continues",
			params: BulkDeleteParams{
				Owner:     "testowner",
				OwnerType: "user",
				ImageName: "testimage",
				Versions: []gh.PackageVersionInfo{
					{ID: 500, Tags: []string{}, CreatedAt: "2025-01-01"},
					{ID: 501, Tags: []string{}, CreatedAt: "2025-01-02"},
					{ID: 502, Tags: []string{}, CreatedAt: "2025-01-03"},
				},
				Force:  true,
				DryRun: false,
			},
			deleteErrors: map[int64]error{501: fmt.Errorf("permission denied")},
			wantDeleted:  []int64{500, 502}, // 501 fails but others succeed
			wantErr:      true,
			wantOutput: []string{
				"2 succeeded",
				"1 failed",
				"Failed: permission denied",
			},
		},
		{
			name: "all failures",
			params: BulkDeleteParams{
				Owner:     "testowner",
				OwnerType: "user",
				ImageName: "testimage",
				Versions: []gh.PackageVersionInfo{
					{ID: 600, Tags: []string{}, CreatedAt: "2025-01-01"},
					{ID: 601, Tags: []string{}, CreatedAt: "2025-01-02"},
				},
				Force:  true,
				DryRun: false,
			},
			deleteErrors: map[int64]error{
				600: fmt.Errorf("error 1"),
				601: fmt.Errorf("error 2"),
			},
			wantDeleted: []int64{},
			wantErr:     true,
			wantOutput: []string{
				"0 succeeded",
				"2 failed",
			},
		},
		{
			name: "truncates display at 10 versions",
			params: BulkDeleteParams{
				Owner:     "testowner",
				OwnerType: "user",
				ImageName: "testimage",
				Versions: []gh.PackageVersionInfo{
					{ID: 1, CreatedAt: "2025-01-01"},
					{ID: 2, CreatedAt: "2025-01-01"},
					{ID: 3, CreatedAt: "2025-01-01"},
					{ID: 4, CreatedAt: "2025-01-01"},
					{ID: 5, CreatedAt: "2025-01-01"},
					{ID: 6, CreatedAt: "2025-01-01"},
					{ID: 7, CreatedAt: "2025-01-01"},
					{ID: 8, CreatedAt: "2025-01-01"},
					{ID: 9, CreatedAt: "2025-01-01"},
					{ID: 10, CreatedAt: "2025-01-01"},
					{ID: 11, CreatedAt: "2025-01-01"},
					{ID: 12, CreatedAt: "2025-01-01"},
				},
				Force:  true,
				DryRun: false,
			},
			wantDeleted: []int64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12},
			wantErr:     false,
			wantOutput: []string{
				"... and 2 more",
				"12 succeeded",
			},
		},
		{
			name: "confirmation error",
			params: BulkDeleteParams{
				Owner:     "testowner",
				OwnerType: "user",
				ImageName: "testimage",
				Versions: []gh.PackageVersionInfo{
					{ID: 700, Tags: []string{}, CreatedAt: "2025-01-01"},
				},
				Force:  false,
				DryRun: false,
			},
			confirmErr:  fmt.Errorf("input error"),
			wantDeleted: []int64{},
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := newMockPackageDeleter()
			if tt.deleteErrors != nil {
				mock.deleteErrors = tt.deleteErrors
			}

			var buf strings.Builder
			ctx := context.Background()

			confirmFn := func(count int) (bool, error) {
				return tt.confirmResponse, tt.confirmErr
			}

			err := ExecuteBulkDelete(ctx, mock, tt.params, &buf, confirmFn)

			// Check error
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			// Check deleted versions
			assert.Len(t, mock.deletedVersions, len(tt.wantDeleted), "deleted versions count mismatch")

			// Check output
			output := buf.String()
			for _, want := range tt.wantOutput {
				assert.Contains(t, output, want, "output missing expected string")
			}
			for _, notWant := range tt.wantNotOutput {
				assert.NotContains(t, output, notWant, "output should NOT contain")
			}
		})
	}
}

// =============================================================================
// Tests for helper functions
// =============================================================================

func TestFormatVersionType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		types []string
		want  string
	}{
		{
			name:  "empty types",
			types: []string{},
			want:  "unknown",
		},
		{
			name:  "nil types",
			types: nil,
			want:  "unknown",
		},
		{
			name:  "single type",
			types: []string{"index"},
			want:  "index",
		},
		{
			name:  "signature type",
			types: []string{"signature"},
			want:  "signature",
		},
		{
			name:  "platform type",
			types: []string{"linux/amd64"},
			want:  "linux/amd64",
		},
		{
			name:  "multiple types returns first",
			types: []string{"sbom", "provenance"},
			want:  "sbom",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatVersionType(tt.types)
			assert.Equal(t, tt.want, got, "formatVersionType(%v)", tt.types)
		})
	}
}
