package discovery

import (
	"context"

	"github.com/mhk/ghcrctl/internal/gh"
	"github.com/mhk/ghcrctl/internal/graph"
	"github.com/mhk/ghcrctl/internal/oras"
)

// GHClient defines the interface for GitHub API operations needed by GraphBuilder.
// This allows for testing with mock implementations.
type GHClient interface {
	ListPackageVersions(ctx context.Context, owner, ownerType, packageName string) ([]gh.PackageVersionInfo, error)
}

// GraphBuilder encapsulates graph discovery logic for OCI artifacts.
// It provides a unified way to build artifact graphs across different commands.
type GraphBuilder struct {
	ctx       context.Context
	ghClient  GHClient
	fullImage string
	owner     string
	ownerType string
	imageName string
}

// VersionCache provides efficient lookups of version information by digest or ID.
type VersionCache struct {
	ByDigest map[string]gh.PackageVersionInfo
	ByID     map[int64]gh.PackageVersionInfo
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
		ctx:       ctx,
		ghClient:  ghClient,
		fullImage: fullImage,
		owner:     owner,
		ownerType: ownerType,
		imageName: imageName,
	}
}

// BuildGraph builds a complete OCI artifact graph starting from the given digest.
// It discovers platform manifests, attestations, and handles parent finding automatically.
func (b *GraphBuilder) BuildGraph(digest string) (*graph.Graph, error) {
	// TODO: Implementation will be added incrementally
	return nil, nil
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
		// Note: Using ORAS package functions directly for now
		// TODO: Consider making ORAS operations injectable for testing
		platforms, err := oras.GetPlatformManifests(b.ctx, b.fullImage, candidateDigest)
		if err == nil {
			for _, p := range platforms {
				if p.Digest == digest {
					// Found a parent! This candidate has the child as a platform manifest
					return candidateDigest, nil
				}
			}
		}

		// Check if this candidate has the child digest as a referrer (attestation)
		referrers, err := oras.DiscoverReferrers(b.ctx, b.fullImage, candidateDigest)
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
