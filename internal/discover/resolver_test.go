package discover

import (
	"context"
	"testing"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockResolver implements typeResolver for testing
type mockResolver struct {
	resolveFunc func(ctx context.Context, image, digest string) ([]string, error)
}

func (m *mockResolver) resolveVersionInfo(ctx context.Context, image, digest string) ([]string, int64, error) {
	types, err := m.resolveFunc(ctx, image, digest)
	return types, 1024, err // Return a default size of 1024 for testing
}

func TestResolveVersionInfo_Index(t *testing.T) {
	resolver := &mockResolver{
		resolveFunc: func(ctx context.Context, image, digest string) ([]string, error) {
			return []string{"index"}, nil
		},
	}

	types, _, err := resolver.resolveVersionInfo(context.Background(), "ghcr.io/test/image", "sha256:abc123")
	require.NoError(t, err)
	assert.Equal(t, []string{"index"}, types)
}

func TestResolveVersionInfo_Platform(t *testing.T) {
	resolver := &mockResolver{
		resolveFunc: func(ctx context.Context, image, digest string) ([]string, error) {
			return []string{"linux/amd64"}, nil
		},
	}

	types, _, err := resolver.resolveVersionInfo(context.Background(), "ghcr.io/test/image", "sha256:abc123")
	require.NoError(t, err)
	assert.Equal(t, []string{"linux/amd64"}, types)
}

func TestResolveVersionInfo_MultipleAttestations(t *testing.T) {
	resolver := &mockResolver{
		resolveFunc: func(ctx context.Context, image, digest string) ([]string, error) {
			return []string{"sbom", "provenance"}, nil
		},
	}

	types, _, err := resolver.resolveVersionInfo(context.Background(), "ghcr.io/test/image", "sha256:abc123")
	require.NoError(t, err)
	assert.Len(t, types, 2)
}

func TestPredicateToRole(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		predicateType string
		want          string
	}{
		{
			name:          "SPDX SBOM",
			predicateType: "https://spdx.dev/Document",
			want:          "sbom",
		},
		{
			name:          "CycloneDX SBOM",
			predicateType: "https://cyclonedx.org/bom",
			want:          "sbom",
		},
		{
			name:          "SLSA provenance",
			predicateType: "https://slsa.dev/provenance/v0.2",
			want:          "provenance",
		},
		{
			name:          "generic provenance",
			predicateType: "https://example.com/provenance/v1",
			want:          "provenance",
		},
		{
			name:          "vulnerability scan",
			predicateType: "https://cosign.sigstore.dev/attestation/vuln/v1",
			want:          "vuln-scan",
		},
		{
			name:          "OpenVEX",
			predicateType: "https://openvex.dev/ns/v0.2.0",
			want:          "vex",
		},
		{
			name:          "generic VEX",
			predicateType: "https://example.com/vex/document",
			want:          "vex",
		},
		{
			name:          "unknown predicate",
			predicateType: "https://example.com/custom/predicate",
			want:          "attestation",
		},
		{
			name:          "empty predicate",
			predicateType: "",
			want:          "attestation",
		},
		{
			name:          "case insensitive SPDX",
			predicateType: "https://SPDX.dev/Document",
			want:          "sbom",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := predicateToRole(tt.predicateType)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIsSignature(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		manifest *ocispec.Manifest
		want     bool
	}{
		{
			name: "cosign signature layer",
			manifest: &ocispec.Manifest{
				Layers: []ocispec.Descriptor{
					{MediaType: "application/vnd.dev.cosign.simplesigning.v1+json"},
				},
			},
			want: true,
		},
		{
			name: "cosign signature among other layers",
			manifest: &ocispec.Manifest{
				Layers: []ocispec.Descriptor{
					{MediaType: "application/vnd.oci.image.layer.v1.tar+gzip"},
					{MediaType: "application/vnd.dev.cosign.simplesigning.v1+json"},
				},
			},
			want: true,
		},
		{
			name: "regular image layers",
			manifest: &ocispec.Manifest{
				Layers: []ocispec.Descriptor{
					{MediaType: "application/vnd.oci.image.layer.v1.tar+gzip"},
					{MediaType: "application/vnd.oci.image.layer.v1.tar+gzip"},
				},
			},
			want: false,
		},
		{
			name: "empty layers",
			manifest: &ocispec.Manifest{
				Layers: []ocispec.Descriptor{},
			},
			want: false,
		},
		{
			name:     "nil layers",
			manifest: &ocispec.Manifest{},
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isSignature(tt.manifest))
		})
	}
}

func TestIsAttestation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		manifest *ocispec.Manifest
		want     bool
	}{
		{
			name: "in-toto config media type",
			manifest: &ocispec.Manifest{
				Config: ocispec.Descriptor{
					MediaType: "application/vnd.in-toto+json",
				},
			},
			want: true,
		},
		{
			name: "in-toto layer media type",
			manifest: &ocispec.Manifest{
				Config: ocispec.Descriptor{
					MediaType: "application/vnd.oci.image.config.v1+json",
				},
				Layers: []ocispec.Descriptor{
					{MediaType: "application/vnd.in-toto+json"},
				},
			},
			want: true,
		},
		{
			name: "predicate type annotation",
			manifest: &ocispec.Manifest{
				Config: ocispec.Descriptor{
					MediaType: "application/vnd.oci.image.config.v1+json",
				},
				Layers: []ocispec.Descriptor{
					{
						MediaType: "application/vnd.dsse.envelope.v1+json",
						Annotations: map[string]string{
							"in-toto.io/predicate-type": "https://spdx.dev/Document",
						},
					},
				},
			},
			want: true,
		},
		{
			name: "predicateType annotation (alternate key)",
			manifest: &ocispec.Manifest{
				Config: ocispec.Descriptor{
					MediaType: "application/vnd.oci.image.config.v1+json",
				},
				Layers: []ocispec.Descriptor{
					{
						MediaType: "application/vnd.dsse.envelope.v1+json",
						Annotations: map[string]string{
							"predicateType": "https://slsa.dev/provenance/v0.2",
						},
					},
				},
			},
			want: true,
		},
		{
			name: "regular image manifest",
			manifest: &ocispec.Manifest{
				Config: ocispec.Descriptor{
					MediaType: "application/vnd.oci.image.config.v1+json",
				},
				Layers: []ocispec.Descriptor{
					{MediaType: "application/vnd.oci.image.layer.v1.tar+gzip"},
				},
			},
			want: false,
		},
		{
			name:     "empty manifest",
			manifest: &ocispec.Manifest{},
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isAttestation(tt.manifest))
		})
	}
}

func TestDetermineAttestationRoles(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		manifest *ocispec.Manifest
		want     []string
	}{
		{
			name: "SBOM attestation",
			manifest: &ocispec.Manifest{
				Layers: []ocispec.Descriptor{
					{
						Annotations: map[string]string{
							"in-toto.io/predicate-type": "https://spdx.dev/Document",
						},
					},
				},
			},
			want: []string{"sbom"},
		},
		{
			name: "provenance attestation",
			manifest: &ocispec.Manifest{
				Layers: []ocispec.Descriptor{
					{
						Annotations: map[string]string{
							"predicateType": "https://slsa.dev/provenance/v0.2",
						},
					},
				},
			},
			want: []string{"provenance"},
		},
		{
			name: "multiple attestations",
			manifest: &ocispec.Manifest{
				Layers: []ocispec.Descriptor{
					{
						Annotations: map[string]string{
							"in-toto.io/predicate-type": "https://spdx.dev/Document",
						},
					},
					{
						Annotations: map[string]string{
							"predicateType": "https://slsa.dev/provenance/v0.2",
						},
					},
				},
			},
			want: []string{"sbom", "provenance"},
		},
		{
			name: "no annotations",
			manifest: &ocispec.Manifest{
				Layers: []ocispec.Descriptor{
					{MediaType: "application/vnd.in-toto+json"},
				},
			},
			want: []string{},
		},
		{
			name:     "empty manifest",
			manifest: &ocispec.Manifest{},
			want:     []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := determineAttestationRoles(tt.manifest)

			// For multiple roles, order may vary - check containment
			assert.Len(t, got, len(tt.want))

			gotMap := make(map[string]bool)
			for _, r := range got {
				gotMap[r] = true
			}
			for _, w := range tt.want {
				assert.True(t, gotMap[w], "missing expected role %q, got %v", w, got)
			}
		})
	}
}
