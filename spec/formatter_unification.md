# Graph Builder and Display Formatter Unification

**Status:** 🔴 Not Started
**Created:** 2025-11-26
**Last Updated:** 2025-11-26

## Problem Statement

The `graph`, `versions`, and `delete` commands all build OCI artifact graphs and display version information, but they duplicate this logic across three different implementations. This leads to:

1. **Code duplication:** ~600 lines of duplicated graph building logic
2. **Inconsistent behavior:** Same filters show different information
3. **Maintenance burden:** Bug fixes must be applied in multiple places
4. **Poor UX:** Delete command lacks type information that versions command shows

## Current State Analysis

### Command Data Flow Comparison

#### `graph` command (single artifact focus)
```
1. Resolve digest (by tag/version/digest)
2. ListPackageVersions() → cache all versions
3. Create graph.Graph object (inline, lines 157-360)
4. GetPlatformManifests(digest) → discover children
5. DiscoverReferrers(digest) → discover attestations
6. If no children: findParentDigest() → search for parent
7. For each platform: DiscoverReferrers(platform)
8. Map digests to version IDs via cache
9. Display complete graph structure
```

#### `versions` command (filtered list focus)
```
1. ListPackageVersions() → get all versions
2. Apply filters → get filtered subset
3. For EACH filtered version:
   - discoverRelatedVersionsByDigest()
   - GetPlatformManifests(digest)
   - DiscoverReferrers(digest)
   - Determine graphType (index/manifest/standalone)
4. Build VersionGraph objects
5. Display table with TYPE column
```

#### `delete version` command (minimal info)
```
1. ListPackageVersions() → get all versions
2. Apply filters → get filtered subset
3. Display: ID, Tags, Created
   ❌ NO graph building, NO type discovery
```

#### `delete graph` command
```
1. Resolve digest
2. buildGraph() function (lines 598-694)
   - Nearly identical to graph command!
   - ListPackageVersions() → cache
   - GetPlatformManifests()
   - DiscoverReferrers()
   - Find parent if needed
3. Collect version IDs for deletion
```

### Duplicated Code Locations

#### 1. Graph Building Logic (3 implementations!)

| Location | Lines | Notes |
|----------|-------|-------|
| `graph.go` inline | 157-360 | Inline in RunE function |
| `delete.go:buildGraph()` | 598-694 | Nearly identical copy |
| `versions.go:buildVersionGraphs()` | 171-280 | Different struct but same logic |

#### 2. Version Cache Pattern (3 copies!)
```go
// Duplicated in: graph.go:179, delete.go:612, versions.go:173
versionCache := make(map[string]gh.PackageVersionInfo)
for _, ver := range allVersions {
    versionCache[ver.Name] = ver
}
```

#### 3. Parent Finding Logic (2 copies!)
```go
// Duplicated in: graph.go:224-252, delete.go:630-639
if len(platformInfos) == 0 && len(initialReferrers) == 0 {
    parentDigest, err := findParentDigest(...)
    // Recreate graph with parent
}
```

## Proposed Solution

### Architecture Overview

```
┌─────────────────────────────────────────────────────┐
│                  Commands Layer                      │
│  (graph.go, versions.go, delete.go)                 │
└─────────────────┬───────────────────────────────────┘
                  │
                  ├─► Uses
                  │
┌─────────────────▼───────────────────────────────────┐
│          internal/discovery Package                  │
│  • GraphBuilder: Unified graph building              │
│  • VersionCache: Efficient version lookups           │
│  • Parent finding logic                              │
└─────────────────┬───────────────────────────────────┘
                  │
                  ├─► Uses
                  │
┌─────────────────▼───────────────────────────────────┐
│          internal/display Package                    │
│  • VersionDisplay: Common data structure             │
│  • DisplayOptions: Configurable formatting           │
│  • FormatVersions: Table/JSON output                 │
│  • FormatGraph: Graph tree output                    │
└──────────────────────────────────────────────────────┘
```

## Implementation Plan

### ✅ Phase 0: Preparation
- [x] Create spec/formatter_unification.md
- [ ] Review and approve plan
- [ ] Create feature branch

### 🔴 Phase 1: Extract Graph Builder
**Goal:** Create `internal/discovery` package with unified graph building logic

#### Step 1.1: Create internal/discovery package
- [ ] Create `internal/discovery/builder.go`
- [ ] Create `internal/discovery/builder_test.go`
- [ ] Define core interfaces:
  ```go
  type GraphBuilder struct {
      ctx        context.Context
      ghClient   *gh.Client
      fullImage  string
      owner      string
      ownerType  string
      imageName  string
  }

  type VersionCache struct {
      byDigest map[string]gh.PackageVersionInfo
      byID     map[int64]gh.PackageVersionInfo
  }

  func NewGraphBuilder(...) *GraphBuilder
  func (b *GraphBuilder) BuildGraph(digest string) (*graph.Graph, error)
  func (b *GraphBuilder) GetVersionCache() (*VersionCache, error)
  func (b *GraphBuilder) FindParentDigest(digest string) (string, error)
  ```

#### Step 1.2: Implement GraphBuilder
- [ ] Extract graph building logic from `graph.go`
- [ ] Write tests for GraphBuilder
- [ ] Ensure all tests pass
- [ ] Performance: Should be equivalent to current implementation

#### Step 1.3: Update graph command
- [ ] Replace inline graph building with `GraphBuilder`
- [ ] Remove duplicated code
- [ ] Verify behavior unchanged (run integration tests)
- [ ] Estimated lines removed: ~200

#### Step 1.4: Update delete graph command
- [ ] Replace `buildGraph()` with `discovery.GraphBuilder`
- [ ] Remove duplicate function
- [ ] Verify behavior unchanged
- [ ] Estimated lines removed: ~100

#### Step 1.5: Update versions command
- [ ] Use `discovery.GraphBuilder` instead of `buildVersionGraphs`
- [ ] Keep output format unchanged initially
- [ ] Verify behavior unchanged
- [ ] Estimated lines removed: ~100

**Success Criteria:**
- All tests pass
- No behavior changes visible to users
- ~400 lines of duplicated code removed
- Single source of truth for graph building

### 🔴 Phase 2: Extract Display Formatting
**Goal:** Create `internal/display` package for consistent formatting

#### Step 2.1: Create internal/display package
- [ ] Create `internal/display/formatter.go`
- [ ] Create `internal/display/formatter_test.go`
- [ ] Define core structures:
  ```go
  type VersionDisplay struct {
      ID       int64
      Tags     []string
      Digest   string
      Type     string
      Created  string
      Graph    *graph.Graph // optional
  }

  type DisplayOptions struct {
      ShowType    bool
      ShowGraph   bool
      ShowDigest  bool
      Format      string // "table" or "json"
      MaxRows     int
  }

  func FormatVersions(versions []VersionDisplay, opts DisplayOptions) (string, error)
  func FormatGraph(g *graph.Graph, opts DisplayOptions) (string, error)
  ```

#### Step 2.2: Extract formatting from versions command
- [ ] Move table formatting logic to display package
- [ ] Add tests for formatter
- [ ] Update versions command to use formatter
- [ ] Verify output unchanged

#### Step 2.3: Extract formatting from graph command
- [ ] Move graph tree formatting to display package
- [ ] Add tests
- [ ] Update graph command to use formatter
- [ ] Verify output unchanged

**Success Criteria:**
- Consistent formatting across commands
- Reusable display logic
- All tests pass

### 🔴 Phase 3: Enhance delete Command
**Goal:** Add type information to delete command

#### Step 3.1: Add --show-type flag
- [ ] Add `--show-type` flag to delete version command
- [ ] When enabled: use GraphBuilder to get types
- [ ] Format output using display package
- [ ] Add tests

#### Step 3.2: Smart defaults
- [ ] Auto-enable type display for small result sets (<= 10 versions)
- [ ] Keep fast for bulk operations
- [ ] Add configuration option

**Success Criteria:**
- Delete command can optionally show type information
- Performance acceptable for bulk operations
- Consistent with versions command display

### 🔴 Phase 4: Documentation and Cleanup
- [ ] Update command help text
- [ ] Update README with new flags
- [ ] Remove obsolete code
- [ ] Run full test suite
- [ ] Update OBSERVATIONS.md

## Testing Strategy

### Unit Tests
- [ ] GraphBuilder tests (various graph types)
- [ ] VersionCache tests (lookup performance)
- [ ] Formatter tests (all output formats)
- [ ] Parent finding tests

### Integration Tests
- [ ] graph command output unchanged
- [ ] versions command output unchanged
- [ ] delete graph command works correctly
- [ ] delete version with --show-type

### Performance Tests
- [ ] Graph building performance (should match current)
- [ ] Bulk delete performance (should remain fast)
- [ ] Cache effectiveness

## Rollout Plan

1. **Phase 1** (graph builder): Can be merged independently
2. **Phase 2** (display): Builds on Phase 1
3. **Phase 3** (delete enhancement): Optional feature, low risk
4. **Phase 4** (docs): Final polish

## Success Metrics

| Metric | Before | After | Status |
|--------|--------|-------|--------|
| Duplicated graph building code | ~600 lines | ~200 lines | 🔴 Not started |
| Graph builder implementations | 3 | 1 | 🔴 Not started |
| Commands with type info | 1 (versions) | 2 (versions + delete) | 🔴 Not started |
| Display formatters | 3 | 1 shared | 🔴 Not started |
| Test coverage | Unknown | >80% | 🔴 Not started |

## Risks and Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| Behavior changes | High | Extensive testing, no changes to public APIs in Phase 1 |
| Performance regression | Medium | Benchmark tests, caching strategy |
| Complex refactoring | Medium | Incremental approach, phase by phase |
| Breaking changes | Low | Phase 1 has zero user-facing changes |

## Open Questions

1. Should delete command show types by default for all operations?
   - **Decision pending:** Start with opt-in via --show-type flag

2. Should we cache graph building results across commands?
   - **Decision pending:** Implement in Phase 1 if needed for performance

3. Should display package support additional formats (CSV, YAML)?
   - **Decision pending:** Out of scope for initial implementation

## Progress Log

### 2025-11-26
- ✅ Created specification document
- ✅ Analyzed current code duplication
- ✅ Designed architecture
- 🔴 Phase 1 not started

---

**Next Steps:**
1. Review and approve this specification
2. Create feature branch
3. Begin Phase 1.1: Create internal/discovery package
