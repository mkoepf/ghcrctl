package cmd

import (
	"bytes"
	"testing"

	"github.com/mkoepf/ghcrctl/internal/discover"
	"github.com/mkoepf/ghcrctl/internal/filter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListImagesCmd_RequiresPackageName(t *testing.T) {
	rootCmd := NewRootCmd()
	var out bytes.Buffer
	var errOut bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&errOut)
	rootCmd.SetArgs([]string{"list", "images"})

	err := rootCmd.Execute()
	assert.Error(t, err, "expected error for missing package name")
}

func TestListImagesCmd_HasFlags(t *testing.T) {
	rootCmd := NewRootCmd()
	imagesCmd, _, err := rootCmd.Find([]string{"list", "images"})
	require.NoError(t, err, "Failed to find list images command")

	// Check --json flag exists
	jsonFlag := imagesCmd.Flags().Lookup("json")
	assert.NotNil(t, jsonFlag, "expected --json flag")

	// Check --flat flag exists (replaced --tree, default is now tree)
	flatFlag := imagesCmd.Flags().Lookup("flat")
	assert.NotNil(t, flatFlag, "expected --flat flag")

	// Check -o flag exists
	outputFlag := imagesCmd.Flags().Lookup("output")
	assert.NotNil(t, outputFlag, "expected -o/--output flag")
}

func TestListImagesCmd_InvalidOutputFormat(t *testing.T) {
	rootCmd := NewRootCmd()
	var out bytes.Buffer
	var errOut bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&errOut)
	rootCmd.SetArgs([]string{"list", "images", "owner/test-package", "-o", "invalid"})

	err := rootCmd.Execute()
	assert.Error(t, err, "expected error for invalid output format")
}

func TestListImagesCmd_HasVersionAndDigestFlags(t *testing.T) {
	rootCmd := NewRootCmd()
	imagesCmd, _, err := rootCmd.Find([]string{"list", "images"})
	require.NoError(t, err, "Failed to find list images command")

	// Check --version flag exists
	versionFlag := imagesCmd.Flags().Lookup("version")
	assert.NotNil(t, versionFlag, "expected --version flag")

	// Check --digest flag exists
	digestFlag := imagesCmd.Flags().Lookup("digest")
	assert.NotNil(t, digestFlag, "expected --digest flag")
}

func TestListImagesCmd_MutuallyExclusiveFlags(t *testing.T) {
	rootCmd := NewRootCmd()
	var out bytes.Buffer
	var errOut bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&errOut)
	rootCmd.SetArgs([]string{"list", "images", "owner/test-package", "--version", "123", "--digest", "sha256:abc"})

	err := rootCmd.Execute()
	assert.Error(t, err, "expected error when both --version and --digest are specified")
}

func TestListImagesCmd_HasTimeFlags(t *testing.T) {
	t.Parallel()
	rootCmd := NewRootCmd()
	imagesCmd, _, err := rootCmd.Find([]string{"list", "images"})
	require.NoError(t, err, "Failed to find list images command")

	// Check --older-than flag exists
	olderThanFlag := imagesCmd.Flags().Lookup("older-than")
	assert.NotNil(t, olderThanFlag, "expected --older-than flag")

	// Check --newer-than flag exists
	newerThanFlag := imagesCmd.Flags().Lookup("newer-than")
	assert.NotNil(t, newerThanFlag, "expected --newer-than flag")
}

func TestVersionMatchesTime(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		createdAt string
		olderThan string
		newerThan string
		want      bool
	}{
		{
			name:      "matches older-than date",
			createdAt: "2025-01-01",
			olderThan: "2025-06-01",
			want:      true,
		},
		{
			name:      "does not match older-than date",
			createdAt: "2025-07-01",
			olderThan: "2025-06-01",
			want:      false,
		},
		{
			name:      "matches newer-than date",
			createdAt: "2025-07-01",
			newerThan: "2025-06-01",
			want:      true,
		},
		{
			name:      "does not match newer-than date",
			createdAt: "2025-01-01",
			newerThan: "2025-06-01",
			want:      false,
		},
		{
			name:      "no filter always matches",
			createdAt: "2025-01-01",
			want:      true,
		},
		{
			name:      "invalid date returns false",
			createdAt: "invalid-date",
			olderThan: "2025-06-01",
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tf := &filter.VersionFilter{}
			if tt.olderThan != "" {
				parsed, err := filter.ParseDate(tt.olderThan)
				require.NoError(t, err)
				tf.OlderThan = parsed
			}
			if tt.newerThan != "" {
				parsed, err := filter.ParseDate(tt.newerThan)
				require.NoError(t, err)
				tf.NewerThan = parsed
			}

			got := versionMatchesTime(tt.createdAt, tf)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFilterImagesByTime(t *testing.T) {
	t.Parallel()

	// Set up test data
	allVersions := map[string]discover.VersionInfo{
		"sha256:parent1": {
			Digest:       "sha256:parent1",
			CreatedAt:    "2025-01-15",
			OutgoingRefs: []string{"sha256:child1"},
		},
		"sha256:child1": {
			Digest:    "sha256:child1",
			CreatedAt: "2025-01-15",
		},
		"sha256:parent2": {
			Digest:       "sha256:parent2",
			CreatedAt:    "2025-07-15",
			OutgoingRefs: []string{"sha256:child2"},
		},
		"sha256:child2": {
			Digest:    "sha256:child2",
			CreatedAt: "2025-07-15",
		},
	}

	images := []discover.VersionInfo{
		allVersions["sha256:parent1"],
		allVersions["sha256:parent2"],
	}

	t.Run("filter older-than includes old image", func(t *testing.T) {
		olderThan, err := filter.ParseDate("2025-06-01")
		require.NoError(t, err)
		tf := &filter.VersionFilter{OlderThan: olderThan}

		result := filterImagesByTime(images, allVersions, tf)
		assert.Len(t, result, 1)
		assert.Equal(t, "sha256:parent1", result[0].Digest)
	})

	t.Run("filter newer-than includes new image", func(t *testing.T) {
		newerThan, err := filter.ParseDate("2025-06-01")
		require.NoError(t, err)
		tf := &filter.VersionFilter{NewerThan: newerThan}

		result := filterImagesByTime(images, allVersions, tf)
		assert.Len(t, result, 1)
		assert.Equal(t, "sha256:parent2", result[0].Digest)
	})

	t.Run("no filter returns all images", func(t *testing.T) {
		tf := &filter.VersionFilter{}
		result := filterImagesByTime(images, allVersions, tf)
		assert.Len(t, result, 2)
	})
}
