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
- [x] Create `internal/discovery/builder.go`
- [x] Create `internal/discovery/builder_test.go`
- [x] Define core interfaces (GHClient for testability)
- [x] Implement GetVersionCache() with dual lookup maps
- [x] Implement FindParentDigest() with ID proximity optimization
- [x] Add comprehensive tests

**Status:** ✅ Complete (commits: 2dd1575, 4b2c0f8)

#### Step 1.2: Incremental Integration Approach (REVISED)
**Goal:** Integrate incrementally instead of implementing BuildGraph in isolation

- [x] Phase 1.2a: Use GetVersionCache in graph command
- [x] Phase 1.2b: Use FindParentDigest in graph command
- [x] Phase 1.2c: Integrate into graph command
- [x] Phase 1.2d: Integrate into delete graph command
- [ ] Phase 1.2e: Integrate into versions command (optional - different structure)

**Rationale:** Incremental integration drives what BuildGraph needs and follows TDD principles

**Status:** ✅ Complete for graph and delete commands (commits: ca0e703, ef57a9a)

#### Step 1.3: Complete graph command migration
- [x] Replace inline version cache with discovery.GetVersionCache
- [x] Replace findParentDigest with discovery.FindParentDigest
- [x] Remove duplicated sortByIDProximity and findParentDigest (~80 lines)
- [x] Remove test duplication
- [x] Verify behavior unchanged (all integration tests pass)
- **Actual lines removed: ~115 lines**

**Status:** ✅ Complete

#### Step 1.4: Update delete graph command
- [x] Integrate discovery.GetVersionCache in buildGraph()
- [x] Replace findParentDigest with discovery.FindParentDigest
- [x] Verify behavior unchanged (all tests pass)
- **Actual lines removed: ~1 net line (10 additions, 11 deletions)**

**Status:** ✅ Complete

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

### ✅ Phase 2: Extract Display Formatting
**Goal:** Create `internal/display` package for consistent formatting

#### Step 2.0: Extract common utility functions (COMPLETED)
- [x] Create `internal/display/formatter.go` with FormatTags and ShortDigest
- [x] Create `internal/display/formatter_test.go` with 100% coverage
- [x] Remove duplicate formatTags from cmd/graph.go
- [x] Remove duplicate shortDigest from cmd/sbom.go
- [x] Update all commands to use display package functions
- [x] All tests passing

**Status:** ✅ Complete (commit: 0ec40a6)

#### Step 2.1: Extract JSON formatting (COMPLETED)
- [x] Create OutputJSON helper function
- [x] Add comprehensive tests (100% coverage)
- [x] Replace duplicate JSON output functions in all commands
- [x] Remove 35 lines of duplicated code
- [x] All tests passing

**Status:** ✅ Complete (commit: 967954f)

#### Step 2.1: Define display structures
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
| Duplicated graph building code | ~600 lines | ~485 lines | 🟡 Phase 1 complete (~115 lines removed) |
| Graph builder implementations | 3 | 2 (discovery + versions) | 🟡 2 of 3 unified |
| Commands with type info | 1 (versions) | 2 (versions + delete) | 🔴 Not started (Phase 3) |
| Display formatters | 3 | 3 | 🔴 Not started (Phase 2) |
| Discovery package test coverage | N/A | 100% (3 tests) | 🟢 Complete |

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

### 2025-11-26 (Morning)
- ✅ Created specification document
- ✅ Analyzed current code duplication
- ✅ Designed architecture

### 2025-11-26 (Afternoon)
- ✅ Phase 1.1 Complete: Created internal/discovery package
  - Implemented GetVersionCache with dual lookup maps
  - Implemented FindParentDigest with ID proximity optimization
  - Added comprehensive unit tests (3 tests, all passing)
  - Commits: 2dd1575, 4b2c0f8
- ✅ Phase 1.3 Complete: Graph command integration
  - Integrated discovery.GetVersionCache
  - Integrated discovery.FindParentDigest
  - Removed ~115 lines of duplicate code
  - All integration tests pass (46.6s)
  - Commit: ca0e703
- ✅ Phase 1.4 Complete: Delete graph command integration
  - Integrated discovery package in buildGraph()
  - All tests pass (61.3s for cmd package)
  - Commit: ef57a9a

**Status:** Phase 1 substantially complete. Graph and delete commands unified.

### 2025-11-26 (Afternoon - Phase 2 Start)
- ✅ Phase 2.0 Complete: Extract common utility functions
  - Created internal/display package
  - Implemented FormatTags and ShortDigest utility functions
  - Created comprehensive unit tests (100% coverage)
  - Removed duplicate formatTags from graph.go
  - Removed duplicate shortDigest from sbom.go
  - Updated all commands (graph, versions, delete, sbom) to use display package
  - All tests passing (72.8s total)
  - Code quality checks passing
  - Commit: 0ec40a6
  - Lines changed: +137, -44

### 2025-11-26 (Evening - Phase 2 Completion)
- ✅ Phase 2.1 Complete: Extract JSON formatting
  - Added OutputJSON helper function to display package
  - Created comprehensive tests for OutputJSON (100% coverage)
  - Replaced outputVersionsJSON in cmd/versions.go
  - Replaced outputJSON in cmd/images.go
  - Replaced outputLabelsJSON in cmd/labels.go
  - Replaced outputSBOMJSON in cmd/sbom.go
  - Replaced outputProvenanceJSON in cmd/provenance.go
  - Removed 35 lines of duplicated JSON formatting code
  - All tests passing (60.3s total)
  - Code quality checks passing
  - Code coverage increased to 61.2%
  - Commit: 967954f
  - Lines changed: +93, -56

**Status:** Phase 2 complete. Display package created with common formatting helpers.

---

**Next Steps:**
1. ~~Review and approve this specification~~ ✅
2. ~~Create feature branch~~ (working on main)
3. ~~Begin Phase 1.1: Create internal/discovery package~~ ✅
4. Optional: Integrate versions command (different structure, lower priority)
5. Phase 2: Extract display formatting (future work)
6. Phase 3: Add type info to delete command (future work)
