package oras

import (
	"context"
	"fmt"
	"strings"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/registry/remote"
)

// DiscoverChildren discovers all child artifacts for a given parent digest.
// This is the single entry point for all artifact discovery, combining:
// - Platform manifests from image index
// - Attestations embedded in image index (buildx style)
// - Signatures and attestations from cosign tags (future)
//
// The allTags parameter is optional - if provided, it enables cosign tag discovery.
func DiscoverChildren(ctx context.Context, image, parentDigest string, allTags []string) ([]ChildArtifact, error) {
	// Validate inputs
	if image == "" {
		return nil, fmt.Errorf("image cannot be empty")
	}
	if parentDigest == "" {
		return nil, fmt.Errorf("digest cannot be empty")
	}
	if !validateDigestFormat(parentDigest) {
		return nil, fmt.Errorf("invalid digest format: %s", parentDigest)
	}

	// Parse image reference
	registry, path, err := parseImageReference(image)
	if err != nil {
		return nil, err
	}

	// Create repository reference
	repo, err := remote.NewRepository(fmt.Sprintf("%s/%s", registry, path))
	if err != nil {
		return nil, fmt.Errorf("failed to create repository reference: %w", err)
	}

	// Configure authentication
	if err := configureAuth(ctx, repo); err != nil {
		return nil, fmt.Errorf("failed to configure authentication: %w", err)
	}

	var children []ChildArtifact

	// Discover from index (platforms + buildx attestations)
	indexChildren, err := discoverFromIndex(ctx, repo, parentDigest)
	if err == nil {
		children = append(children, indexChildren...)
	}

	// Discover from cosign tags (signatures + attestations)
	if len(allTags) > 0 {
		cosignChildren, err := discoverFromCosignTags(ctx, repo, parentDigest, allTags)
		if err == nil {
			children = append(children, cosignChildren...)
		}
	}

	return children, nil
}

// discoverFromIndex fetches an image index and extracts platform manifests and attestations.
func discoverFromIndex(ctx context.Context, repo *remote.Repository, digest string) ([]ChildArtifact, error) {
	// Resolve the digest to get the descriptor
	desc, err := cachedResolve(ctx, repo, digest)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve digest: %w", err)
	}

	// Check if this is an index
	if desc.MediaType != "application/vnd.docker.distribution.manifest.list.v2+json" &&
		desc.MediaType != ocispec.MediaTypeImageIndex {
		// Not an index, no children to discover
		return nil, nil
	}

	// Fetch and parse the index
	index, err := cachedFetchIndex(ctx, repo, desc)
	if err != nil {
		return nil, err
	}

	var children []ChildArtifact

	for _, manifest := range index.Manifests {
		childList := manifestToChildArtifacts(ctx, repo, manifest)
		children = append(children, childList...)
	}

	return children, nil
}

// manifestToChildArtifacts converts an index manifest entry to ChildArtifacts.
// For multi-layer attestations, this returns multiple entries (one per layer type).
func manifestToChildArtifacts(ctx context.Context, repo *remote.Repository, manifest ocispec.Descriptor) []ChildArtifact {
	// Check if this is an attestation
	isAttestation := false

	// Check annotations for attestation marker
	if manifest.Annotations != nil {
		if refType, ok := manifest.Annotations["vnd.docker.reference.type"]; ok {
			if refType == "attestation-manifest" {
				isAttestation = true
			}
		}
	}

	// Check if platform is unknown (common for attestations)
	if manifest.Platform != nil {
		if manifest.Platform.OS == "unknown" && manifest.Platform.Architecture == "unknown" {
			isAttestation = true
		}
	}

	// Check for in-toto attestation media types
	if strings.Contains(manifest.MediaType, "in-toto") ||
		strings.Contains(manifest.ArtifactType, "in-toto") {
		isAttestation = true
	}

	if isAttestation {
		// Determine attestation roles - may return multiple for multi-layer manifests
		roles := determineRolesFromManifest(ctx, repo, manifest)
		if len(roles) == 0 {
			roles = []string{"attestation"}
		}

		var children []ChildArtifact
		for _, role := range roles {
			children = append(children, ChildArtifact{
				Digest: manifest.Digest.String(),
				Size:   manifest.Size,
				Type:   ArtifactType{Role: role},
				Tag:    "",
			})
		}
		return children
	}

	// Check if it's a platform manifest
	if manifest.Platform != nil {
		platformStr := manifest.Platform.OS + "/" + manifest.Platform.Architecture
		if manifest.Platform.Variant != "" {
			platformStr += "/" + manifest.Platform.Variant
		}

		return []ChildArtifact{{
			Digest: manifest.Digest.String(),
			Size:   manifest.Size,
			Type:   ArtifactType{Role: "platform", Platform: platformStr},
			Tag:    "",
		}}
	}

	// Unknown manifest type, skip
	return nil
}

// determineRolesFromManifest fetches an attestation manifest and determines all roles.
// For multi-layer attestations, this returns multiple roles (one per layer type).
func determineRolesFromManifest(ctx context.Context, repo *remote.Repository, desc ocispec.Descriptor) []string {
	// Fetch the attestation manifest (cached to avoid redundant fetches)
	manifest, err := cachedFetchManifest(ctx, repo, desc)
	if err != nil {
		return nil
	}

	// Collect all unique roles from layers
	roleSet := make(map[string]bool)

	// Check layer annotations for predicate type
	// buildx uses "in-toto.io/predicate-type", cosign uses "predicateType"
	for _, layer := range manifest.Layers {
		if layer.Annotations != nil {
			// Check buildx annotation key
			if predType, ok := layer.Annotations["in-toto.io/predicate-type"]; ok {
				role := predicateTypeToRole(predType)
				roleSet[role] = true
			}
			// Check cosign annotation key
			if predType, ok := layer.Annotations["predicateType"]; ok {
				role := predicateTypeToRole(predType)
				roleSet[role] = true
			}
		}
	}

	// If no roles found in layers, check manifest and config annotations
	if len(roleSet) == 0 {
		if manifest.Annotations != nil {
			if predType, ok := manifest.Annotations["in-toto.io/predicate-type"]; ok {
				roleSet[predicateTypeToRole(predType)] = true
			}
		}
		if manifest.Config.Annotations != nil {
			if predType, ok := manifest.Config.Annotations["in-toto.io/predicate-type"]; ok {
				roleSet[predicateTypeToRole(predType)] = true
			}
		}
	}

	// Convert set to slice
	var roles []string
	for role := range roleSet {
		roles = append(roles, role)
	}
	return roles
}

// predicateTypeToRole maps in-toto predicate types to roles.
func predicateTypeToRole(predicateType string) string {
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

// discoverFromCosignTags discovers cosign-stored artifacts by tag pattern matching.
func discoverFromCosignTags(ctx context.Context, repo *remote.Repository, parentDigest string, allTags []string) ([]ChildArtifact, error) {
	sigTag, attTag := findCosignTags(parentDigest, allTags)

	var children []ChildArtifact

	// Discover signature
	if sigTag != "" {
		desc, err := repo.Resolve(ctx, sigTag)
		if err == nil {
			children = append(children, ChildArtifact{
				Digest: desc.Digest.String(),
				Size:   desc.Size,
				Type:   ArtifactType{Role: "signature"},
				Tag:    sigTag,
			})
		}
	}

	// Discover attestation
	if attTag != "" {
		desc, err := repo.Resolve(ctx, attTag)
		if err == nil {
			// Determine specific attestation type by fetching and parsing the manifest
			roles := determineRolesFromManifest(ctx, repo, desc)
			if len(roles) == 0 {
				roles = []string{"attestation"}
			}
			// For cosign attestations, add each role as a separate child
			for _, role := range roles {
				children = append(children, ChildArtifact{
					Digest: desc.Digest.String(),
					Size:   desc.Size,
					Type:   ArtifactType{Role: role},
					Tag:    attTag,
				})
			}
		}
	}

	return children, nil
}

// digestToTagPrefix converts a digest to the cosign tag prefix.
// "sha256:abc123..." -> "sha256-abc123..."
func digestToTagPrefix(digest string) string {
	return strings.Replace(digest, ":", "-", 1)
}

// findCosignTags finds cosign signature and attestation tags for a given digest.
func findCosignTags(parentDigest string, allTags []string) (sigTag, attTag string) {
	prefix := digestToTagPrefix(parentDigest)
	expectedSig := prefix + ".sig"
	expectedAtt := prefix + ".att"

	for _, tag := range allTags {
		if tag == expectedSig {
			sigTag = tag
		}
		if tag == expectedAtt {
			attTag = tag
		}
	}

	return sigTag, attTag
}
