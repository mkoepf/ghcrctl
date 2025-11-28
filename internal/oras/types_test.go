package oras

import (
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
