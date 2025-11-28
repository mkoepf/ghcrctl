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
	GetPlatformManifests(ctx context.Context, image, digest string) ([]oras.PlatformInfo, error)
	DiscoverReferrers(ctx context.Context, image, digest string) ([]oras.ReferrerInfo, error)
}

// defaultOrasClient wraps the oras package functions to implement OrasClient.
type defaultOrasClient struct{}

func (d *defaultOrasClient) GetPlatformManifests(ctx context.Context, image, digest string) ([]oras.PlatformInfo, error) {
	return oras.GetPlatformManifests(ctx, image, digest)
}

func (d *defaultOrasClient) DiscoverReferrers(ctx context.Context, image, digest string) ([]oras.ReferrerInfo, error) {
	return oras.DiscoverReferrers(ctx, image, digest)
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

// VersionGraph represents a group of related OCI artifact versions.
// This is the unified structure used by both versions and delete commands.
type VersionGraph struct {
	RootVersion gh.PackageVersionInfo
	Children    []VersionChild
	Type        string // "index", "manifest", or "standalone"
}

// VersionChild represents a child version with its artifact type.
type VersionChild struct {
	Version      gh.PackageVersionInfo
	ArtifactType string // "platform", "sbom", "provenance", or "attestation"
	Platform     string // e.g., "linux/amd64" for platform manifests
	Size         int64  // Size in bytes
	RefCount     int    // Number of graphs referencing this version (>1 means shared)
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

	// Discover platform manifests
	platforms, _ := b.orasClient.GetPlatformManifests(b.ctx, b.fullImage, rootDigest)
	if len(platforms) > 0 {
		graph.Type = "index"
		for _, p := range platforms {
			if childVer, found := cache.ByDigest[p.Digest]; found {
				graph.Children = append(graph.Children, VersionChild{
					Version:      childVer,
					ArtifactType: "platform",
					Platform:     p.Platform,
					Size:         p.Size,
				})
			}
		}
	}

	// Discover referrers (attestations)
	referrers, _ := b.orasClient.DiscoverReferrers(b.ctx, b.fullImage, rootDigest)
	for _, ref := range referrers {
		if childVer, found := cache.ByDigest[ref.Digest]; found {
			graph.Children = append(graph.Children, VersionChild{
				Version:      childVer,
				ArtifactType: ref.ArtifactType,
				Platform:     "",
				Size:         ref.Size,
			})
		}
	}

	// If no platforms but has referrers, it's a manifest (single-arch)
	if len(platforms) == 0 && len(referrers) > 0 {
		graph.Type = "manifest"
	}

	return graph, nil
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

	// For each version, check if it references the child digest
	for _, ver := range versions {
		candidateDigest := ver.Name

		// Skip if this is the same as the child digest
		if candidateDigest == digest {
			continue
		}

		// Check if this candidate has the child digest as a platform manifest
		platforms, err := b.orasClient.GetPlatformManifests(b.ctx, b.fullImage, candidateDigest)
		if err == nil {
			for _, p := range platforms {
				if p.Digest == digest {
					// Found a parent! This candidate has the child as a platform manifest
					return candidateDigest, nil
				}
			}
		}

		// Check if this candidate has the child digest as a referrer (attestation)
		referrers, err := b.orasClient.DiscoverReferrers(b.ctx, b.fullImage, candidateDigest)
		if err == nil {
			for _, r := range referrers {
				if r.Digest == digest {
					// Found a parent! This candidate has the child as a referrer
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
