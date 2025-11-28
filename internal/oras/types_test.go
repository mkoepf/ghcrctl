package oras

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestArtifactTypeDisplayType(t *testing.T) {
	tests := []struct {
		name     string
		artifact ArtifactType
		want     string
	}{
		{
			name: "index type",
			artifact: ArtifactType{
				ManifestType: "index",
				Role:         "index",
				Platform:     "",
			},
			want: "index",
		},
		{
			name: "platform manifest",
			artifact: ArtifactType{
				ManifestType: "manifest",
				Role:         "platform",
				Platform:     "linux/amd64",
			},
			want: "linux/amd64",
		},
		{
			name: "platform manifest with variant",
			artifact: ArtifactType{
				ManifestType: "manifest",
				Role:         "platform",
				Platform:     "linux/arm/v7",
			},
			want: "linux/arm/v7",
		},
		{
			name: "sbom attestation",
			artifact: ArtifactType{
				ManifestType: "manifest",
				Role:         "sbom",
				Platform:     "",
			},
			want: "sbom",
		},
		{
			name: "provenance attestation",
			artifact: ArtifactType{
				ManifestType: "manifest",
				Role:         "provenance",
				Platform:     "",
			},
			want: "provenance",
		},
		{
			name: "generic attestation",
			artifact: ArtifactType{
				ManifestType: "manifest",
				Role:         "attestation",
				Platform:     "",
			},
			want: "attestation",
		},
		{
			name: "signature",
			artifact: ArtifactType{
				ManifestType: "manifest",
				Role:         "signature",
				Platform:     "",
			},
			want: "signature",
		},
		{
			name: "vuln-scan",
			artifact: ArtifactType{
				ManifestType: "manifest",
				Role:         "vuln-scan",
				Platform:     "",
			},
			want: "vuln-scan",
		},
		{
			name: "artifact",
			artifact: ArtifactType{
				ManifestType: "manifest",
				Role:         "artifact",
				Platform:     "",
			},
			want: "artifact",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.artifact.DisplayType()
			if got != tt.want {
				t.Errorf("ArtifactType.DisplayType() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestArtifactTypeIsAttestation(t *testing.T) {
	tests := []struct {
		name     string
		artifact ArtifactType
		want     bool
	}{
		{
			name:     "sbom is attestation",
			artifact: ArtifactType{Role: "sbom"},
			want:     true,
		},
		{
			name:     "provenance is attestation",
			artifact: ArtifactType{Role: "provenance"},
			want:     true,
		},
		{
			name:     "attestation is attestation",
			artifact: ArtifactType{Role: "attestation"},
			want:     true,
		},
		{
			name:     "vuln-scan is attestation",
			artifact: ArtifactType{Role: "vuln-scan"},
			want:     true,
		},
		{
			name:     "platform is not attestation",
			artifact: ArtifactType{Role: "platform"},
			want:     false,
		},
		{
			name:     "index is not attestation",
			artifact: ArtifactType{Role: "index"},
			want:     false,
		},
		{
			name:     "signature is not attestation",
			artifact: ArtifactType{Role: "signature"},
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.artifact.IsAttestation()
			if got != tt.want {
				t.Errorf("ArtifactType.IsAttestation() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestArtifactTypeIsPlatform(t *testing.T) {
	tests := []struct {
		name     string
		artifact ArtifactType
		want     bool
	}{
		{
			name:     "platform role is platform",
			artifact: ArtifactType{Role: "platform", Platform: "linux/amd64"},
			want:     true,
		},
		{
			name:     "sbom is not platform",
			artifact: ArtifactType{Role: "sbom"},
			want:     false,
		},
		{
			name:     "index is not platform",
			artifact: ArtifactType{Role: "index"},
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.artifact.IsPlatform()
			if got != tt.want {
				t.Errorf("ArtifactType.IsPlatform() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResolveTypeInputValidation(t *testing.T) {
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
			_, err := ResolveType(ctx, tt.image, tt.digest)

			if tt.wantError {
				if err == nil {
					t.Error("Expected error but got none")
				} else if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing %q, got %q", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

// Integration tests for ResolveType - require GITHUB_TOKEN
func TestResolveTypeIntegration(t *testing.T) {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		t.Skip("Skipping integration test - GITHUB_TOKEN not set")
	}

	ctx := context.Background()
	testImage := "ghcr.io/mkoepf/ghcrctl-test-with-sbom"

	// First resolve the tag to get the index digest
	indexDigest, err := ResolveTag(ctx, testImage, "latest")
	if err != nil {
		t.Fatalf("Failed to resolve tag: %v", err)
	}

	t.Run("resolves index type", func(t *testing.T) {
		artType, err := ResolveType(ctx, testImage, indexDigest)
		if err != nil {
			t.Fatalf("Failed to resolve type: %v", err)
		}

		if artType.ManifestType != "index" {
			t.Errorf("Expected ManifestType 'index', got %q", artType.ManifestType)
		}
		if artType.Role != "index" {
			t.Errorf("Expected Role 'index', got %q", artType.Role)
		}
		if artType.DisplayType() != "index" {
			t.Errorf("Expected DisplayType() 'index', got %q", artType.DisplayType())
		}
	})

	// Get platform manifests to test platform type resolution
	platforms, err := GetPlatformManifests(ctx, testImage, indexDigest)
	if err != nil {
		t.Fatalf("Failed to get platform manifests: %v", err)
	}

	if len(platforms) > 0 {
		t.Run("resolves platform type", func(t *testing.T) {
			platformDigest := platforms[0].Digest
			artType, err := ResolveType(ctx, testImage, platformDigest)
			if err != nil {
				t.Fatalf("Failed to resolve type: %v", err)
			}

			if artType.ManifestType != "manifest" {
				t.Errorf("Expected ManifestType 'manifest', got %q", artType.ManifestType)
			}
			if artType.Role != "platform" {
				t.Errorf("Expected Role 'platform', got %q", artType.Role)
			}
			if artType.Platform == "" {
				t.Error("Expected non-empty Platform for platform manifest")
			}
			// Platform should contain a slash (os/arch)
			if !strings.Contains(artType.Platform, "/") {
				t.Errorf("Expected Platform to contain '/', got %q", artType.Platform)
			}
		})
	}

	// Get attestations to test attestation type resolution
	referrers, err := DiscoverReferrers(ctx, testImage, indexDigest)
	if err != nil {
		t.Fatalf("Failed to discover referrers: %v", err)
	}

	for _, ref := range referrers {
		if ref.ArtifactType == "sbom" {
			t.Run("resolves sbom attestation type", func(t *testing.T) {
				artType, err := ResolveType(ctx, testImage, ref.Digest)
				if err != nil {
					t.Fatalf("Failed to resolve type: %v", err)
				}

				if artType.ManifestType != "manifest" {
					t.Errorf("Expected ManifestType 'manifest', got %q", artType.ManifestType)
				}
				// Role should be sbom or attestation
				if artType.Role != "sbom" && artType.Role != "attestation" {
					t.Errorf("Expected Role 'sbom' or 'attestation', got %q", artType.Role)
				}
			})
			break
		}
	}
}
