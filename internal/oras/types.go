package oras

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
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
	// Fetch the manifest to inspect its contents (cached to avoid redundant fetches)
	manifest, err := cachedFetchManifest(ctx, repo, desc)
	if err != nil {
		return ArtifactType{}, err
	}

	// Check for cosign signature first (before attestation check)
	if isSignature := checkSignatureIndicators(manifest); isSignature {
		return ArtifactType{Role: "signature"}, nil
	}

	// Check for attestation indicators
	if isAttestation := checkAttestationIndicators(manifest); isAttestation {
		// Determine the specific attestation type
		role := determineAttestationRole(manifest)
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

// determineAttestationRole determines the specific role(s) of an attestation manifest.
// For multi-predicate attestations (e.g., cosign with multiple layers), returns combined roles.
func determineAttestationRole(manifest *ocispec.Manifest) string {
	// Collect all unique roles from layers
	roleSet := make(map[string]bool)

	// Check layer annotations for predicate type
	// buildx uses "in-toto.io/predicate-type", cosign uses "predicateType"
	for _, layer := range manifest.Layers {
		if layer.Annotations != nil {
			// Check buildx annotation key
			if predType, ok := layer.Annotations["in-toto.io/predicate-type"]; ok {
				role := predicateToRole(predType)
				roleSet[role] = true
			}
			// Check cosign annotation key
			if predType, ok := layer.Annotations["predicateType"]; ok {
				role := predicateToRole(predType)
				roleSet[role] = true
			}
		}
	}

	// If no roles found in layers, check manifest and config annotations
	if len(roleSet) == 0 {
		if manifest.Annotations != nil {
			if predType, ok := manifest.Annotations["in-toto.io/predicate-type"]; ok {
				roleSet[predicateToRole(predType)] = true
			}
		}
		if manifest.Config.Annotations != nil {
			if predType, ok := manifest.Config.Annotations["in-toto.io/predicate-type"]; ok {
				roleSet[predicateToRole(predType)] = true
			}
		}
	}

	// If no roles found, default to generic attestation
	if len(roleSet) == 0 {
		return "attestation"
	}

	// Convert set to sorted slice and join
	var roles []string
	for role := range roleSet {
		roles = append(roles, role)
	}
	// Sort for consistent output
	sort.Strings(roles)
	return strings.Join(roles, ", ")
}

// predicateToRole maps in-toto predicate types to roles.
func predicateToRole(predicateType string) string {
	lower := strings.ToLower(predicateType)

	if strings.Contains(lower, "spdx") || strings.Contains(lower, "cyclonedx") {
		return "sbom"
	}
	if strings.Contains(lower, "slsa") || strings.Contains(lower, "provenance") {
		return "provenance"
	}
	if strings.Contains(lower, "vuln") {
		return "vuln-scan"
	}
	if strings.Contains(lower, "openvex") || strings.Contains(lower, "vex") {
		return "vex"
	}

	return "attestation"
}
