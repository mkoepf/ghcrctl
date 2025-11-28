# Release Roadmap

This document outlines what needs to be done before releasing ghcrctl v1.0 to the public.

## Project State Summary

**ghcrctl** is a well-structured CLI tool for managing GitHub Container Registry. The core functionality is complete and working:

- Image listing and version exploration
- SBOM and provenance attestation viewing
- Tagging and labeling operations
- Safe deletion with confirmation prompts and dry-run mode

### Current Strengths

- Comprehensive test coverage with unit and integration tests
- CI/CD pipeline (lint, test, build on Linux/macOS/Windows)
- MIT License
- Detailed README documentation
- Shell completion built-in (via Cobra)
- Cross-platform support (Go 1.23)

---

## MUST-HAVE (Before Public Release)

### 1. ~~Add `--version` flag~~ ✅ DONE

Version flag implemented in `cmd/root.go`. Build with ldflags:
```bash
go build -ldflags "-X github.com/mkoepf/ghcrctl/cmd.Version=v1.0.0"
```

---

### 2. ~~Fix module path inconsistency~~ ✅ DONE

Module path unified to `github.com/mkoepf/ghcrctl` across all files.

---

### 3. Add `go install` instructions

**Issue:** README only shows "from source" build instructions.

**Impact:** Standard Go installation method missing - friction for users.

**Solution:** Add to README:
```bash
go install github.com/<owner>/ghcrctl@latest
```

---

### 4. Remove or implement `BuildGraph` stub

**Issue:** `internal/discovery/builder.go:64-66` contains a stub that returns `nil`.

```go
func (b *GraphBuilder) BuildGraph(digest string) (*graph.Graph, error) {
    // TODO: Implementation will be added incrementally
    return nil, nil
}
```

**Impact:** Dead code / appears incomplete.

**Solution:** This method is never called anywhere in the codebase. Either:
- Remove it entirely (recommended)
- Implement it if there's a planned use case

---

### 5. Release workflow

**Issue:** No automated release process.

**Impact:** No way to publish versioned releases with binaries.

**Solution:** Add goreleaser configuration:
- Create `.goreleaser.yaml`
- Add release workflow to `.github/workflows/release.yml`
- Trigger on version tags (`v*`)

---

### 6. Verify README examples work

**Issue:** Some examples may reference inconsistent paths or owners.

**Impact:** Confusing for new users trying to follow documentation.

**Solution:** Review all code examples in README and verify they work.

---

## NICE-TO-HAVE (Post-Release)

| Item | Description | Priority |
|------|-------------|----------|
| **CONTRIBUTING.md** | Guide for external contributors | Medium |
| **CHANGELOG.md** | Track changes between versions | Medium |
| **Pre-built binaries** | GitHub Releases with downloadable binaries | High |
| **Homebrew formula** | `brew install ghcrctl` | Medium |
| **Docker image** | Run without local Go installation | Low |
| **Interactive mode** | Iteration 10 in spec/plan.md | Low |
| **golangci-lint in CI** | `.golangci.yml` exists but not used in CI | Low |

---

## Code TODOs

These TODOs exist in the codebase:

| File | Line | Note |
|------|------|------|
| `internal/discovery/builder.go` | 65 | Stub implementation (remove) |
| `internal/discovery/builder.go` | 124 | Testing improvement (minor) |

---

## Checklist

### Pre-Release

- [x] Add `--version` flag with build-time injection
- [x] Unify repository/module path
- [ ] Update README with `go install` instructions
- [ ] Remove unused `BuildGraph` stub
- [ ] Add goreleaser configuration
- [ ] Add release workflow
- [ ] Verify all README examples work
- [ ] Tag v1.0.0

### Post-Release

- [ ] Create CONTRIBUTING.md
- [ ] Create CHANGELOG.md
- [ ] Set up Homebrew tap
- [ ] Consider Docker image distribution
