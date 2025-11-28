package oras

import (
	"context"
	"os"
	"testing"
)

func TestDiscoverChildrenInputValidation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		image        string
		parentDigest string
		wantError    bool
		errorMsg     string
	}{
		{
			name:         "empty image",
			image:        "",
			parentDigest: "sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
			wantError:    true,
			errorMsg:     "image cannot be empty",
		},
		{
			name:         "empty digest",
			image:        "ghcr.io/owner/image",
			parentDigest: "",
			wantError:    true,
			errorMsg:     "digest cannot be empty",
		},
		{
			name:         "invalid digest format",
			image:        "ghcr.io/owner/image",
			parentDigest: "invalid-digest",
			wantError:    true,
			errorMsg:     "invalid digest format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			_, err := DiscoverChildren(ctx, tt.image, tt.parentDigest, nil)

			if tt.wantError {
				if err == nil {
					t.Error("Expected error but got none")
				} else if err.Error() != tt.errorMsg && !contains(err.Error(), tt.errorMsg) {
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

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsAt(s, substr, 0))
}

func containsAt(s, substr string, start int) bool {
	for i := start; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestDigestToTagPrefix(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		digest string
		want   string
	}{
		{
			name:   "standard sha256 digest",
			digest: "sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
			want:   "sha256-1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
		},
		{
			name:   "another digest",
			digest: "sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
			want:   "sha256-abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := digestToTagPrefix(tt.digest)
			if got != tt.want {
				t.Errorf("digestToTagPrefix(%q) = %q, want %q", tt.digest, got, tt.want)
			}
		})
	}
}

func TestFindCosignTags(t *testing.T) {
	t.Parallel()
	parentDigest := "sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
	prefix := "sha256-1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"

	tests := []struct {
		name      string
		allTags   []string
		wantSig   string
		wantAtt   string
		wantSigOk bool
		wantAttOk bool
	}{
		{
			name:      "no cosign tags",
			allTags:   []string{"latest", "v1.0.0"},
			wantSigOk: false,
			wantAttOk: false,
		},
		{
			name:      "has signature tag",
			allTags:   []string{"latest", prefix + ".sig"},
			wantSig:   prefix + ".sig",
			wantSigOk: true,
			wantAttOk: false,
		},
		{
			name:      "has attestation tag",
			allTags:   []string{"latest", prefix + ".att"},
			wantAttOk: true,
			wantAtt:   prefix + ".att",
			wantSigOk: false,
		},
		{
			name:      "has both sig and att tags",
			allTags:   []string{"latest", prefix + ".sig", prefix + ".att"},
			wantSig:   prefix + ".sig",
			wantAtt:   prefix + ".att",
			wantSigOk: true,
			wantAttOk: true,
		},
		{
			name:      "has unrelated cosign tags",
			allTags:   []string{"sha256-otherdigest.sig", "sha256-otherdigest.att"},
			wantSigOk: false,
			wantAttOk: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sigTag, attTag := findCosignTags(parentDigest, tt.allTags)

			if tt.wantSigOk {
				if sigTag != tt.wantSig {
					t.Errorf("Expected sigTag %q, got %q", tt.wantSig, sigTag)
				}
			} else {
				if sigTag != "" {
					t.Errorf("Expected no sigTag, got %q", sigTag)
				}
			}

			if tt.wantAttOk {
				if attTag != tt.wantAtt {
					t.Errorf("Expected attTag %q, got %q", tt.wantAtt, attTag)
				}
			} else {
				if attTag != "" {
					t.Errorf("Expected no attTag, got %q", attTag)
				}
			}
		})
	}
}

func TestExtractParentDigestFromCosignTag(t *testing.T) {
	tests := []struct {
		name       string
		tag        string
		wantDigest string
		wantOk     bool
	}{
		{
			name:       "signature tag",
			tag:        "sha256-1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef.sig",
			wantDigest: "sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
			wantOk:     true,
		},
		{
			name:       "attestation tag",
			tag:        "sha256-abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890.att",
			wantDigest: "sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
			wantOk:     true,
		},
		{
			name:       "regular tag",
			tag:        "latest",
			wantDigest: "",
			wantOk:     false,
		},
		{
			name:       "semver tag",
			tag:        "v1.0.0",
			wantDigest: "",
			wantOk:     false,
		},
		{
			name:       "tag with sha256 but not cosign format",
			tag:        "sha256-something-else",
			wantDigest: "",
			wantOk:     false,
		},
		{
			name:       "empty tag",
			tag:        "",
			wantDigest: "",
			wantOk:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			digest, ok := ExtractParentDigestFromCosignTag(tt.tag)
			if ok != tt.wantOk {
				t.Errorf("ExtractParentDigestFromCosignTag(%q) ok = %v, want %v", tt.tag, ok, tt.wantOk)
			}
			if digest != tt.wantDigest {
				t.Errorf("ExtractParentDigestFromCosignTag(%q) = %q, want %q", tt.tag, digest, tt.wantDigest)
			}
		})
	}
}

// TestExtractSubjectDigest verifies extraction of parent digest from in-toto subject
func TestExtractSubjectDigest(t *testing.T) {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		t.Skip("Skipping integration test - GITHUB_TOKEN not set")
	}

	ctx := context.Background()
	testImage := "ghcr.io/mkoepf/ghcrctl-test-cosign-vuln"

	// First, get the parent image digest
	parentDigest, err := ResolveTag(ctx, testImage, "latest")
	if err != nil {
		t.Fatalf("Failed to resolve parent tag: %v", err)
	}

	// Construct the orphan attestation digest - we need to find an untagged attestation
	// The orphan is the one that only has vuln-scan (not both vex and vuln-scan)
	// For this test, we'll use the tagged attestation first to verify the function works
	attTag := digestToTagPrefix(parentDigest) + ".att"
	attDigest, err := ResolveTag(ctx, testImage, attTag)
	if err != nil {
		t.Fatalf("Failed to resolve attestation tag: %v", err)
	}

	// Extract subject digest from the attestation
	subjectDigest, err := ExtractSubjectDigest(ctx, testImage, attDigest)
	if err != nil {
		t.Fatalf("ExtractSubjectDigest() error = %v", err)
	}

	// The subject should match the parent image digest
	if subjectDigest != parentDigest {
		t.Errorf("Expected subject digest %q, got %q", parentDigest, subjectDigest)
	}

	t.Logf("✓ Extracted subject digest: %s", subjectDigest[:20]+"...")
}

// TestDetermineAttestationRolesFromDescriptor verifies that cosign attestations have their type resolved
func TestDetermineAttestationRolesFromDescriptor(t *testing.T) {
	t.Parallel()
	// Unit test for determineAttestationRolesFromDescriptor
	// The function should return specific roles like "sbom", "provenance" etc., not "attestation"
	tests := []struct {
		name          string
		predicateType string
		wantRole      string
	}{
		{
			name:          "SPDX SBOM predicate",
			predicateType: "https://spdx.dev/Document",
			wantRole:      "sbom",
		},
		{
			name:          "CycloneDX SBOM predicate",
			predicateType: "https://cyclonedx.org/bom",
			wantRole:      "sbom",
		},
		{
			name:          "SLSA provenance predicate",
			predicateType: "https://slsa.dev/provenance/v0.2",
			wantRole:      "provenance",
		},
		{
			name:          "SLSA provenance v1 predicate",
			predicateType: "https://slsa.dev/provenance/v1",
			wantRole:      "provenance",
		},
		{
			name:          "Vulnerability scan predicate",
			predicateType: "https://example.com/vulnerability-scan",
			wantRole:      "vuln-scan",
		},
		{
			name:          "VEX predicate",
			predicateType: "https://openvex.dev/ns/v0.2.0",
			wantRole:      "vex",
		},
		{
			name:          "Unknown predicate type",
			predicateType: "https://example.com/custom",
			wantRole:      "attestation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := predicateTypeToRole(tt.predicateType)
			if got != tt.wantRole {
				t.Errorf("predicateTypeToRole(%q) = %q, want %q", tt.predicateType, got, tt.wantRole)
			}
		})
	}
}

// Integration test for DiscoverChildren - requires GITHUB_TOKEN
func TestDiscoverChildrenIntegration(t *testing.T) {
	t.Parallel()
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

	t.Run("discovers platform manifests and attestations", func(t *testing.T) {
		children, err := DiscoverChildren(ctx, testImage, indexDigest, nil)
		if err != nil {
			t.Fatalf("Failed to discover children: %v", err)
		}

		if len(children) == 0 {
			t.Error("Expected at least one child artifact")
		}

		// Check that we have both platforms and attestations
		hasPlatform := false
		hasAttestation := false

		for _, child := range children {
			if child.Type.IsPlatform() {
				hasPlatform = true
				if child.Digest == "" {
					t.Error("Platform child has empty digest")
				}
				if child.Type.Platform == "" {
					t.Error("Platform child has empty platform string")
				}
			}
			if child.Type.IsAttestation() {
				hasAttestation = true
				if child.Digest == "" {
					t.Error("Attestation child has empty digest")
				}
			}
		}

		if !hasPlatform {
			t.Error("Expected at least one platform manifest")
		}
		if !hasAttestation {
			t.Error("Expected at least one attestation")
		}
	})

	t.Run("returns empty for non-index manifest", func(t *testing.T) {
		// First get a platform digest
		children, err := DiscoverChildren(ctx, testImage, indexDigest, nil)
		if err != nil {
			t.Fatalf("Failed to discover children: %v", err)
		}

		var platformDigest string
		for _, child := range children {
			if child.Type.IsPlatform() {
				platformDigest = child.Digest
				break
			}
		}

		if platformDigest == "" {
			t.Skip("No platform manifest found to test")
		}

		// A platform manifest should have no children
		platformChildren, err := DiscoverChildren(ctx, testImage, platformDigest, nil)
		if err != nil {
			t.Fatalf("Failed to discover children for platform: %v", err)
		}

		if len(platformChildren) != 0 {
			t.Errorf("Expected no children for platform manifest, got %d", len(platformChildren))
		}
	})
}
