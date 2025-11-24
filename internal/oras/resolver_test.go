package oras

import (
	"context"
	"strings"
	"testing"
)

func TestResolveTag(t *testing.T) {
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
				if err == nil {
					t.Error("Expected error but got none")
				} else if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing '%s', got '%s'", tt.errorMsg, err.Error())
				}
				if digest != "" {
					t.Errorf("Expected empty digest on error, got '%s'", digest)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				// Digest should be in format sha256:...
				if !strings.HasPrefix(digest, "sha256:") {
					t.Errorf("Expected digest to start with 'sha256:', got '%s'", digest)
				}
				if len(digest) != 71 { // "sha256:" (7) + 64 hex chars
					t.Errorf("Expected digest length 71, got %d", len(digest))
				}
			}
		})
	}
}

func TestValidateDigestFormat(t *testing.T) {
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
			valid := validateDigestFormat(tt.digest)
			if valid != tt.wantValid {
				t.Errorf("validateDigestFormat(%s) = %v, want %v", tt.digest, valid, tt.wantValid)
			}
		})
	}
}

func TestParseImageReference(t *testing.T) {
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
			registry, path, err := parseImageReference(tt.image)

			if tt.wantError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if registry != tt.wantRegistry {
					t.Errorf("Expected registry '%s', got '%s'", tt.wantRegistry, registry)
				}
				if path != tt.wantPath {
					t.Errorf("Expected path '%s', got '%s'", tt.wantPath, path)
				}
			}
		})
	}
}

func TestParseDigest(t *testing.T) {
	tests := []struct {
		name      string
		digest    string
		wantError bool
		errorMsg  string
	}{
		{
			name:      "valid digest",
			digest:    "sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
			wantError: false,
		},
		{
			name:      "invalid format - empty",
			digest:    "",
			wantError: true,
			errorMsg:  "invalid digest format",
		},
		{
			name:      "invalid format - no prefix",
			digest:    "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
			wantError: true,
			errorMsg:  "invalid digest format",
		},
		{
			name:      "invalid format - too short",
			digest:    "sha256:1234",
			wantError: true,
			errorMsg:  "invalid digest format",
		},
		{
			name:      "invalid format - invalid characters",
			digest:    "sha256:gggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggg",
			wantError: true,
			errorMsg:  "invalid digest format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			desc, err := parseDigest(tt.digest)

			if tt.wantError {
				if err == nil {
					t.Error("Expected error but got none")
				} else if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if desc.Digest.String() != tt.digest {
					t.Errorf("Expected digest '%s', got '%s'", tt.digest, desc.Digest.String())
				}
			}
		})
	}
}

func TestDiscoverReferrers(t *testing.T) {
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
			image:     "ghcr.io/owner/repo",
			digest:    "",
			wantError: true,
			errorMsg:  "digest cannot be empty",
		},
		{
			name:      "invalid digest format",
			image:     "ghcr.io/owner/repo",
			digest:    "invalid",
			wantError: true,
			errorMsg:  "invalid digest format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			referrers, err := DiscoverReferrers(ctx, tt.image, tt.digest)

			if tt.wantError {
				if err == nil {
					t.Error("Expected error but got none")
				} else if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing '%s', got '%s'", tt.errorMsg, err.Error())
				}
				if referrers != nil {
					t.Errorf("Expected nil referrers on error, got %v", referrers)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if referrers == nil {
					t.Error("Expected non-nil referrers slice")
				}
			}
		})
	}
}

func TestDetermineArtifactType(t *testing.T) {
	tests := []struct {
		name         string
		artifactType string
		want         string
	}{
		{
			name:         "SPDX SBOM",
			artifactType: "application/spdx+json",
			want:         "sbom",
		},
		{
			name:         "CycloneDX SBOM",
			artifactType: "application/vnd.cyclonedx+json",
			want:         "sbom",
		},
		{
			name:         "SLSA provenance",
			artifactType: "application/vnd.in-toto+json",
			want:         "provenance",
		},
		{
			name:         "attestation",
			artifactType: "application/vnd.dev.cosign.simplesigning.v1+json",
			want:         "attestation",
		},
		{
			name:         "unknown type",
			artifactType: "application/octet-stream",
			want:         "unknown",
		},
		{
			name:         "empty type",
			artifactType: "",
			want:         "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := determineArtifactType(tt.artifactType)
			if result != tt.want {
				t.Errorf("determineArtifactType(%s) = %s, want %s", tt.artifactType, result, tt.want)
			}
		})
	}
}

func TestGetPlatformManifests(t *testing.T) {
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
			platforms, err := GetPlatformManifests(ctx, tt.image, tt.digest)

			if tt.wantError {
				if err == nil {
					t.Error("Expected error but got none")
				} else if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing '%s', got '%s'", tt.errorMsg, err.Error())
				}
				if platforms != nil {
					t.Errorf("Expected nil platforms on error, got %v", platforms)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				// For success case, platforms can be empty list (single-arch) or populated (multi-arch)
			}
		})
	}
}

func TestFetchArtifactContent(t *testing.T) {
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
				if err == nil {
					t.Error("Expected error but got none")
				} else if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing '%s', got '%s'", tt.errorMsg, err.Error())
				}
				if content != nil {
					t.Errorf("Expected nil content on error, got %v", content)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if content == nil {
					t.Error("Expected non-nil content, got nil")
				}
			}
		})
	}
}
