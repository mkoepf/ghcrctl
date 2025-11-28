# buildGraph Optimization Plan

## Overview

The `buildGraph` function in `cmd/delete.go` is used by `ghcrctl delete graph` to build an OCI artifact graph with RefCount information. Currently it takes ~4 seconds for a small package due to excessive API calls.

## Current Algorithm

```
buildGraph(targetDigest):
    1. GetVersionCache()                          → ListPackageVersions (GitHub API)
    2. DiscoverChildren(targetDigest)             → 2 + A OCI calls
    3. builder.BuildGraph(targetDigest)           → DiscoverChildren again (mostly cached)
    4. ListPackageVersions()                      → GitHub API [DUPLICATE!]
    5. buildVersionGraphs(ALL versions):
         For EACH version V:
           a. discoverRelatedVersionsByDigest()   → DiscoverChildren → 2 + A OCI calls
           b. ResolveType()                       → 1-3 OCI calls
         For EACH standalone:
           c. ResolveType()                       → 1-3 OCI calls
```

### OCI Call Breakdown

#### `DiscoverChildren(digest)`
| Step | Function | Cached? |
|------|----------|---------|
| 1 | `cachedResolve(digest)` | Yes |
| 2 | `cachedFetchIndex(digest)` | Yes |
| 3 | `determineRolesFromManifest()` per attestation | **No** |

#### `ResolveType(digest)`
| Step | Function | Cached? |
|------|----------|---------|
| 1 | `cachedResolve(digest)` | Yes |
| 2 | `repo.Fetch(manifest)` | **No** |
| 3 | `repo.Fetch(config)` for platforms | **No** |

### Current Cost

For a package with V=20 versions, T=5 tagged, A=2 attestations:

```
GitHub API:  2 calls (duplicate ListPackageVersions)
OCI calls:   ~84 calls
Total time:  ~4 seconds
```

---

## Issues Identified

| # | Issue | Location | Impact |
|---|-------|----------|--------|
| 1 | Duplicate `ListPackageVersions` | `delete.go:764` + `delete.go:788` | 1 extra GitHub API call |
| 2 | `determineRolesFromManifest` not cached | `discovery.go:169` | A uncached fetches per index |
| 3 | `ResolveType` fetches manifest+config | `types.go:100,121` | 2 extra OCI calls per version |
| 4 | Builds ALL graphs for RefCount | `delete.go:798` | O(V) unnecessary graph builds |
| 5 | No parallelism | Throughout | High latency |
| 6 | Checks untagged versions for RefCount | `versions.go:254-260` | Unnecessary iterations |

---

## Optimization Plan

### Phase 1: Quick Wins (Low Risk)

#### 1.1 Remove Duplicate `ListPackageVersions`

**File:** `cmd/delete.go`

**Current:**
```go
// Line 764
cache, err := builder.GetVersionCache()  // Calls ListPackageVersions internally

// Line 788
allVersions, err := ghClient.ListPackageVersions(ctx, owner, ownerType, imageName)  // DUPLICATE!
```

**Solution:**
```go
cache, err := builder.GetVersionCache()
if err != nil {
    return nil, fmt.Errorf("failed to get version cache: %w", err)
}

// Reuse versions from cache instead of fetching again
allVersions := cache.AllVersions()  // Add this method to VersionCache
```

**Impact:** Eliminates 1 GitHub API call per invocation.

---

#### 1.2 Add `AllVersions()` Method to VersionCache

**File:** `internal/discovery/builder.go`

**Add:**
```go
// AllVersions returns all versions in the cache as a slice.
func (c *VersionCache) AllVersions() []gh.PackageVersionInfo {
    versions := make([]gh.PackageVersionInfo, 0, len(c.ByID))
    for _, v := range c.ByID {
        versions = append(versions, v)
    }
    return versions
}
```

---

### Phase 2: Caching Improvements (Medium Risk)

#### 2.1 Cache Attestation Manifest Fetches

**File:** `internal/oras/discovery.go`

**Current:** `determineRolesFromManifest` calls `repo.Fetch(ctx, desc)` without caching.

**Solution:** Add manifest content cache.

```go
var (
    manifestContentCache   map[string]*ocispec.Manifest
    manifestContentCacheMu sync.RWMutex
)

func init() {
    manifestContentCache = make(map[string]*ocispec.Manifest)
}

func cachedFetchManifest(ctx context.Context, repo *remote.Repository, desc ocispec.Descriptor) (*ocispec.Manifest, error) {
    digestStr := desc.Digest.String()

    // Check cache
    manifestContentCacheMu.RLock()
    if manifest, found := manifestContentCache[digestStr]; found {
        manifestContentCacheMu.RUnlock()
        return manifest, nil
    }
    manifestContentCacheMu.RUnlock()

    // Fetch and cache
    manifestBytes, err := repo.Fetch(ctx, desc)
    if err != nil {
        return nil, err
    }
    defer manifestBytes.Close()

    var manifest ocispec.Manifest
    if err := json.NewDecoder(manifestBytes).Decode(&manifest); err != nil {
        return nil, err
    }

    manifestContentCacheMu.Lock()
    manifestContentCache[digestStr] = &manifest
    manifestContentCacheMu.Unlock()

    return &manifest, nil
}
```

**Update `determineRolesFromManifest`:**
```go
func determineRolesFromManifest(ctx context.Context, repo *remote.Repository, desc ocispec.Descriptor) []string {
    manifest, err := cachedFetchManifest(ctx, repo, desc)
    if err != nil {
        return nil
    }
    // ... rest of function using manifest
}
```

**Impact:** Eliminates A duplicate fetches per repeated index access.

---

#### 2.2 Cache Manifest Fetches in `ResolveType`

**File:** `internal/oras/types.go`

**Current:** Lines 100-104 fetch manifest without caching.

**Solution:** Use `cachedFetchManifest` (from 2.1).

```go
// Replace:
manifestBytes, err := repo.Fetch(ctx, desc)

// With:
manifest, err := cachedFetchManifest(ctx, repo, desc)
```

**Impact:** Eliminates duplicate manifest fetches across `ResolveType` calls.

---

### Phase 3: Algorithm Optimization (Medium-High Risk)

#### 3.1 Skip `ResolveType` in RefCount Calculation

**File:** `cmd/versions.go`

**Current:** `buildVersionGraphs` calls `ResolveType` for every version to determine graph type.

**Insight:** For RefCount calculation, we don't need graph types. We only need to know parent-child relationships.

**Solution:** Infer type from children instead of fetching.

```go
// In buildGraph closure (line 197-241)
func buildGraph(ver gh.PackageVersionInfo) *VersionGraph {
    relatedArtifacts := discoverRelatedVersionsByDigest(ctx, fullImage, ver.Name, ver.Name)

    if len(relatedArtifacts) == 0 && len(ver.Tags) == 0 {
        return nil
    }

    // REMOVE: ResolveType call
    // graphType := "manifest"
    // if artType, err := oras.ResolveType(ctx, fullImage, ver.Name); err == nil {
    //     graphType = artType.DisplayType()
    // }

    // REPLACE WITH: Infer from children
    graphType := inferGraphType(relatedArtifacts)

    // ... rest of function
}

func inferGraphType(children []oras.ChildArtifact) string {
    for _, c := range children {
        if c.Type.IsPlatform() {
            return "index"
        }
    }
    if len(children) > 0 {
        return "manifest"  // Has attestations but no platforms
    }
    return "standalone"
}
```

**Impact:** Eliminates 1-3 OCI calls per version.

---

#### 3.2 ~~Only Check Tagged Versions for RefCount~~ (INCORRECT - DO NOT IMPLEMENT)

**Status:** This optimization was found to be incorrect during implementation.

**Insight (WRONG):** The original assumption was that untagged orphan versions don't create meaningful dependencies.

**Reality:** Untagged versions CAN share children with each other. For example, two untagged index manifests can share the same platform manifest. When deleting one, we must preserve the shared child for the other.

**Correct Solution:** Check ALL versions (excluding target root and its children) for potential sharing. Use parallelism to maintain performance.

---

#### 3.3 Targeted RefCount Calculation

**File:** `cmd/delete.go`

**Current:** Builds full graphs for ALL versions, then extracts RefCount.

**Better:** Only count references to the target graph's children.

```go
func calculateRefCountsDirect(ctx context.Context, fullImage string,
    targetChildren []discovery.VersionChild,
    allVersions []gh.PackageVersionInfo,
    targetRootDigest string) {

    // Build set of child digests we care about
    childDigests := make(map[string]*discovery.VersionChild)
    for i := range targetChildren {
        childDigests[targetChildren[i].Version.Name] = &targetChildren[i]
        targetChildren[i].RefCount = 1  // Start with self-reference
    }

    // Only check tagged versions (they're the ones that matter for "breaking" refs)
    for _, ver := range allVersions {
        if len(ver.Tags) == 0 || ver.Name == targetRootDigest {
            continue
        }

        // Discover this version's children
        children, _ := oras.DiscoverChildren(ctx, fullImage, ver.Name, nil)

        // Check if any match our target's children
        for _, c := range children {
            if child, found := childDigests[c.Digest]; found {
                child.RefCount++
            }
        }
    }
}
```

**Impact:**
- Skips `ResolveType` entirely
- Only iterates tagged versions
- Only discovers children, doesn't build full graph structures

---

### Phase 4: Parallelism (Higher Risk)

#### 4.1 Parallel Child Discovery

**File:** `cmd/delete.go` or new `internal/discovery/parallel.go`

```go
func discoverChildrenParallel(ctx context.Context, fullImage string, digests []string) map[string][]oras.ChildArtifact {
    results := make(map[string][]oras.ChildArtifact)
    var mu sync.Mutex
    var wg sync.WaitGroup

    // Semaphore to limit concurrent requests
    sem := make(chan struct{}, 10)

    for _, digest := range digests {
        wg.Add(1)
        go func(d string) {
            defer wg.Done()
            sem <- struct{}{}        // Acquire
            defer func() { <-sem }() // Release

            children, err := oras.DiscoverChildren(ctx, fullImage, d, nil)
            if err == nil {
                mu.Lock()
                results[d] = children
                mu.Unlock()
            }
        }(digest)
    }

    wg.Wait()
    return results
}
```

**Impact:** Reduces wall-clock time by ~70% for network-bound operations.

---

## Implementation Order

| Phase | Task | Risk | Effort | Impact |
|-------|------|------|--------|--------|
| 1.1 | Remove duplicate ListPackageVersions | Low | 15 min | -1 API call |
| 1.2 | Add AllVersions() method | Low | 10 min | Enables 1.1 |
| 2.1 | Cache attestation manifest fetches | Medium | 30 min | -A calls/index |
| 2.2 | Cache ResolveType manifest fetches | Medium | 20 min | -2 calls/version |
| 3.1 | Skip ResolveType for RefCount | Medium | 30 min | -3 calls/version |
| ~~3.2~~ | ~~Only check tagged versions~~ | N/A | N/A | INCORRECT - breaks shared manifest detection |
| 3.3 | Targeted RefCount calculation | Medium-High | 1 hour | Major restructure |
| 4.1 | Add parallelism | High | 1 hour | -70% latency |

---

## Projected Results

| Metric | Current | After Phase 1-2 | After Phase 3 | After Phase 4 |
|--------|---------|-----------------|---------------|---------------|
| GitHub API calls | 2 | 1 | 1 | 1 |
| OCI calls (20 ver, 5 tagged) | ~84 | ~60 | ~15 | ~15 |
| Wall-clock time | ~4s | ~3s | ~1.5s | ~0.5s |

---

## Testing Strategy

1. **Unit tests:** Mock OCI client to verify call counts
2. **Integration tests:** Measure actual timing improvements
3. **Regression tests:** Ensure RefCount values remain correct
4. **Edge cases:**
   - Package with no tagged versions
   - Package with only one version
   - Deeply nested attestations
   - Shared platform manifests across many tags

---

## Implementation Notes (2025-11-28)

### What Was Implemented

All phases were implemented except 3.2 which was found to be incorrect:

1. **Phase 1.1-1.2**: Removed duplicate `ListPackageVersions` by adding `AllVersions()` to `VersionCache`
2. **Phase 2.1-2.2**: Added `cachedFetchManifest()` and updated `determineRolesFromManifest` and `ResolveType` to use it
3. **Phase 3.1 & 3.3**: Created `calculateRefCountsDirect()` in `cmd/delete.go` that:
   - Skips ResolveType entirely (infers type from children)
   - Uses targeted RefCount calculation instead of building full graphs
   - Checks ALL versions (not just tagged) to correctly detect shared children
4. **Phase 4.1**: Added parallel child discovery with bounded concurrency (10 concurrent requests)

### Key Correction

**Phase 3.2 was incorrect.** The assumption that "untagged orphan versions don't create meaningful dependencies" is wrong. Two untagged index manifests CAN share the same platform manifest. When deleting one graph, we must detect this sharing to preserve the child for the other graph.

### Actual Results

| Test | Before | After | Improvement |
|------|--------|-------|-------------|
| `TestBuildGraphMultiarchWithSBOMAndProvenance` | 11.55s | 1.17s | **90%** |
| `TestFindParentDigestIntegration_RootHasNoParent` | 8.78s | 2.41s | **73%** |
| `TestBuildGraphNoSBOMImage` | 7.99s | 0.92s | **88%** |
