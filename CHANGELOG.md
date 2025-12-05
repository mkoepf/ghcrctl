# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0] - 2025-12-05

### Added

**Commands**
- `list packages <owner>` - List container packages with JSON output support
- `list versions <owner/image>` - List package versions with filtering by tag, digest, version ID, date ranges, and regex patterns
- `list graphs <owner/image>` - Display OCI artifact graphs in tree format showing platforms, attestations, and relationships
- `get labels <owner/image>` - Extract OCI annotations from images
- `get sbom <owner/image>` - Display SBOM attestations (SPDX, CycloneDX, Syft, Docker buildx formats)
- `get provenance <owner/image>` - Display provenance attestations (SLSA v0.2, v1.0, in-toto, Docker buildx formats)
- `tag <owner/image> <new-tag>` - Add tags to existing images via OCI registry API
- `delete version <owner/image>` - Delete single or multiple versions with filters
- `delete graph <owner/image>` - Delete complete OCI graphs including platforms and attestations
- `delete package <owner/image>` - Delete entire packages with all versions
- `stats <owner/image>` - Display package statistics
- `completion` - Shell completion for bash, zsh, fish, and PowerShell with dynamic package suggestions

**Features**
- `--log-api-calls` flag for JSON logging of all HTTP requests with timing data
- `--quiet` / `-q` flag for scripting mode
- `--dry-run` flag on delete commands to preview operations
- `--force` / `--yes` / `-y` flags to skip confirmation prompts
- Shared manifest detection during deletion to preserve artifacts used by multiple images
- Multi-platform builds for Linux, macOS, Windows on amd64 and arm64

**Infrastructure**
- CI pipeline with multi-platform testing, race detection, and coverage reporting
- Security scanning with govulncheck, gosec, and trivy
- Automated releases via GoReleaser
- Integration test suite against real GHCR images
- Mutating test suite for write operations
