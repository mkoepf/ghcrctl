package cmd

// This file contains graph-building code that is used by the delete command.
// This will be migrated to use internal/discover in Phase 3 of the consolidation.

import (
	"context"

	"github.com/mkoepf/ghcrctl/internal/discovery"
	"github.com/mkoepf/ghcrctl/internal/gh"
	"github.com/mkoepf/ghcrctl/internal/oras"
)

// VersionGraph is an alias for discovery.VersionGraph for backward compatibility
type VersionGraph = discovery.VersionGraph

// VersionChild is an alias for discovery.VersionChild for backward compatibility
type VersionChild = discovery.VersionChild

func buildVersionGraphs(ctx context.Context, fullImage string, versionsToGraph []gh.PackageVersionInfo, allVersions []gh.PackageVersionInfo, client *gh.Client, owner, ownerType, imageName string) ([]VersionGraph, error) {
	// Create version cache for efficient lookups (use ALL versions for child discovery)
	cache := discovery.NewVersionCacheFromSlice(allVersions)

	// Track which versions are graph roots (to avoid treating them as standalone later)
	isGraphRoot := make(map[int64]bool)

	// Track reference counts for each version ID (how many graphs reference it)
	refCounts := make(map[int64]int)

	// Identify cosign children: versions whose tags indicate they are .sig or .att artifacts
	// Map from parent digest -> list of cosign child versions
	cosignChildrenByParent := make(map[string][]gh.PackageVersionInfo)
	isCosignChild := make(map[int64]bool)
	for _, ver := range allVersions {
		for _, tag := range ver.Tags {
			if parentDigest, ok := oras.ExtractParentDigestFromCosignTag(tag); ok {
				cosignChildrenByParent[parentDigest] = append(cosignChildrenByParent[parentDigest], ver)
				isCosignChild[ver.ID] = true
				break // Only need to check one cosign tag per version
			}
		}
	}

	// Identify orphan attestations: untagged versions that might be attestations
	// Try to detect their parent by extracting the subject digest from their content
	for _, ver := range allVersions {
		// Skip if already identified as a child or if it has tags
		if isCosignChild[ver.ID] || len(ver.Tags) > 0 {
			continue
		}

		// Check if this might be an attestation by trying to extract its subject
		parentDigest, err := oras.ExtractSubjectDigest(ctx, fullImage, ver.Name)
		if err == nil && parentDigest != "" {
			// Found parent - add as a child
			cosignChildrenByParent[parentDigest] = append(cosignChildrenByParent[parentDigest], ver)
			isCosignChild[ver.ID] = true
		}
	}

	var graphs []VersionGraph

	// Helper to build a graph for a version
	buildGraph := func(ver gh.PackageVersionInfo) *VersionGraph {
		relatedArtifacts := discoverRelatedVersionsByDigest(ctx, fullImage, ver.Name, ver.Name)

		// Skip if no children and not tagged (standalone untagged versions are handled separately)
		if len(relatedArtifacts) == 0 && len(ver.Tags) == 0 {
			return nil
		}

		// Use ResolveType for consistent type determination
		graphType := "manifest" // default
		if artType, err := oras.ResolveType(ctx, fullImage, ver.Name); err == nil {
			graphType = artType.DisplayType()
		}

		graph := &VersionGraph{
			RootVersion: ver,
			Children:    []VersionChild{},
			Type:        graphType,
		}

		// Find child versions by digest and consolidate artifact types
		childMap := make(map[int64]*VersionChild)
		for _, artifact := range relatedArtifacts {
			if childVer, found := cache.ByDigest[artifact.Digest]; found {
				if _, exists := childMap[childVer.ID]; exists {
					// Skip duplicates - the first type wins
				} else {
					// Only count reference once per unique child per graph
					refCounts[childVer.ID]++
					childMap[childVer.ID] = &VersionChild{
						Version: childVer,
						Type:    artifact.Type,
						Size:    artifact.Size,
					}
				}
			}
		}

		// Add children to graph
		for _, child := range childMap {
			graph.Children = append(graph.Children, *child)
		}

		// Add cosign children (versions with .sig or .att tags referencing this digest)
		if cosignChildren, found := cosignChildrenByParent[ver.Name]; found {
			for _, cosignVer := range cosignChildren {
				if _, exists := childMap[cosignVer.ID]; exists {
					continue // Already added via ORAS discovery
				}
				// Resolve type for the cosign artifact
				childType := oras.ArtifactType{Role: "attestation"} // default
				if artType, err := oras.ResolveType(ctx, fullImage, cosignVer.Name); err == nil {
					childType = artType
				}
				refCounts[cosignVer.ID]++
				graph.Children = append(graph.Children, VersionChild{
					Version: cosignVer,
					Type:    childType,
					Size:    0, // Size not easily available from version info
				})
			}
		}

		return graph
	}

	// Process tagged versions first (potential graph roots)
	// Skip versions that are cosign children - they'll be added to their parent's graph
	for _, ver := range versionsToGraph {
		if len(ver.Tags) > 0 && !isGraphRoot[ver.ID] && !isCosignChild[ver.ID] {
			isGraphRoot[ver.ID] = true
			if graph := buildGraph(ver); graph != nil {
				graphs = append(graphs, *graph)
			}
		}
	}

	// Process untagged versions as potential graph roots
	for _, ver := range versionsToGraph {
		if len(ver.Tags) == 0 && !isGraphRoot[ver.ID] && !isCosignChild[ver.ID] {
			if graph := buildGraph(ver); graph != nil {
				isGraphRoot[ver.ID] = true
				graphs = append(graphs, *graph)
			}
		}
	}

	// Add remaining unassigned versions as standalone (not a root and not a child of any graph)
	// This includes orphan attestations (untagged versions with attestation types)
	for _, ver := range versionsToGraph {
		if !isGraphRoot[ver.ID] && refCounts[ver.ID] == 0 && !isCosignChild[ver.ID] {
			// Use ResolveType for consistent type determination
			graphType := "manifest" // default
			if artType, err := oras.ResolveType(ctx, fullImage, ver.Name); err == nil {
				graphType = artType.DisplayType()
			}

			graph := VersionGraph{
				RootVersion: ver,
				Children:    []VersionChild{},
				Type:        graphType,
			}
			graphs = append(graphs, graph)
		}
	}

	// Second pass: update RefCount on all children based on total references
	for i := range graphs {
		for j := range graphs[i].Children {
			graphs[i].Children[j].RefCount = refCounts[graphs[i].Children[j].Version.ID]
		}
	}

	return graphs, nil
}

// discoverRelatedVersionsByDigest discovers children using a digest directly
func discoverRelatedVersionsByDigest(ctx context.Context, fullImage, digest, rootDigest string) []oras.ChildArtifact {
	// Use unified discovery
	children, err := oras.DiscoverChildren(ctx, fullImage, digest, nil)
	if err != nil {
		return nil
	}

	// Filter out the root digest
	var filtered []oras.ChildArtifact
	for _, child := range children {
		if child.Digest != rootDigest {
			filtered = append(filtered, child)
		}
	}

	return filtered
}
