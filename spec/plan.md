# ghcrctl Development Plan

## Overview

This plan breaks down the development of `ghcrctl` into small, incremental iterations. Each iteration produces a usable tool, starting from a minimal viable version and progressively adding features. We follow a TDD (Test-Driven Development) approach throughout.

---

## Iteration 1: Project Setup & Configuration Foundation

### Goal
Establish project structure and implement basic configuration management.

### Features
- Go module initialization
- Basic CLI structure using Cobra
- Configuration file support (`~/.ghcrctl/config.yaml`) using Viper
- Commands:
  - `ghcrctl config show` - Display current configuration
  - `ghcrctl config org <org-or-user>` - Set GHCR owner

### Deliverable
A working CLI that can store and retrieve configuration settings.

### Testing Concept
- **Unit Tests**: Test config read/write operations with temporary config files
- **CLI Tests**: Test command execution and output using cobra testing utilities
- **Integration Tests**: Verify config file is created in correct location with correct format
- **Test Cases**:
  - Config file creation when missing
  - Reading existing config
  - Setting and persisting owner value
  - Error handling for invalid config paths

---

## Iteration 2: GitHub Authentication & Basic Connection

### Goal
Establish authenticated connection to GitHub API.

### Features
- GitHub token detection from environment (`GITHUB_TOKEN`)
- Basic GitHub API client initialization using go-github
- Validation of token and permissions
- Error messages for missing/invalid tokens

### Deliverable
Tool can authenticate with GitHub and validate access to GHCR.

### Testing Concept
- **Unit Tests**: Mock GitHub API client, test token loading
- **Integration Tests**: Test with valid/invalid tokens (use test tokens or mocks)
- **Test Cases**:
  - Token loaded from environment
  - Error when token missing
  - Error when token invalid
  - Error when insufficient permissions
  - Connection validation succeeds

---

## Iteration 3: List Container Images

### Goal
Implement the `images` command to list GHCR packages.

### Features
- Commands:
  - `ghcrctl images` - List all container images for configured owner
  - `ghcrctl images --json` - JSON output
- Display images in alphabetical order
- Human-readable table format by default

### Deliverable
Tool can query and display all container images from GHCR.

### Testing Concept
- **Unit Tests**: Mock GitHub API responses, test sorting and formatting
- **Integration Tests**: Test against real/test GHCR account (or mock server)
- **Test Cases**:
  - List empty packages (no images)
  - List multiple images, verify alphabetical order
  - JSON output format validation
  - Error handling when owner not configured
  - Error handling when API call fails

---

## Iteration 4: ORAS Integration & Tag Resolution

### Goal
Integrate ORAS SDK for OCI operations.

### Features
- ORAS client initialization
- Tag to digest resolution
- Basic connectivity to GHCR via OCI distribution API
- Internal module `internal/oras` with:
  - `ResolveTag(image, tag) -> digest`
  - Connection configuration for ghcr.io

### Deliverable
Tool can resolve image tags to digest values using ORAS.

### Testing Concept
- **Unit Tests**: Mock ORAS operations, test digest parsing
- **Integration Tests**: Test tag resolution against real or test registry
- **Test Cases**:
  - Resolve existing tag to digest
  - Error when tag doesn't exist
  - Error when image doesn't exist
  - Handle missing :latest gracefully
  - Validate digest format (sha256:...)

---

## Iteration 5: OCI Graph Discovery

### Goal
Implement full OCI artifact graph resolution.

### Features
- Commands:
  - `ghcrctl graph <image>` - Display OCI graph
  - `ghcrctl graph <image> --tag <tag>` - Specify tag
  - `ghcrctl graph <image> --json` - JSON output
- Discover referrers (SBOM, provenance) using ORAS
- Map digests to GHCR version IDs
- Display relationships between image, SBOM, and provenance
- Internal module `internal/graph` for graph representation

### Deliverable
Tool can discover and display the complete OCI artifact graph for any image.

### Testing Concept
- **Unit Tests**: Mock referrer discovery, test graph building logic
- **Integration Tests**: Test with images that have/don't have SBOM/provenance
- **Test Cases**:
  - Image with complete graph (image + SBOM + provenance)
  - Image without referrers
  - Image with only SBOM or only provenance
  - Multiple tags pointing to same digest
  - Untagged versions
  - JSON output format validation
  - Error handling for resolution failures

---

## Iteration 6: Tagging Functionality

### Goal
Implement the `tag` command to add tags to existing versions.

### Features
- Commands:
  - `ghcrctl tag <image> <existing-tag> <new-tag>` - Add new tag
- Workflow:
  1. Resolve existing tag to digest using ORAS
  2. Find GHCR version ID for that digest
  3. PATCH metadata to add new tag
- Validation and error handling

### Deliverable
Tool can add new tags to existing GHCR versions.

### Testing Concept
- **Unit Tests**: Mock GitHub API PATCH operations
- **Integration Tests**: Add tags to test images, verify via GitHub API
- **Test Cases**:
  - Successfully add new tag to existing version
  - Error when existing tag doesn't exist
  - Error when new tag already exists
  - Error when insufficient permissions
  - Verify tag appears in subsequent listings

---

## Iteration 7: Labeling Functionality

### Goal
Implement the `label` command to apply labels to versions and their artifacts.

### Features
- Commands:
  - `ghcrctl label <image> <tag> key=value` - Apply label to graph
  - `ghcrctl label <image> <tag> key=value --json` - JSON output
- Apply label to:
  - Image version
  - SBOM artifact
  - Provenance artifact
- Support multiple key=value pairs
- Output summary of all patches applied

### Deliverable
Tool can label entire OCI graphs (image + SBOM + provenance) atomically.

### Testing Concept
- **Unit Tests**: Mock multiple PATCH operations, test label parsing
- **Integration Tests**: Apply labels and verify via GitHub API
- **Test Cases**:
  - Label complete graph (3 versions)
  - Label when SBOM/provenance missing
  - Parse multiple key=value pairs
  - Error on invalid label syntax
  - Verify all versions receive label
  - JSON output includes all operations

---

## Iteration 8: Basic Deletion with Safety

### Goal
Implement safe deletion of single versions.

### Features
- Commands:
  - `ghcrctl delete <image> <version-id>` - Delete single version
  - `ghcrctl delete <image> <version-id> --force` - Skip confirmation
- Interactive confirmation prompt: "Are you sure? [y/N]:"
- Internal module `internal/prompts` for user input
- Clear output of what will be deleted

### Deliverable
Tool can safely delete individual GHCR versions with user confirmation.

### Testing Concept
- **Unit Tests**: Mock deletion API, test prompt logic with simulated input
- **Integration Tests**: Delete test versions, verify removal
- **Test Cases**:
  - Delete with confirmation (y)
  - Cancel deletion (n or empty)
  - Delete with --force flag
  - Error when version doesn't exist
  - Error when insufficient permissions
  - Verify version is actually deleted

---

## Iteration 9: Advanced Deletion Operations

### Goal
Implement bulk deletion and graph deletion features.

### Features
- Commands:
  - `ghcrctl delete <image> --untagged` - Delete untagged versions
  - `ghcrctl delete <image> --older-than <days>` - Delete old versions
  - `ghcrctl delete graph <image> <tag>` - Delete entire graph
- Safety features:
  - Dry-run mode (`--dry-run`)
  - Summary of what will be deleted
  - Confirmation prompts (unless --force)
- Prevent orphaning artifacts (warn if deleting SBOM/provenance separately)

### Deliverable
Tool can perform safe bulk deletions and manage complete OCI graphs.

### Testing Concept
- **Unit Tests**: Test date filtering logic, test dry-run mode
- **Integration Tests**: Perform bulk deletions on test data
- **Test Cases**:
  - Delete all untagged versions
  - Delete versions older than N days
  - Delete complete graph (image + SBOM + provenance)
  - Dry-run shows changes without executing
  - Verify correct versions deleted in bulk operations
  - Graph deletion removes all related artifacts
  - Error when trying to delete only SBOM/provenance

---

## Iteration 10: Interactive Mode & Polish

### Goal
Add interactive features and finalize the tool.

### Features
- Interactive image selection for `ghcrctl images --interactive`
- Enhanced error messages with actionable guidance
- Completion of error handling requirements from spec
- Documentation:
  - README with installation and usage examples
  - Command help text refinement
- Performance optimizations:
  - Caching for repeated API calls
  - Parallel processing where applicable

### Deliverable
Production-ready CLI tool with excellent UX and complete feature set.

### Testing Concept
- **Unit Tests**: Test interactive prompt logic with simulated input
- **Integration Tests**: End-to-end scenarios covering full workflows
- **Test Cases**:
  - Interactive mode selection flow
  - All error conditions produce helpful messages
  - Performance benchmarks for list/graph operations
  - Complete workflow tests:
    - Configure → list → graph → label → tag
    - Configure → list → delete workflow
  - Regression tests for all previous iterations

---

## Testing Infrastructure

### Tools & Frameworks
- **Testing Framework**: Go standard `testing` package
- **Mocking**: `github.com/stretchr/testify/mock` for API mocks
- **CLI Testing**: Cobra's built-in testing support
- **Integration Tests**: Separate test suite with `//go:build integration` tag
- **Coverage**: Aim for >80% code coverage

### Test Organization
```
ghcrctl/
├── internal/
│   ├── config/
│   │   ├── config.go
│   │   └── config_test.go
│   ├── gh/
│   │   ├── client.go
│   │   ├── client_test.go
│   │   └── mock_client.go
│   ├── oras/
│   │   ├── resolver.go
│   │   └── resolver_test.go
│   └── graph/
│       ├── graph.go
│       └── graph_test.go
├── cmd/
│   ├── root_test.go
│   ├── config_test.go
│   └── images_test.go
└── test/
    ├── integration/
    │   ├── config_test.go
    │   ├── images_test.go
    │   └── graph_test.go
    └── fixtures/
        └── test_data.json
```

### CI/CD Integration
- Run unit tests on every commit
- Run integration tests on PRs (with test GHCR account or mocks)
- Lint with `golangci-lint`
- Check code coverage and fail if below threshold
- Build for multiple platforms (Linux, macOS, Windows)

---

## Development Workflow

### TDD Cycle for Each Iteration
1. **Write Tests First**: Define expected behavior through tests
2. **Implement Feature**: Write minimal code to pass tests
3. **Refactor**: Clean up code while keeping tests green
4. **Manual Testing**: Quick smoke test of the actual CLI
5. **Documentation**: Update command help and README

### Branch Strategy
- `main` branch: stable, working code
- `iteration-N` branches: development for each iteration
- Merge to main only when iteration is complete and tested

### Review Checklist
- [ ] All tests pass
- [ ] Code coverage maintained/improved
- [ ] Error handling tested
- [ ] Help text updated
- [ ] README reflects new features
- [ ] Manual testing completed

---

## Dependencies

### Core Libraries
- `github.com/spf13/cobra` - CLI framework
- `github.com/spf13/viper` - Configuration management
- `github.com/google/go-github/v58` - GitHub API client
- `oras.land/oras-go/v2` - OCI registry client

### Testing Libraries
- `github.com/stretchr/testify` - Assertions and mocks
- `github.com/golang/mock` - Mock generation (if needed)

### Development Tools
- `golangci-lint` - Linting
- `goreleaser` - Release automation

---

## Success Criteria

Each iteration is considered complete when:
1. All unit tests pass
2. All integration tests pass (where applicable)
3. Manual testing confirms expected behavior
4. Code coverage is ≥80%
5. Documentation is updated
6. Code review completed (if team-based)
7. Tool can be used for its intended purpose at that iteration level

The final tool (after Iteration 10) should meet all requirements from the specification document.
