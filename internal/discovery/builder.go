package discovery

import (
	"context"

	"github.com/mhk/ghcrctl/internal/gh"
	"github.com/mhk/ghcrctl/internal/graph"
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
	byDigest map[string]gh.PackageVersionInfo
	byID     map[int64]gh.PackageVersionInfo
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
		byDigest: make(map[string]gh.PackageVersionInfo),
		byID:     make(map[int64]gh.PackageVersionInfo),
	}

	// Populate both maps
	for _, ver := range allVersions {
		cache.byDigest[ver.Name] = ver
		cache.byID[ver.ID] = ver
	}

	return cache, nil
}

// FindParentDigest attempts to find a parent digest that contains the given digest
// as a child (platform manifest or attestation). Returns empty string if no parent found.
func (b *GraphBuilder) FindParentDigest(digest string, cache *VersionCache) (string, error) {
	// TODO: Implementation will be added incrementally
	return "", nil
}
