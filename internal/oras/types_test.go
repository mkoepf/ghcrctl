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
			name: "platform manifest",
			artifact: ArtifactType{
				Role:     "platform",
				Platform: "linux/amd64",
			},
			want: "linux/amd64",
		},
		{
			name: "platform manifest with variant",
			artifact: ArtifactType{
				Role:     "platform",
				Platform: "linux/arm/v7",
			},
			want: "linux/arm/v7",
		},
		{
			name: "sbom attestation",
			artifact: ArtifactType{
				Role: "sbom",
			},
			want: "sbom",
		},
		{
			name: "provenance attestation",
			artifact: ArtifactType{
				Role: "provenance",
			},
			want: "provenance",
		},
		{
			name: "generic attestation",
			artifact: ArtifactType{
				Role: "attestation",
			},
			want: "attestation",
		},
		{
			name: "signature",
			artifact: ArtifactType{
				Role: "signature",
			},
			want: "signature",
		},
		{
			name: "vuln-scan",
			artifact: ArtifactType{
				Role: "vuln-scan",
			},
			want: "vuln-scan",
		},
		{
			name: "vex",
			artifact: ArtifactType{
				Role: "vex",
			},
			want: "vex",
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
			name:     "vex is attestation",
			artifact: ArtifactType{Role: "vex"},
			want:     true,
		},
		{
			name:     "platform is not attestation",
			artifact: ArtifactType{Role: "platform"},
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

func TestChildArtifact(t *testing.T) {
	t.Run("platform child", func(t *testing.T) {
		child := ChildArtifact{
			Digest: "sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
			Size:   1024,
			Type:   ArtifactType{Role: "platform", Platform: "linux/amd64"},
			Tag:    "",
		}

		if child.Type.DisplayType() != "linux/amd64" {
			t.Errorf("Expected DisplayType 'linux/amd64', got %q", child.Type.DisplayType())
		}
		if !child.Type.IsPlatform() {
			t.Error("Expected IsPlatform() to be true")
		}
	})

	t.Run("cosign signature child", func(t *testing.T) {
		child := ChildArtifact{
			Digest: "sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
			Size:   512,
			Type:   ArtifactType{Role: "signature"},
			Tag:    "sha256-1234567890abcdef.sig",
		}

		if child.Type.DisplayType() != "signature" {
			t.Errorf("Expected DisplayType 'signature', got %q", child.Type.DisplayType())
		}
		if child.Type.IsAttestation() {
			t.Error("Expected IsAttestation() to be false for signature")
		}
		if child.Tag == "" {
			t.Error("Expected non-empty Tag for cosign artifact")
		}
	})

	t.Run("buildx sbom child", func(t *testing.T) {
		child := ChildArtifact{
			Digest: "sha256:fedcba0987654321fedcba0987654321fedcba0987654321fedcba0987654321",
			Size:   2048,
			Type:   ArtifactType{Role: "sbom"},
			Tag:    "", // buildx artifacts have no tag
		}

		if child.Type.DisplayType() != "sbom" {
			t.Errorf("Expected DisplayType 'sbom', got %q", child.Type.DisplayType())
		}
		if !child.Type.IsAttestation() {
			t.Error("Expected IsAttestation() to be true for sbom")
		}
	})
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

// TestResolveTypeCosignSignature verifies signature type resolution for cosign .sig artifacts
func TestResolveTypeCosignSignature(t *testing.T) {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		t.Skip("Skipping integration test - GITHUB_TOKEN not set")
	}

	ctx := context.Background()
	testImage := "ghcr.io/mkoepf/ghcrctl-test-cosign-signed"

	// Get the cosign signature digest via tag
	sigTag := "sha256-f683151837ef93048ffa68193f438b19bf5b42c183981301bbad169960bd10af.sig"
	sigDigest, err := ResolveTag(ctx, testImage, sigTag)
	if err != nil {
		t.Fatalf("Failed to resolve signature tag: %v", err)
	}

	artType, err := ResolveType(ctx, testImage, sigDigest)
	if err != nil {
		t.Fatalf("ResolveType() error = %v", err)
	}

	if artType.Role != "signature" {
		t.Errorf("Expected Role 'signature', got %q", artType.Role)
	}
}

// TestResolveTypeCosignAttestation verifies attestation type resolution for cosign .att artifacts
func TestResolveTypeCosignAttestation(t *testing.T) {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		t.Skip("Skipping integration test - GITHUB_TOKEN not set")
	}

	ctx := context.Background()
	testImage := "ghcr.io/mkoepf/ghcrctl-test-cosign-signed"

	// Get the cosign attestation digest via tag
	attTag := "sha256-f683151837ef93048ffa68193f438b19bf5b42c183981301bbad169960bd10af.att"
	attDigest, err := ResolveTag(ctx, testImage, attTag)
	if err != nil {
		t.Fatalf("Failed to resolve attestation tag: %v", err)
	}

	artType, err := ResolveType(ctx, testImage, attDigest)
	if err != nil {
		t.Fatalf("ResolveType() error = %v", err)
	}

	// Should be sbom since we attached an SPDX SBOM
	if artType.Role != "sbom" {
		t.Errorf("Expected Role 'sbom', got %q", artType.Role)
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

	t.Run("returns index type for index digest", func(t *testing.T) {
		artType, err := ResolveType(ctx, testImage, indexDigest)
		if err != nil {
			t.Fatalf("ResolveType() error = %v", err)
		}
		if artType.Role != "index" {
			t.Errorf("Expected Role 'index', got %q", artType.Role)
		}
	})

	// Get children to test platform type resolution
	children, err := DiscoverChildren(ctx, testImage, indexDigest, nil)
	if err != nil {
		t.Fatalf("Failed to discover children: %v", err)
	}

	// Find a platform manifest
	for _, child := range children {
		if child.Type.IsPlatform() {
			t.Run("resolves platform type", func(t *testing.T) {
				artType, err := ResolveType(ctx, testImage, child.Digest)
				if err != nil {
					t.Fatalf("Failed to resolve type: %v", err)
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
			break
		}
	}

	// Find an attestation
	for _, child := range children {
		if child.Type.IsAttestation() {
			t.Run("resolves attestation type", func(t *testing.T) {
				artType, err := ResolveType(ctx, testImage, child.Digest)
				if err != nil {
					t.Fatalf("Failed to resolve type: %v", err)
				}

				// Role should contain sbom, provenance, or attestation
				// (may be combined for multi-predicate attestations like "provenance, sbom")
				role := artType.Role
				hasValidRole := strings.Contains(role, "sbom") ||
					strings.Contains(role, "provenance") ||
					role == "attestation"
				if !hasValidRole {
					t.Errorf("Expected attestation Role containing sbom/provenance/attestation, got %q", role)
				}
			})
			break
		}
	}
}

// TestResolveTypeCosignMultiPredicateAttestation verifies that multi-predicate cosign attestations
// return combined roles (e.g., "vuln-scan, vex" instead of just "vuln-scan")
func TestResolveTypeCosignMultiPredicateAttestation(t *testing.T) {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		t.Skip("Skipping integration test - GITHUB_TOKEN not set")
	}

	ctx := context.Background()
	testImage := "ghcr.io/mkoepf/ghcrctl-test-cosign-vuln"

	// Resolve the image digest first
	imageDigest, err := ResolveTag(ctx, testImage, "latest")
	if err != nil {
		t.Fatalf("Failed to resolve image tag: %v", err)
	}

	// Get the cosign attestation tag
	attTag := "sha256-" + strings.TrimPrefix(imageDigest, "sha256:") + ".att"
	attDigest, err := ResolveTag(ctx, testImage, attTag)
	if err != nil {
		t.Fatalf("Failed to resolve attestation tag: %v", err)
	}

	artType, err := ResolveType(ctx, testImage, attDigest)
	if err != nil {
		t.Fatalf("ResolveType() error = %v", err)
	}

	// Should contain both vuln-scan and vex since the image has both attestations
	role := artType.Role
	if !strings.Contains(role, "vuln-scan") {
		t.Errorf("Expected Role to contain 'vuln-scan', got %q", role)
	}
	if !strings.Contains(role, "vex") {
		t.Errorf("Expected Role to contain 'vex', got %q", role)
	}

	t.Logf("✓ Multi-predicate attestation role: %s", role)
}
