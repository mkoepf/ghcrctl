package graph

import (
	"fmt"
	"strings"
)

// Artifact type constants
const (
	ArtifactTypeImage       = "image"
	ArtifactTypeSBOM        = "sbom"
	ArtifactTypeProvenance  = "provenance"
	ArtifactTypeAttestation = "attestation"
	ArtifactTypeUnknown     = "unknown"
)

// Artifact represents a single OCI artifact (image, SBOM, provenance, etc.)
type Artifact struct {
	Digest    string   // OCI digest (sha256:...)
	Type      string   // Type of artifact (image, sbom, provenance, etc.)
	Tags      []string // Tags pointing to this digest
	VersionID int64    // GHCR version ID (for deletion)
}

// Graph represents the complete OCI artifact graph
type Graph struct {
	Root      *Artifact   // The main image artifact
	Referrers []*Artifact // Related artifacts (SBOM, provenance, etc.)
}

// NewArtifact creates a new artifact with validation
func NewArtifact(digest, artifactType string) (*Artifact, error) {
	if digest == "" {
		return nil, fmt.Errorf("digest cannot be empty")
	}

	// Validate digest format
	if !strings.HasPrefix(digest, "sha256:") || len(digest) != 71 {
		return nil, fmt.Errorf("invalid digest format: %s", digest)
	}

	if artifactType == "" {
		return nil, fmt.Errorf("artifact type cannot be empty")
	}

	return &Artifact{
		Digest: digest,
		Type:   artifactType,
		Tags:   []string{},
	}, nil
}

// AddTag adds a tag to the artifact if it doesn't already exist
func (a *Artifact) AddTag(tag string) {
	// Check if tag already exists
	for _, t := range a.Tags {
		if t == tag {
			return
		}
	}
	a.Tags = append(a.Tags, tag)
}

// SetVersionID sets the GHCR version ID for this artifact
func (a *Artifact) SetVersionID(versionID int64) {
	a.VersionID = versionID
}

// NewGraph creates a new graph with the given root digest
func NewGraph(rootDigest string) (*Graph, error) {
	root, err := NewArtifact(rootDigest, ArtifactTypeImage)
	if err != nil {
		return nil, err
	}

	return &Graph{
		Root:      root,
		Referrers: []*Artifact{},
	}, nil
}

// AddReferrer adds a referrer artifact to the graph
func (g *Graph) AddReferrer(artifact *Artifact) {
	g.Referrers = append(g.Referrers, artifact)
}

// HasSBOM returns true if the graph contains an SBOM artifact
func (g *Graph) HasSBOM() bool {
	for _, r := range g.Referrers {
		if r.Type == ArtifactTypeSBOM {
			return true
		}
	}
	return false
}

// HasProvenance returns true if the graph contains a provenance artifact
func (g *Graph) HasProvenance() bool {
	for _, r := range g.Referrers {
		if r.Type == ArtifactTypeProvenance {
			return true
		}
	}
	return false
}

// GetReferrersByType returns all referrers of a specific type
func (g *Graph) GetReferrersByType(artifactType string) []*Artifact {
	result := []*Artifact{}
	for _, r := range g.Referrers {
		if r.Type == artifactType {
			result = append(result, r)
		}
	}
	return result
}

// UniqueArtifactCount returns the count of unique artifacts (by digest)
// This is important because a single manifest can contain multiple attestation types,
// resulting in multiple referrer entries with the same digest
func (g *Graph) UniqueArtifactCount() int {
	// Start with 1 for the root
	uniqueDigests := make(map[string]bool)
	uniqueDigests[g.Root.Digest] = true

	// Add unique referrer digests
	for _, r := range g.Referrers {
		uniqueDigests[r.Digest] = true
	}

	return len(uniqueDigests)
}
