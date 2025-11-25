package graph

import (
	"testing"
)

func TestNewArtifact(t *testing.T) {
	tests := []struct {
		name         string
		digest       string
		artifactType string
		wantErr      bool
	}{
		{
			name:         "valid image artifact",
			digest:       "sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
			artifactType: ArtifactTypeImage,
			wantErr:      false,
		},
		{
			name:         "valid SBOM artifact",
			digest:       "sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
			artifactType: ArtifactTypeSBOM,
			wantErr:      false,
		},
		{
			name:         "empty digest",
			digest:       "",
			artifactType: ArtifactTypeImage,
			wantErr:      true,
		},
		{
			name:         "invalid digest format",
			digest:       "invalid",
			artifactType: ArtifactTypeImage,
			wantErr:      true,
		},
		{
			name:         "empty artifact type",
			digest:       "sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
			artifactType: "",
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			artifact, err := NewArtifact(tt.digest, tt.artifactType)

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error but got none")
				}
				if artifact != nil {
					t.Error("Expected nil artifact on error")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if artifact == nil {
					t.Error("Expected non-nil artifact")
				}
				if artifact.Digest != tt.digest {
					t.Errorf("Expected digest %s, got %s", tt.digest, artifact.Digest)
				}
				if artifact.Type != tt.artifactType {
					t.Errorf("Expected type %s, got %s", tt.artifactType, artifact.Type)
				}
			}
		})
	}
}

func TestArtifactAddTag(t *testing.T) {
	artifact, err := NewArtifact("sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef", ArtifactTypeImage)
	if err != nil {
		t.Fatalf("Failed to create artifact: %v", err)
	}

	// Add first tag
	artifact.AddTag("v1.0.0")
	if len(artifact.Tags) != 1 {
		t.Errorf("Expected 1 tag, got %d", len(artifact.Tags))
	}
	if artifact.Tags[0] != "v1.0.0" {
		t.Errorf("Expected tag 'v1.0.0', got '%s'", artifact.Tags[0])
	}

	// Add second tag
	artifact.AddTag("latest")
	if len(artifact.Tags) != 2 {
		t.Errorf("Expected 2 tags, got %d", len(artifact.Tags))
	}

	// Add duplicate tag - should not add
	artifact.AddTag("v1.0.0")
	if len(artifact.Tags) != 2 {
		t.Errorf("Expected 2 tags (duplicate not added), got %d", len(artifact.Tags))
	}
}

func TestArtifactSetVersionID(t *testing.T) {
	artifact, _ := NewArtifact("sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef", ArtifactTypeImage)

	artifact.SetVersionID(12345)
	if artifact.VersionID != 12345 {
		t.Errorf("Expected version ID 12345, got %d", artifact.VersionID)
	}
}

func TestNewGraph(t *testing.T) {
	digest := "sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
	graph, err := NewGraph(digest)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if graph == nil {
		t.Fatal("Expected non-nil graph")
	}
	if graph.Root == nil {
		t.Error("Expected non-nil root artifact")
	}
	if graph.Root.Digest != digest {
		t.Errorf("Expected root digest %s, got %s", digest, graph.Root.Digest)
	}
	if graph.Root.Type != ArtifactTypeImage {
		t.Errorf("Expected root type %s, got %s", ArtifactTypeImage, graph.Root.Type)
	}
	if len(graph.Referrers) != 0 {
		t.Errorf("Expected 0 referrers, got %d", len(graph.Referrers))
	}
}

func TestGraphAddReferrer(t *testing.T) {
	graph, _ := NewGraph("sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef")

	// Add SBOM
	sbom, _ := NewArtifact("sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890", ArtifactTypeSBOM)
	graph.AddReferrer(sbom)

	if len(graph.Referrers) != 1 {
		t.Errorf("Expected 1 referrer, got %d", len(graph.Referrers))
	}

	// Add provenance
	provenance, _ := NewArtifact("sha256:fedcba0987654321fedcba0987654321fedcba0987654321fedcba0987654321", ArtifactTypeProvenance)
	graph.AddReferrer(provenance)

	if len(graph.Referrers) != 2 {
		t.Errorf("Expected 2 referrers, got %d", len(graph.Referrers))
	}
}

func TestGraphHasSBOM(t *testing.T) {
	graph, _ := NewGraph("sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef")

	if graph.HasSBOM() {
		t.Error("Expected HasSBOM to be false for empty graph")
	}

	sbom, _ := NewArtifact("sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890", ArtifactTypeSBOM)
	graph.AddReferrer(sbom)

	if !graph.HasSBOM() {
		t.Error("Expected HasSBOM to be true after adding SBOM")
	}
}

func TestGraphHasProvenance(t *testing.T) {
	graph, _ := NewGraph("sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef")

	if graph.HasProvenance() {
		t.Error("Expected HasProvenance to be false for empty graph")
	}

	provenance, _ := NewArtifact("sha256:fedcba0987654321fedcba0987654321fedcba0987654321fedcba0987654321", ArtifactTypeProvenance)
	graph.AddReferrer(provenance)

	if !graph.HasProvenance() {
		t.Error("Expected HasProvenance to be true after adding provenance")
	}
}

func TestGraphGetReferrersByType(t *testing.T) {
	graph, _ := NewGraph("sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef")

	// Add multiple referrers
	sbom, _ := NewArtifact("sha256:abcd1234567890abcdef1234567890abcdef1234567890abcdef1234567890ab", ArtifactTypeSBOM)
	provenance, _ := NewArtifact("sha256:bcde1234567890abcdef1234567890abcdef1234567890abcdef1234567890ab", ArtifactTypeProvenance)
	attestation, _ := NewArtifact("sha256:cdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890ab", ArtifactTypeAttestation)

	graph.AddReferrer(sbom)
	graph.AddReferrer(provenance)
	graph.AddReferrer(attestation)

	// Get SBOMs
	sboms := graph.GetReferrersByType(ArtifactTypeSBOM)
	if len(sboms) != 1 {
		t.Errorf("Expected 1 SBOM, got %d", len(sboms))
	}

	// Get provenances
	provenances := graph.GetReferrersByType(ArtifactTypeProvenance)
	if len(provenances) != 1 {
		t.Errorf("Expected 1 provenance, got %d", len(provenances))
	}

	// Get attestations
	attestations := graph.GetReferrersByType(ArtifactTypeAttestation)
	if len(attestations) != 1 {
		t.Errorf("Expected 1 attestation, got %d", len(attestations))
	}

	// Get non-existent type
	others := graph.GetReferrersByType("unknown")
	if len(others) != 0 {
		t.Errorf("Expected 0 unknown types, got %d", len(others))
	}
}

func TestGraphUniqueArtifactCount(t *testing.T) {
	// Create a graph
	g, err := NewGraph("sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef")
	if err != nil {
		t.Fatalf("Failed to create graph: %v", err)
	}

	// Initially should be 1 (just the root)
	if count := g.UniqueArtifactCount(); count != 1 {
		t.Errorf("Expected count 1 for root only, got %d", count)
	}

	// Add an SBOM artifact
	sbom, _ := NewArtifact("sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", ArtifactTypeSBOM)
	g.AddReferrer(sbom)

	// Should be 2 (root + 1 unique referrer)
	if count := g.UniqueArtifactCount(); count != 2 {
		t.Errorf("Expected count 2, got %d", count)
	}

	// Add a provenance artifact with the SAME digest (simulating multi-layer manifest)
	provenance, _ := NewArtifact("sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", ArtifactTypeProvenance)
	g.AddReferrer(provenance)

	// Should still be 2 (same digest, so same unique artifact)
	if count := g.UniqueArtifactCount(); count != 2 {
		t.Errorf("Expected count 2 (same digest), got %d", count)
	}

	// Add another artifact with different digest
	sbom2, _ := NewArtifact("sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", ArtifactTypeSBOM)
	g.AddReferrer(sbom2)

	// Should be 3 (root + 2 unique referrers)
	if count := g.UniqueArtifactCount(); count != 3 {
		t.Errorf("Expected count 3, got %d", count)
	}
}

func TestNewPlatform(t *testing.T) {
	digest := "sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
	platform := NewPlatform(digest, "linux/amd64", "amd64", "linux", "")

	if platform == nil {
		t.Fatal("Expected non-nil platform")
	}
	if platform.Manifest == nil {
		t.Error("Expected non-nil manifest artifact")
	}
	if platform.Manifest.Digest != digest {
		t.Errorf("Expected digest %s, got %s", digest, platform.Manifest.Digest)
	}
	if platform.Platform != "linux/amd64" {
		t.Errorf("Expected platform 'linux/amd64', got '%s'", platform.Platform)
	}
	if platform.Architecture != "amd64" {
		t.Errorf("Expected architecture 'amd64', got '%s'", platform.Architecture)
	}
	if platform.OS != "linux" {
		t.Errorf("Expected OS 'linux', got '%s'", platform.OS)
	}
	if len(platform.Referrers) != 0 {
		t.Errorf("Expected 0 referrers, got %d", len(platform.Referrers))
	}
}

func TestPlatformAddReferrer(t *testing.T) {
	platform := NewPlatform(
		"sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
		"linux/amd64", "amd64", "linux", "",
	)

	// Add SBOM
	sbom, _ := NewArtifact("sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890", ArtifactTypeSBOM)
	platform.AddReferrer(sbom)

	if len(platform.Referrers) != 1 {
		t.Errorf("Expected 1 referrer, got %d", len(platform.Referrers))
	}

	// Add provenance
	provenance, _ := NewArtifact("sha256:fedcba0987654321fedcba0987654321fedcba0987654321fedcba0987654321", ArtifactTypeProvenance)
	platform.AddReferrer(provenance)

	if len(platform.Referrers) != 2 {
		t.Errorf("Expected 2 referrers, got %d", len(platform.Referrers))
	}
}

func TestGraphAddPlatform(t *testing.T) {
	graph, _ := NewGraph("sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef")

	// Add platform
	platform := NewPlatform(
		"sha256:abcd1234567890abcdef1234567890abcdef1234567890abcdef1234567890ab",
		"linux/amd64", "amd64", "linux", "",
	)
	graph.AddPlatform(platform)

	if len(graph.Platforms) != 1 {
		t.Errorf("Expected 1 platform, got %d", len(graph.Platforms))
	}
	if graph.Platforms[0].Platform != "linux/amd64" {
		t.Errorf("Expected platform 'linux/amd64', got '%s'", graph.Platforms[0].Platform)
	}
}

func TestGraphWithPlatformsAndReferrers(t *testing.T) {
	// Create a multi-arch graph with platforms and their referrers
	graph, _ := NewGraph("sha256:1111111111111111111111111111111111111111111111111111111111111111")

	// Add linux/amd64 platform with SBOM and provenance
	amd64Platform := NewPlatform(
		"sha256:2222222222222222222222222222222222222222222222222222222222222222",
		"linux/amd64", "amd64", "linux", "",
	)
	amd64SBOM, _ := NewArtifact("sha256:3333333333333333333333333333333333333333333333333333333333333333", ArtifactTypeSBOM)
	amd64Prov, _ := NewArtifact("sha256:3333333333333333333333333333333333333333333333333333333333333333", ArtifactTypeProvenance)
	amd64Platform.AddReferrer(amd64SBOM)
	amd64Platform.AddReferrer(amd64Prov)
	graph.AddPlatform(amd64Platform)

	// Add linux/arm64 platform with SBOM and provenance
	arm64Platform := NewPlatform(
		"sha256:4444444444444444444444444444444444444444444444444444444444444444",
		"linux/arm64", "arm64", "linux", "",
	)
	arm64SBOM, _ := NewArtifact("sha256:5555555555555555555555555555555555555555555555555555555555555555", ArtifactTypeSBOM)
	arm64Prov, _ := NewArtifact("sha256:5555555555555555555555555555555555555555555555555555555555555555", ArtifactTypeProvenance)
	arm64Platform.AddReferrer(arm64SBOM)
	arm64Platform.AddReferrer(arm64Prov)
	graph.AddPlatform(arm64Platform)

	// Verify structure
	if len(graph.Platforms) != 2 {
		t.Errorf("Expected 2 platforms, got %d", len(graph.Platforms))
	}
	if len(graph.Platforms[0].Referrers) != 2 {
		t.Errorf("Expected 2 referrers for amd64, got %d", len(graph.Platforms[0].Referrers))
	}
	if len(graph.Platforms[1].Referrers) != 2 {
		t.Errorf("Expected 2 referrers for arm64, got %d", len(graph.Platforms[1].Referrers))
	}

	// Verify HasSBOM and HasProvenance work across platforms
	if !graph.HasSBOM() {
		t.Error("Expected HasSBOM to be true")
	}
	if !graph.HasProvenance() {
		t.Error("Expected HasProvenance to be true")
	}
}
