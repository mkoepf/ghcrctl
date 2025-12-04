package discover

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveTag(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		image     string
		tag       string
		wantError bool
		errorMsg  string
	}{
		{
			name:      "empty image",
			image:     "",
			tag:       "latest",
			wantError: true,
			errorMsg:  "image cannot be empty",
		},
		{
			name:      "empty tag",
			image:     "ghcr.io/owner/image",
			tag:       "",
			wantError: true,
			errorMsg:  "tag cannot be empty",
		},
		{
			name:      "invalid image format - no registry",
			image:     "owner/image",
			tag:       "latest",
			wantError: true,
			errorMsg:  "invalid image format",
		},
		{
			name:      "invalid image format - no path",
			image:     "ghcr.io",
			tag:       "latest",
			wantError: true,
			errorMsg:  "invalid image format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			digest, err := ResolveTag(ctx, tt.image, tt.tag)

			if tt.wantError {
				require.Error(t, err)
				assert.ErrorContains(t, err, tt.errorMsg)
				assert.Empty(t, digest, "Expected empty digest on error")
			} else {
				require.NoError(t, err)
				assert.True(t, len(digest) > 0 && digest[:7] == "sha256:", "Expected digest to start with 'sha256:'")
				assert.Len(t, digest, 71, "Expected digest length 71")
			}
		})
	}
}

func TestValidateDigestFormat(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		digest    string
		wantValid bool
	}{
		{
			name:      "valid sha256 digest",
			digest:    "sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
			wantValid: true,
		},
		{
			name:      "empty digest",
			digest:    "",
			wantValid: false,
		},
		{
			name:      "missing sha256 prefix",
			digest:    "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
			wantValid: false,
		},
		{
			name:      "wrong prefix",
			digest:    "sha512:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
			wantValid: false,
		},
		{
			name:      "too short",
			digest:    "sha256:1234",
			wantValid: false,
		},
		{
			name:      "invalid characters",
			digest:    "sha256:gggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggg",
			wantValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid := ValidateDigestFormat(tt.digest)
			assert.Equal(t, tt.wantValid, valid)
		})
	}
}

func TestParseImageReference(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		image        string
		wantRegistry string
		wantPath     string
		wantError    bool
	}{
		{
			name:         "valid ghcr.io image",
			image:        "ghcr.io/owner/repo",
			wantRegistry: "ghcr.io",
			wantPath:     "owner/repo",
			wantError:    false,
		},
		{
			name:         "valid ghcr.io with nested path",
			image:        "ghcr.io/owner/org/repo",
			wantRegistry: "ghcr.io",
			wantPath:     "owner/org/repo",
			wantError:    false,
		},
		{
			name:         "missing registry",
			image:        "owner/repo",
			wantRegistry: "",
			wantPath:     "",
			wantError:    true,
		},
		{
			name:         "only registry",
			image:        "ghcr.io",
			wantRegistry: "",
			wantPath:     "",
			wantError:    true,
		},
		{
			name:         "empty image",
			image:        "",
			wantRegistry: "",
			wantPath:     "",
			wantError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry, path, err := ParseImageReference(tt.image)

			if tt.wantError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantRegistry, registry)
				assert.Equal(t, tt.wantPath, path)
			}
		})
	}
}

func TestFetchArtifactContent(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		image     string
		digest    string
		wantError bool
		errorMsg  string
	}{
		{
			name:      "empty image",
			image:     "",
			digest:    "sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
			wantError: true,
			errorMsg:  "image cannot be empty",
		},
		{
			name:      "empty digest",
			image:     "ghcr.io/owner/image",
			digest:    "",
			wantError: true,
			errorMsg:  "digest cannot be empty",
		},
		{
			name:      "invalid digest format",
			image:     "ghcr.io/owner/image",
			digest:    "invalid-digest",
			wantError: true,
			errorMsg:  "invalid digest format",
		},
		{
			name:      "invalid image format",
			image:     "owner/image",
			digest:    "sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
			wantError: true,
			errorMsg:  "invalid image format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			content, err := FetchArtifactContent(ctx, tt.image, tt.digest)

			if tt.wantError {
				require.Error(t, err)
				assert.ErrorContains(t, err, tt.errorMsg)
				assert.Nil(t, content, "Expected nil content on error")
			} else {
				require.NoError(t, err)
				assert.NotNil(t, content)
			}
		})
	}
}

func TestFetchImageConfig(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		image     string
		digest    string
		wantError bool
		errorMsg  string
	}{
		{
			name:      "empty image",
			image:     "",
			digest:    "sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
			wantError: true,
			errorMsg:  "image cannot be empty",
		},
		{
			name:      "empty digest",
			image:     "ghcr.io/owner/image",
			digest:    "",
			wantError: true,
			errorMsg:  "digest cannot be empty",
		},
		{
			name:      "invalid digest format",
			image:     "ghcr.io/owner/image",
			digest:    "invalid-digest",
			wantError: true,
			errorMsg:  "invalid digest format",
		},
		{
			name:      "invalid image format",
			image:     "owner/image",
			digest:    "sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
			wantError: true,
			errorMsg:  "invalid image format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			config, err := FetchImageConfig(ctx, tt.image, tt.digest)

			if tt.wantError {
				require.Error(t, err)
				assert.ErrorContains(t, err, tt.errorMsg)
				assert.Nil(t, config, "Expected nil config on error")
			} else {
				require.NoError(t, err)
				assert.NotNil(t, config)
			}
		})
	}
}
