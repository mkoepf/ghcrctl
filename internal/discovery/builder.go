package discovery

import (
	"context"

	"github.com/mhk/ghcrctl/internal/gh"
	"github.com/mhk/ghcrctl/internal/oras"
)

// GHClient defines the interface for GitHub API operations needed by GraphBuilder.
// This allows for testing with mock implementations.
type GHClient interface {
	ListPackageVersions(ctx context.Context, owner, ownerType, packageName string) ([]gh.PackageVersionInfo, error)
}

// OrasClient defines the interface for OCI registry operations needed by GraphBuilder.
// This allows for testing with mock implementations.
type OrasClient interface {
	DiscoverChildren(ctx context.Context, image, parentDigest string, allTags []string) ([]oras.ChildArtifact, error)
}

// defaultOrasClient wraps the oras package functions to implement OrasClient.
type defaultOrasClient struct{}

func (d *defaultOrasClient) DiscoverChildren(ctx context.Context, image, parentDigest string, allTags []string) ([]oras.ChildArtifact, error) {
	return oras.DiscoverChildren(ctx, image, parentDigest, allTags)
}

// GraphBuilder encapsulates graph discovery logic for OCI artifacts.
// It provides a unified way to build artifact graphs across different commands.
type GraphBuilder struct {
	ctx        context.Context
	ghClient   GHClient
	orasClient OrasClient
	fullImage  string
	owner      string
	ownerType  string
	imageName  string
}

// VersionCache provides efficient lookups of version information by digest or ID.
type VersionCache struct {
	ByDigest map[string]gh.PackageVersionInfo
	ByID     map[int64]gh.PackageVersionInfo
}

// AllVersions returns all versions in the cache as a slice.
func (c *VersionCache) AllVersions() []gh.PackageVersionInfo {
	versions := make([]gh.PackageVersionInfo, 0, len(c.ByID))
	for _, v := range c.ByID {
		versions = append(versions, v)
	}
	return versions
}

// VersionGraph represents a group of related OCI artifact versions.
// This is the unified structure used by both versions and delete commands.
type VersionGraph struct {
	RootVersion gh.PackageVersionInfo
	Children    []VersionChild
	Type        string // "index", "manifest", or "standalone"
}

// VersionChild represents a child version with its artifact type.
type VersionChild struct {
	Version  gh.PackageVersionInfo
	Type     oras.ArtifactType // Unified type from oras package
	Size     int64             // Size in bytes
	RefCount int               // Number of graphs referencing this version (>1 means shared)
}

// NewVersionCacheFromSlice creates a VersionCache from an existing slice of versions.
// This is useful when versions have already been fetched and don't need to be re-fetched.
func NewVersionCacheFromSlice(versions []gh.PackageVersionInfo) *VersionCache {
	cache := &VersionCache{
		ByDigest: make(map[string]gh.PackageVersionInfo),
		ByID:     make(map[int64]gh.PackageVersionInfo),
	}

	for _, ver := range versions {
		cache.ByDigest[ver.Name] = ver
		cache.ByID[ver.ID] = ver
	}

	return cache
}

// NewGraphBuilder creates a new GraphBuilder instance.
func NewGraphBuilder(ctx context.Context, ghClient GHClient, fullImage, owner, ownerType, imageName string) *GraphBuilder {
	return &GraphBuilder{
		ctx:        ctx,
		ghClient:   ghClient,
		orasClient: &defaultOrasClient{},
		fullImage:  fullImage,
		owner:      owner,
		ownerType:  ownerType,
		imageName:  imageName,
	}
}

// NewGraphBuilderWithOras creates a new GraphBuilder with a custom OrasClient (for testing).
func NewGraphBuilderWithOras(ctx context.Context, ghClient GHClient, orasClient OrasClient, fullImage, owner, ownerType, imageName string) *GraphBuilder {
	return &GraphBuilder{
		ctx:        ctx,
		ghClient:   ghClient,
		orasClient: orasClient,
		fullImage:  fullImage,
		owner:      owner,
		ownerType:  ownerType,
		imageName:  imageName,
	}
}

// BuildGraph builds a complete OCI artifact graph starting from the given digest.
// It discovers platform manifests, attestations, and handles parent finding automatically.
// If the digest is a child artifact (platform manifest or attestation), it will attempt
// to find and return the parent graph instead.
func (b *GraphBuilder) BuildGraph(rootDigest string, cache *VersionCache) (*VersionGraph, error) {
	// Get root version info from cache
	rootVersion, found := cache.ByDigest[rootDigest]
	if !found {
		return nil, nil // Digest not found in cache
	}

	graph := &VersionGraph{
		RootVersion: rootVersion,
		Children:    []VersionChild{},
		Type:        "standalone", // Default, will be updated based on discovery
	}

	// Extract all tags from cache for cosign discovery
	allTags := extractAllTags(cache)

	// Discover all children using unified discovery
	children, _ := b.orasClient.DiscoverChildren(b.ctx, b.fullImage, rootDigest, allTags)

	// Determine graph type and add children
	hasPlatform := false
	hasAttestation := false

	for _, child := range children {
		if childVer, found := cache.ByDigest[child.Digest]; found {
			graph.Children = append(graph.Children, VersionChild{
				Version: childVer,
				Type:    child.Type,
				Size:    child.Size,
			})

			if child.Type.IsPlatform() {
				hasPlatform = true
			}
			if child.Type.IsAttestation() || child.Type.Role == "signature" {
				hasAttestation = true
			}
		}
	}

	// Determine graph type based on children
	if hasPlatform {
		graph.Type = "index"
	} else if hasAttestation {
		graph.Type = "manifest"
	}

	return graph, nil
}

// extractAllTags extracts all unique tags from a version cache.
func extractAllTags(cache *VersionCache) []string {
	tagSet := make(map[string]bool)
	for _, ver := range cache.ByDigest {
		for _, tag := range ver.Tags {
			tagSet[tag] = true
		}
	}

	tags := make([]string, 0, len(tagSet))
	for tag := range tagSet {
		tags = append(tags, tag)
	}
	return tags
}

// GetVersionCache fetches all package versions and creates an efficient lookup cache.
func (b *GraphBuilder) GetVersionCache() (*VersionCache, error) {
	// Fetch all versions for this package
	allVersions, err := b.ghClient.ListPackageVersions(b.ctx, b.owner, b.ownerType, b.imageName)
	if err != nil {
		return nil, err
	}

	// Create cache with both digest and ID lookups
	cache := &VersionCache{
		ByDigest: make(map[string]gh.PackageVersionInfo),
		ByID:     make(map[int64]gh.PackageVersionInfo),
	}

	// Populate both maps
	for _, ver := range allVersions {
		cache.ByDigest[ver.Name] = ver
		cache.ByID[ver.ID] = ver
	}

	return cache, nil
}

// FindParentDigest attempts to find a parent digest that contains the given digest
// as a child (platform manifest or attestation). Returns empty string if no parent found.
func (b *GraphBuilder) FindParentDigest(digest string, cache *VersionCache) (string, error) {
	// Find the child version's ID to optimize search order
	var childID int64
	if ver, found := cache.ByDigest[digest]; found {
		childID = ver.ID
	}

	// Get all versions as a slice for sorting
	versions := make([]gh.PackageVersionInfo, 0, len(cache.ByDigest))
	for _, ver := range cache.ByDigest {
		versions = append(versions, ver)
	}

	// Sort versions by proximity to child ID - related artifacts are typically created together
	// and have IDs within a small range (typically ±4-20)
	if childID != 0 {
		versions = sortByIDProximity(versions, childID)
	}

	// Extract all tags for cosign discovery
	allTags := extractAllTags(cache)

	// For each version, check if it references the child digest
	for _, ver := range versions {
		candidateDigest := ver.Name

		// Skip if this is the same as the child digest
		if candidateDigest == digest {
			continue
		}

		// Check if this candidate has the child digest as a child
		children, err := b.orasClient.DiscoverChildren(b.ctx, b.fullImage, candidateDigest, allTags)
		if err == nil {
			for _, child := range children {
				if child.Digest == digest {
					// Found a parent! This candidate has the child
					return candidateDigest, nil
				}
			}
		}
	}

	// No parent found
	return "", nil
}

// sortByIDProximity sorts versions by their distance from a target ID.
// Versions closer to the target ID are placed first.
func sortByIDProximity(versions []gh.PackageVersionInfo, targetID int64) []gh.PackageVersionInfo {
	// Create a copy to avoid modifying the original slice
	sorted := make([]gh.PackageVersionInfo, len(versions))
	copy(sorted, versions)

	// Sort by absolute distance from targetID
	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			distI := sorted[i].ID - targetID
			if distI < 0 {
				distI = -distI
			}
			distJ := sorted[j].ID - targetID
			if distJ < 0 {
				distJ = -distJ
			}
			if distJ < distI {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	return sorted
}
