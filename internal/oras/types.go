package oras

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/registry/remote"
)

// ArtifactType represents the canonical type of an OCI artifact.
// It provides consistent type determination regardless of how the artifact was discovered.
type ArtifactType struct {
	Role     string // "platform", "sbom", "provenance", "signature", "vuln-scan", "vex", "attestation"
	Platform string // e.g., "linux/amd64" if Role is "platform"
}

// ChildArtifact represents any artifact that is a child of another artifact.
// This includes platform manifests, attestations, and signatures.
type ChildArtifact struct {
	Digest string       // SHA256 digest of this artifact
	Size   int64        // Size in bytes
	Type   ArtifactType // Unified type information
	Tag    string       // Tag if discovered via cosign (empty for buildx)
}

// DisplayType returns the user-facing type string for display in tables and output.
// For platform manifests, it returns the platform string (e.g., "linux/amd64").
// For other types, it returns the role directly.
func (t ArtifactType) DisplayType() string {
	if t.Role == "platform" {
		return t.Platform
	}
	return t.Role
}

// IsAttestation returns true if this artifact is an attestation type
// (sbom, provenance, vuln-scan, vex, or generic attestation).
func (t ArtifactType) IsAttestation() bool {
	switch t.Role {
	case "sbom", "provenance", "attestation", "vuln-scan", "vex":
		return true
	default:
		return false
	}
}

// IsPlatform returns true if this artifact is a platform-specific manifest.
func (t ArtifactType) IsPlatform() bool {
	return t.Role == "platform"
}

// ResolveType determines the canonical type of an OCI artifact by inspecting the manifest.
// This provides consistent type determination regardless of how the artifact was discovered.
func ResolveType(ctx context.Context, image, digest string) (ArtifactType, error) {
	// Validate inputs
	if image == "" {
		return ArtifactType{}, fmt.Errorf("image cannot be empty")
	}
	if digest == "" {
		return ArtifactType{}, fmt.Errorf("digest cannot be empty")
	}
	if !validateDigestFormat(digest) {
		return ArtifactType{}, fmt.Errorf("invalid digest format: %s", digest)
	}

	// Parse image reference
	registry, path, err := parseImageReference(image)
	if err != nil {
		return ArtifactType{}, err
	}

	// Create repository reference
	repo, err := remote.NewRepository(fmt.Sprintf("%s/%s", registry, path))
	if err != nil {
		return ArtifactType{}, fmt.Errorf("failed to create repository reference: %w", err)
	}

	// Configure authentication
	if err := configureAuth(ctx, repo); err != nil {
		return ArtifactType{}, fmt.Errorf("failed to configure authentication: %w", err)
	}

	// Resolve the digest to get the descriptor with media type
	desc, err := cachedResolve(ctx, repo, digest)
	if err != nil {
		return ArtifactType{}, fmt.Errorf("failed to resolve digest: %w", err)
	}

	// Check if this is an index (multi-arch manifest list)
	if desc.MediaType == "application/vnd.docker.distribution.manifest.list.v2+json" ||
		desc.MediaType == ocispec.MediaTypeImageIndex {
		return ArtifactType{Role: "index"}, nil
	}

	// It's a manifest - need to determine if it's a platform manifest or attestation
	// Fetch the manifest to inspect its contents
	manifestBytes, err := repo.Fetch(ctx, desc)
	if err != nil {
		return ArtifactType{}, fmt.Errorf("failed to fetch manifest: %w", err)
	}
	defer manifestBytes.Close()

	var manifest ocispec.Manifest
	if err := json.NewDecoder(manifestBytes).Decode(&manifest); err != nil {
		return ArtifactType{}, fmt.Errorf("failed to decode manifest: %w", err)
	}

	// Check for cosign signature first (before attestation check)
	if isSignature := checkSignatureIndicators(&manifest); isSignature {
		return ArtifactType{Role: "signature"}, nil
	}

	// Check for attestation indicators
	if isAttestation := checkAttestationIndicators(&manifest); isAttestation {
		// Determine the specific attestation type
		role := determineAttestationRole(&manifest)
		return ArtifactType{
			Role: role,
		}, nil
	}

	// It's a platform manifest - fetch config to get os/arch
	configBytes, err := repo.Fetch(ctx, manifest.Config)
	if err != nil {
		// If we can't fetch config, return as generic manifest
		return ArtifactType{
			Role:     "platform",
			Platform: "unknown",
		}, nil
	}
	defer configBytes.Close()

	var imageConfig ocispec.Image
	if err := json.NewDecoder(configBytes).Decode(&imageConfig); err != nil {
		return ArtifactType{
			Role:     "platform",
			Platform: "unknown",
		}, nil
	}

	// Build platform string
	platform := imageConfig.OS + "/" + imageConfig.Architecture
	if imageConfig.Variant != "" {
		platform += "/" + imageConfig.Variant
	}

	return ArtifactType{
		Role:     "platform",
		Platform: platform,
	}, nil
}

// checkSignatureIndicators checks if a manifest is a cosign signature.
func checkSignatureIndicators(manifest *ocispec.Manifest) bool {
	// Cosign signatures use application/vnd.dev.cosign.simplesigning.v1+json media type
	for _, layer := range manifest.Layers {
		if layer.MediaType == "application/vnd.dev.cosign.simplesigning.v1+json" {
			return true
		}
	}
	return false
}

// checkAttestationIndicators checks if a manifest is an attestation based on various indicators.
func checkAttestationIndicators(manifest *ocispec.Manifest) bool {
	// Check config media type for attestation indicators
	if strings.Contains(manifest.Config.MediaType, "in-toto") {
		return true
	}

	// Check for in-toto layers or cosign attestation layers
	for _, layer := range manifest.Layers {
		if strings.Contains(layer.MediaType, "in-toto") {
			return true
		}
		// Check layer annotations for predicate type (buildx or cosign style)
		if layer.Annotations != nil {
			if _, ok := layer.Annotations["in-toto.io/predicate-type"]; ok {
				return true
			}
			// Cosign stores predicate type as "predicateType"
			if _, ok := layer.Annotations["predicateType"]; ok {
				return true
			}
		}
	}

	// Check manifest annotations
	if manifest.Annotations != nil {
		if _, ok := manifest.Annotations["in-toto.io/predicate-type"]; ok {
			return true
		}
	}

	return false
}

// determineAttestationRole determines the specific role of an attestation manifest.
func determineAttestationRole(manifest *ocispec.Manifest) string {
	// Check annotations for predicate type
	predicateType := ""

	if manifest.Annotations != nil {
		if pt, ok := manifest.Annotations["in-toto.io/predicate-type"]; ok {
			predicateType = pt
		}
	}

	// Check config annotations
	if predicateType == "" && manifest.Config.Annotations != nil {
		if pt, ok := manifest.Config.Annotations["in-toto.io/predicate-type"]; ok {
			predicateType = pt
		}
	}

	// Check layer annotations (buildx uses "in-toto.io/predicate-type", cosign uses "predicateType")
	if predicateType == "" {
		for _, layer := range manifest.Layers {
			if layer.Annotations != nil {
				if pt, ok := layer.Annotations["in-toto.io/predicate-type"]; ok {
					predicateType = pt
					break
				}
				if pt, ok := layer.Annotations["predicateType"]; ok {
					predicateType = pt
					break
				}
			}
		}
	}

	// Map predicate type to role
	if predicateType != "" {
		if strings.Contains(predicateType, "spdx") || strings.Contains(predicateType, "cyclonedx") {
			return "sbom"
		}
		if strings.Contains(predicateType, "slsa") || strings.Contains(predicateType, "provenance") {
			return "provenance"
		}
		if strings.Contains(predicateType, "vuln") {
			return "vuln-scan"
		}
	}

	// Default to generic attestation
	return "attestation"
}
