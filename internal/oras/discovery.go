package oras

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
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

// ExtractParentDigestFromCosignTag extracts the parent digest from a cosign tag.
// Cosign tags have the format "sha256-<hex>.sig" or "sha256-<hex>.att".
// Returns the parent digest in standard format "sha256:<hex>" and true if valid.
func ExtractParentDigestFromCosignTag(tag string) (string, bool) {
	// Check for .sig or .att suffix
	var prefix string
	if strings.HasSuffix(tag, ".sig") {
		prefix = strings.TrimSuffix(tag, ".sig")
	} else if strings.HasSuffix(tag, ".att") {
		prefix = strings.TrimSuffix(tag, ".att")
	} else {
		return "", false
	}

	// Must start with "sha256-"
	if !strings.HasPrefix(prefix, "sha256-") {
		return "", false
	}

	// Extract the hex part and validate length (64 chars for sha256)
	hex := strings.TrimPrefix(prefix, "sha256-")
	if len(hex) != 64 {
		return "", false
	}

	// Convert back to standard digest format
	return "sha256:" + hex, true
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

// ExtractSubjectDigest extracts the parent (subject) digest from an in-toto attestation.
// This is useful for orphan attestations that have no tag linking them to their parent.
func ExtractSubjectDigest(ctx context.Context, image, attestationDigest string) (string, error) {
	// Validate inputs
	if image == "" {
		return "", fmt.Errorf("image cannot be empty")
	}
	if attestationDigest == "" {
		return "", fmt.Errorf("attestation digest cannot be empty")
	}
	if !validateDigestFormat(attestationDigest) {
		return "", fmt.Errorf("invalid digest format: %s", attestationDigest)
	}

	// Parse image reference
	registry, path, err := parseImageReference(image)
	if err != nil {
		return "", err
	}

	// Create repository reference
	repo, err := remote.NewRepository(fmt.Sprintf("%s/%s", registry, path))
	if err != nil {
		return "", fmt.Errorf("failed to create repository reference: %w", err)
	}

	// Configure authentication
	if err := configureAuth(ctx, repo); err != nil {
		return "", fmt.Errorf("failed to configure authentication: %w", err)
	}

	// Resolve the attestation digest
	desc, err := cachedResolve(ctx, repo, attestationDigest)
	if err != nil {
		return "", fmt.Errorf("failed to resolve attestation digest: %w", err)
	}

	// Fetch the manifest
	manifestBytes, err := repo.Fetch(ctx, desc)
	if err != nil {
		return "", fmt.Errorf("failed to fetch attestation manifest: %w", err)
	}
	defer manifestBytes.Close()

	var manifest ocispec.Manifest
	if err := json.NewDecoder(manifestBytes).Decode(&manifest); err != nil {
		return "", fmt.Errorf("failed to decode attestation manifest: %w", err)
	}

	// Check each layer for in-toto statement with subject
	for _, layer := range manifest.Layers {
		// Fetch the layer content
		layerBytes, err := repo.Fetch(ctx, layer)
		if err != nil {
			continue
		}

		// Try to extract subject from layer content
		subjectDigest, err := extractSubjectFromLayer(layerBytes)
		_ = layerBytes.Close()
		if err == nil && subjectDigest != "" {
			return subjectDigest, nil
		}
	}

	return "", fmt.Errorf("no subject digest found in attestation")
}

// extractSubjectFromLayer attempts to extract subject digest from a layer.
// Handles both direct in-toto statements and DSSE envelopes (used by cosign).
func extractSubjectFromLayer(r io.Reader) (string, error) {
	content, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}

	// First try: parse as DSSE envelope (cosign format)
	var envelope dsseEnvelope
	if err := json.Unmarshal(content, &envelope); err == nil && envelope.Payload != "" {
		// Decode base64 payload
		payload, err := base64.StdEncoding.DecodeString(envelope.Payload)
		if err != nil {
			return "", err
		}

		// Parse the in-toto statement from payload
		var statement inTotoStatement
		if err := json.Unmarshal(payload, &statement); err == nil {
			if digest := extractDigestFromSubjects(statement.Subject); digest != "" {
				return digest, nil
			}
		}
	}

	// Second try: parse as direct in-toto statement
	var statement inTotoStatement
	if err := json.Unmarshal(content, &statement); err == nil {
		if digest := extractDigestFromSubjects(statement.Subject); digest != "" {
			return digest, nil
		}
	}

	return "", fmt.Errorf("no subject found in layer")
}

// extractDigestFromSubjects extracts sha256 digest from in-toto subjects.
func extractDigestFromSubjects(subjects []inTotoSubject) string {
	if len(subjects) > 0 {
		if digest, ok := subjects[0].Digest["sha256"]; ok {
			return "sha256:" + digest
		}
	}
	return ""
}

// inTotoStatement represents the structure of an in-toto attestation statement.
type inTotoStatement struct {
	Subject []inTotoSubject `json:"subject"`
}

// inTotoSubject represents a subject in an in-toto statement.
type inTotoSubject struct {
	Name   string            `json:"name"`
	Digest map[string]string `json:"digest"`
}

// dsseEnvelope represents a DSSE (Dead Simple Signing Envelope) structure.
// Used by cosign for attestations.
type dsseEnvelope struct {
	PayloadType string `json:"payloadType"`
	Payload     string `json:"payload"` // base64 encoded
}
