package discover

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/registry/remote"

	"github.com/mkoepf/ghcrctl/internal/gh"
)

// ChildDiscoverer discovers children of an OCI artifact.
type ChildDiscoverer interface {
	DiscoverChildren(ctx context.Context, image, digest string, allTags []string) ([]string, error)
}

// PackageDiscoverer discovers all versions and their relationships.
type PackageDiscoverer struct {
	Resolver        TypeResolver
	ChildDiscoverer ChildDiscoverer
}

// NewPackageDiscoverer creates a new PackageDiscoverer with default implementations.
func NewPackageDiscoverer() *PackageDiscoverer {
	resolver := NewOrasResolver()
	return &PackageDiscoverer{
		Resolver:        resolver,
		ChildDiscoverer: &OrasChildDiscoverer{resolver: resolver},
	}
}

// DiscoverPackage discovers all versions and their relationships.
func (d *PackageDiscoverer) DiscoverPackage(ctx context.Context, image string, versions []gh.PackageVersionInfo, allTags []string) ([]VersionInfo, error) {
	// Build version map
	versionMap := make(map[string]*VersionInfo)

	for _, v := range versions {
		info := &VersionInfo{
			ID:        v.ID,
			Digest:    v.Name,
			Tags:      v.Tags,
			CreatedAt: v.CreatedAt,
		}
		versionMap[v.Name] = info
	}

	// Resolve types and discover children for each version in parallel
	var wg sync.WaitGroup
	for digest, info := range versionMap {
		wg.Add(1)
		go func(digest string, info *VersionInfo) {
			defer wg.Done()

			types, err := d.Resolver.ResolveVersionType(ctx, image, digest)
			if err != nil {
				info.Types = []string{"unknown"}
			} else {
				info.Types = types
			}

			children, err := d.ChildDiscoverer.DiscoverChildren(ctx, image, digest, allTags)
			if err == nil {
				info.OutgoingRefs = children
			}
		}(digest, info)
	}
	wg.Wait()

	// Infer incoming refs from outgoing refs
	for digest, info := range versionMap {
		for _, outRef := range info.OutgoingRefs {
			if target, ok := versionMap[outRef]; ok {
				target.IncomingRefs = append(target.IncomingRefs, digest)
			}
		}
	}

	// Convert to slice
	result := make([]VersionInfo, 0, len(versionMap))
	for _, info := range versionMap {
		result = append(result, *info)
	}

	return result, nil
}

// OrasChildDiscoverer discovers children using ORAS.
type OrasChildDiscoverer struct {
	resolver *OrasResolver
}

// DiscoverChildren discovers children of an OCI artifact.
func (d *OrasChildDiscoverer) DiscoverChildren(ctx context.Context, image, digest string, allTags []string) ([]string, error) {
	if !validateDigestFormat(digest) {
		return nil, fmt.Errorf("invalid digest: %s", digest)
	}

	registry, path, err := parseImageReference(image)
	if err != nil {
		return nil, err
	}

	repo, err := remote.NewRepository(fmt.Sprintf("%s/%s", registry, path))
	if err != nil {
		return nil, err
	}

	d.resolver.configureAuth(ctx, repo)

	desc, err := repo.Resolve(ctx, digest)
	if err != nil {
		return nil, err
	}

	var children []string

	// Only discover children from index manifests
	if desc.MediaType == ocispec.MediaTypeImageIndex ||
		desc.MediaType == "application/vnd.docker.distribution.manifest.list.v2+json" {
		children, err = d.discoverFromIndex(ctx, repo, desc)
		if err != nil {
			return nil, err
		}
	}

	// Discover from cosign tags
	cosignChildren := d.discoverFromCosignTags(ctx, repo, digest, allTags)
	children = append(children, cosignChildren...)

	return children, nil
}

func (d *OrasChildDiscoverer) discoverFromIndex(ctx context.Context, repo *remote.Repository, desc ocispec.Descriptor) ([]string, error) {
	indexBytes, err := repo.Fetch(ctx, desc)
	if err != nil {
		return nil, err
	}
	defer indexBytes.Close()

	var index ocispec.Index
	if err := json.NewDecoder(indexBytes).Decode(&index); err != nil {
		return nil, err
	}

	var children []string
	for _, manifest := range index.Manifests {
		children = append(children, manifest.Digest.String())
	}
	return children, nil
}

func (d *OrasChildDiscoverer) discoverFromCosignTags(ctx context.Context, repo *remote.Repository, parentDigest string, allTags []string) []string {
	prefix := strings.Replace(parentDigest, ":", "-", 1)
	expectedSig := prefix + ".sig"
	expectedAtt := prefix + ".att"

	var children []string
	for _, tag := range allTags {
		if tag == expectedSig || tag == expectedAtt {
			desc, err := repo.Resolve(ctx, tag)
			if err == nil {
				children = append(children, desc.Digest.String())
			}
		}
	}
	return children
}
