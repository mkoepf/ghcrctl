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

### 3. ~~Add `go install` instructions~~ ✅ DONE

README includes `go install github.com/mkoepf/ghcrctl@latest` in the Installation section.

---

### 4. ~~Remove or implement `BuildGraph` stub~~ ✅ DONE

The `internal/discovery/builder.go` file has been removed entirely.

---

### 5. ~~Release workflow~~ ✅ DONE

Added goreleaser configuration and GitHub Actions release workflow:
- `.goreleaser.yaml` - builds binaries for Linux/macOS/Windows (amd64/arm64)
- `.github/workflows/release.yml` - triggers on version tags (`v*`)

---

### 6. ~~Verify README examples work~~ ✅ DONE

Verified all README examples. Fixed bug in example message output (`ghcrctl sbom` → `ghcrctl get sbom`). Added practical examples for CI/CD cleanup, auditing, and attestation verification.

---

## NICE-TO-HAVE (Post-Release)

| Item | Description | Priority |
|------|-------------|----------|
| **CONTRIBUTING.md** | Guide for external contributors | Medium |
| **CHANGELOG.md** | Track changes between versions | Medium |
| ~~**Pre-built binaries**~~ | ✅ Handled by goreleaser | ~~High~~ |
| **Homebrew formula** | `brew install ghcrctl` | Medium |
| **Docker image** | Run without local Go installation | Low |
| **Interactive mode** | Iteration 10 in spec/plan.md | Low |
| **golangci-lint in CI** | `.golangci.yml` exists but not used in CI | Low |

---

## Code TODOs

✅ No TODOs remain in the codebase.

---

## Checklist

### Pre-Release

- [x] Add `--version` flag with build-time injection
- [x] Unify repository/module path
- [x] Update README with `go install` instructions
- [x] Remove unused `BuildGraph` stub
- [x] Add goreleaser configuration
- [x] Add release workflow
- [x] Verify all README examples work
- [ ] Tag v1.0.0

### Post-Release

- [ ] Create CONTRIBUTING.md
- [ ] Create CHANGELOG.md
- [ ] Set up Homebrew tap
- [ ] Consider Docker image distribution
