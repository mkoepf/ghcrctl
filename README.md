# ghcrctl

[![CI](https://github.com/mkoepf/ghcrctl/actions/workflows/ci.yml/badge.svg)](https://github.com/mkoepf/ghcrctl/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/mkoepf/ghcrctl)](https://goreportcard.com/report/github.com/mkoepf/ghcrctl)
[![Coverage](https://img.shields.io/endpoint?url=https://gist.githubusercontent.com/mkoepf/baae226f8088579c1405b06f7dd1a07a/raw/ghcrctl-coverage.json)](https://github.com/mkoepf/ghcrctl/actions/workflows/ci.yml)

A command-line tool for interacting with GitHub Container Registry (GHCR).

## Why ghcrctl?

GitHub Container Registry (GHCR) leaves you flying blind.

**The problem:** When you look at your container packages on GitHub, you see a list of "versions" — but what *are* they? An image index? A platform manifest? An SBOM? A signature? GHCR's UI and API don't tell you how these pieces fit together.

**It gets worse.** You might try using standard OCI tools like ORAS to explore and manage your images. You can *discover* artifacts just fine:

```bash
oras discover ghcr.io/myorg/myimage:latest
```

But when you try to delete:

```bash
oras manifest delete ghcr.io/myorg/myimage@sha256:abc123...
# Error: unsupported: The operation is unsupported.
```

GHCR doesn't support deletion via the OCI API. Your only option is GitHub's REST API, which deletes package *versions* — opaque numeric IDs with no visibility into what you're actually removing.

**ghcrctl bridges this gap.** It combines OCI-level transparency with GitHub API access:

- **See the full picture:** View your images as OCI artifact graphs — indexes, platform manifests, attestations, signatures — all in context
- **Delete with confidence:** Know exactly what you're removing before you remove it
- **Work efficiently:** Bulk operations, filters, dry-run mode, and scripting-friendly JSON output

Think of ghcrctl as the container registry CLI that GHCR should have shipped with.

## Features

ghcrctl provides functionality for:

- **Exploring packages** and their versions (multi-arch platforms, SBOM, provenance, signatures)
- **Package statistics** (version counts, date ranges)
- **Viewing images** with their OCI artifact relationships (platforms, attestations)
- **Viewing labels** (OCI annotations) embedded in container images
- **Viewing and adding tags** (tag deletion is not supported by GHCR)
- **Viewing SBOM** (Software Bill of Materials) attestations
- **Viewing provenance** attestations (SLSA)
- **Discovering signatures** and attestations from both Docker buildx and cosign
- **Safe deletion** of package versions, images, and entire packages
- **Shell completion** with dynamic package name suggestions

## Installation

### Using Go Install

```bash
go install github.com/mkoepf/ghcrctl@latest
```

### From Source

```bash
git clone https://github.com/mkoepf/ghcrctl.git
cd ghcrctl
go build -o ghcrctl .
```

## Usage

All commands use the `owner/image[:tag]` format, where owner is automatically detected as user or organization.

### Authentication

Set your GitHub Personal Access Token (PAT) as an environment variable:

```bash
export GITHUB_TOKEN=ghp_your_token_here
```

**Required token type:** Classic Personal Access Token (PAT) or Fine-grained PAT
**Required scopes:**
- `read:packages` - for read operations (list, versions, sbom, provenance)
- `write:packages` - for write operations (tag command)

**Note:** GitHub App installation tokens (`ghs_*` prefix) are not supported for write operations to GHCR via the OCI registry API.

### Global Flags

These flags are available on all commands:

```bash
# Suppress informational output (for scripting)
ghcrctl list versions mkoepf/myimage --quiet
ghcrctl list versions mkoepf/myimage -q

# Log all API calls with timing (for debugging/performance analysis)
ghcrctl list images mkoepf/myimage --log-api-calls
```

### List Packages

List all container packages for an owner:

```bash
ghcrctl list packages mkoepf
ghcrctl list packages myorg
```

Get output in JSON format:

```bash
ghcrctl list packages mkoepf --json
```

### List Images

Display all images in a package with their related artifacts (platforms, attestations, signatures):

```bash
ghcrctl list images mkoepf/myimage
```

This command shows images grouped by their OCI artifact relationships in a tree format:

```
       VERSION ID       TYPE              DIGEST        SIZE    TAGS
       ----------       ----------------  ------------  ------  ----
┌      585861918        index             01af50cc8b0d  1.6 KB  [v1.0.0, latest]
├ [⬇✓] 585861919        linux/amd64       62f946a8267d  2.5 KB
├ [⬇✓] 585861920        linux/arm64       89c3b5f1a432  2.5 KB
├ [⬇✓] 585861921        sbom, provenance  9a1636d22702  839 B
└ [⬇✓] 585861922        sbom, provenance  9a1636d22703  839 B

┌      585850123        index             abc123def456  1.6 KB  [v0.9.0]
├ [⬇✓] 585861919  (2*)  linux/amd64       62f946a8267d  2.5 KB
└ [⬇✓] 585861920  (2*)  linux/arm64       89c3b5f1a432  2.5 KB

Total: 9 versions in 2 graphs. 2 versions appear in multiple graphs.
```

The `(N*)` notation indicates versions shared across multiple graphs.

**Options:**

```bash
# Show images in flat table format
ghcrctl list images mkoepf/myimage --flat

# Output in JSON format
ghcrctl list images mkoepf/myimage --json

# Filter to images containing a specific version ID
ghcrctl list images mkoepf/myimage --version 12345678

# Filter to images containing a specific tag
ghcrctl list images mkoepf/myimage --tag v1.0.0

# Filter to images containing a specific digest
ghcrctl list images mkoepf/myimage --digest sha256:abc123...
```

**Use cases:**
- Quick overview of all images and their artifacts
- Find images that contain a specific manifest
- Identify shared platform manifests across images

### List Package Versions

List all versions of a package as a flat table:

```bash
ghcrctl list versions mkoepf/myimage
```

This command displays all GHCR package versions with their metadata:

**Example output:**
```
Versions for myimage:

  VERSION ID  DIGEST        TAGS                  CREATED
  ----------  ------------  --------------------  -------------------
  585861918   01af50cc8b0d  [v1.0.0, latest]      2025-01-15 10:30:45
  585861919   62f946a8267d  []                    2025-01-15 10:30:44
  585861920   89c3b5f1a432  []                    2025-01-15 10:30:44
  585861921   9a1636d22702  []                    2025-01-15 10:30:46
  585861922   9a1636d22703  []                    2025-01-15 10:30:46

Total: 5 versions.
```

To see artifact relationships (platform manifests, attestations), use `ghcrctl list images` instead.

**Filter options:**
```bash
# Filter by specific tag
ghcrctl list versions mkoepf/myimage --tag v1.0.0

# Show only tagged versions
ghcrctl list versions mkoepf/myimage --tagged

# Show only untagged versions
ghcrctl list versions mkoepf/myimage --untagged

# Show versions matching a tag pattern (regex)
ghcrctl list versions mkoepf/myimage --tag-pattern "^v1\\..*"

# Filter by specific version ID
ghcrctl list versions mkoepf/myimage --version 585861918

# Filter by digest (full or short format)
ghcrctl list versions mkoepf/myimage --digest sha256:01af50cc8b0d
ghcrctl list versions mkoepf/myimage --digest 01af50cc8b0d

# Show versions older than a specific date
ghcrctl list versions mkoepf/myimage --older-than 2025-01-01

# Show versions newer than a specific date
ghcrctl list versions mkoepf/myimage --newer-than 2025-11-01

# Show versions older than 30 days
ghcrctl list versions mkoepf/myimage --older-than-days 30

# Combine filters: untagged versions older than 7 days
ghcrctl list versions mkoepf/myimage --untagged --older-than-days 7
```

**JSON output:**
```bash
ghcrctl list versions mkoepf/myimage --json
# or
ghcrctl list versions mkoepf/myimage -o json
```

**Use cases:**
- Audit all versions of an image
- Understand which versions are tagged vs untagged
- Find orphaned versions for cleanup
- Quick lookup of version IDs for deletion

### Package Statistics

Display statistics for a container package:

```bash
ghcrctl stats mkoepf/myimage
```

This command shows an overview including:
- Total number of versions
- Number of tagged vs untagged versions
- Date range (oldest and newest versions)

**Options:**

```bash
# Output as JSON
ghcrctl stats mkoepf/myimage --json
```

**Use cases:**
- Quick overview of package size and age
- Identify packages with many untagged versions for cleanup
- Audit package activity over time

### Get Image Labels

Display OCI labels (annotations/metadata) from a container image:

```bash
ghcrctl get labels mkoepf/myimage --tag v1.0.0
ghcrctl get labels mkoepf/myimage --version 12345678
ghcrctl get labels mkoepf/myimage --digest abc123
```

Requires a selector: `--tag`, `--digest`, or `--version` to specify which version. The `--digest` flag supports short form (first 12 characters).

Labels are key-value pairs embedded in the image config at build time using Docker LABEL instructions. Common labels include:
- `org.opencontainers.image.source` - Source repository URL
- `org.opencontainers.image.description` - Image description
- `org.opencontainers.image.version` - Semantic version
- `org.opencontainers.image.licenses` - License identifier

**Options:**

```bash
# Show only a specific label key
ghcrctl get labels mkoepf/myimage --tag v1.0.0 --key org.opencontainers.image.source

# Output as JSON
ghcrctl get labels mkoepf/myimage --tag latest --json
```

### Get SBOM (Software Bill of Materials)

Display the SBOM attestation for a container image or version:

```bash
ghcrctl get sbom mkoepf/myimage --tag v1.0.0
ghcrctl get sbom mkoepf/myimage --version 12345678
ghcrctl get sbom mkoepf/myimage --digest abc123
```

Requires a selector: `--tag`, `--digest`, or `--version`. The `--digest` flag supports short form.

If the selected version is itself an SBOM, it is displayed directly. Otherwise, the command finds SBOMs in the image containing that version.

The command automatically handles multiple SBOMs:
- **One SBOM found**: Displays it automatically
- **Multiple SBOMs found**: Lists them so you can select a specific one
- **No SBOM found**: Clear error message

**Options:**

```bash
# Show all SBOMs for an image
ghcrctl get sbom mkoepf/myimage --tag v1.0.0 --all

# Output as raw JSON
ghcrctl get sbom mkoepf/myimage --tag v1.0.0 --json
```

**Example with multiple SBOMs:**
```
Multiple sbom documents found for myimage

Select one by digest, or use --all to show all:

  1. sha256:abc123def456...
  2. sha256:789xyz123456...

Example: ghcrctl get sbom myimage --digest abc123def456
```

**Supported formats:**
- SPDX (JSON)
- CycloneDX (JSON)
- Syft native format
- Docker buildx attestations (in-toto DSSE envelopes)

### Get Provenance Attestation

Display the provenance attestation for a container image or version:

```bash
ghcrctl get provenance mkoepf/myimage --tag v1.0.0
ghcrctl get provenance mkoepf/myimage --version 12345678
ghcrctl get provenance mkoepf/myimage --digest abc123
```

Requires a selector: `--tag`, `--digest`, or `--version`. The `--digest` flag supports short form.

If the selected version is itself a provenance attestation, it is displayed directly. Otherwise, the command finds provenance in the image containing that version.

Provenance attestations contain build information including:
- Builder details (GitHub Actions, GitLab CI, etc.)
- Source repository and commit
- Build invocation and parameters
- SLSA provenance level

**Options:**

```bash
# Show all provenance documents for an image
ghcrctl get provenance mkoepf/myimage --tag v1.0.0 --all

# Output as raw JSON
ghcrctl get provenance mkoepf/myimage --tag v1.0.0 --json
```

**Smart behavior:**
- Automatically displays if only one provenance found
- Lists multiple provenances if more than one exists
- Clear error if no provenance attestation found

**Supported formats:**
- SLSA Provenance v0.2
- SLSA Provenance v1.0
- in-toto attestations
- Docker buildx provenance

### Add Tags to Images

Add a new tag to an existing image version:

```bash
ghcrctl tag mkoepf/myimage latest --tag v1.0.0
ghcrctl tag mkoepf/myimage stable --version 12345678
ghcrctl tag mkoepf/myimage stable --digest abc123
```

Requires a selector: `--tag`, `--digest`, or `--version` to specify the source version. The `--digest` flag supports short form.

This command creates a new tag reference pointing to the same image digest as the source, using the OCI registry API. It works like `docker tag` but operates directly on GHCR.

**Requirements:**
- GITHUB_TOKEN with `write:packages` scope
- Must use Personal Access Token (not GitHub App installation token)

**Example use cases:**
- Promote a version to `latest`: `ghcrctl tag mkoepf/myapp latest --tag v2.1.0`
- Add semantic version alias: `ghcrctl tag mkoepf/myapp v1.2 --tag v1.2.3`
- Tag for environment: `ghcrctl tag mkoepf/myapp production --tag v2.1.0`
- Tag by version ID: `ghcrctl tag mkoepf/myapp release --version 12345678`

### Why There Is No Tag Delete Command

GHCR does not support deleting individual tags. The standard OCI Distribution Spec
`DELETE /v2/<name>/manifests/<tag>` endpoint returns `UNSUPPORTED` on ghcr.io.

The only deletion method available is via GitHub's REST API, which deletes **entire
package versions** (the manifest and all tags pointing to it). If an image has multiple
tags (e.g., `v1.0.0` and `latest`), you cannot remove just one tag while keeping the
other - deleting removes both because they reference the same digest.

**Workaround:** To "move" a tag like `latest` from an old version to a new one, use
`ghcrctl tag` to point the tag at the new digest. The old reference is implicitly
overwritten:

```bash
# "Move" latest from v1.0.0 to v2.0.0
ghcrctl tag mkoepf/myapp latest --tag v2.0.0
```

For more details, see:
- [GitHub Community Discussion #26267](https://github.com/orgs/community/discussions/26267)
- [GitHub Docs - Working with Container Registry](https://docs.github.com/en/packages/working-with-a-github-packages-registry/working-with-the-container-registry)

### Delete Package Versions

Safely delete package versions or complete OCI images from GHCR.

#### Delete a Single Version

Delete an individual package version by version ID, digest, or tag:

```bash
# Delete by version ID
ghcrctl delete version mkoepf/myimage --version 12345678

# Delete by digest (full or short format)
ghcrctl delete version mkoepf/myimage --digest sha256:abc123...
ghcrctl delete version mkoepf/myimage --digest abc123

# Delete by tag
ghcrctl delete version mkoepf/myimage --tag v1.0.0

# Skip confirmation prompt (--force or -y)
ghcrctl delete version mkoepf/myimage --version 12345678 --force
ghcrctl delete version mkoepf/myimage --version 12345678 -y

# Preview what would be deleted (dry-run)
ghcrctl delete version mkoepf/myimage --version 12345678 --dry-run
```

**Use cases:**
- Remove specific untagged versions (e.g., orphaned attestations)
- Clean up individual failed builds
- Delete a single platform manifest

#### Bulk Delete Multiple Versions

Delete multiple versions at once using filters:

```bash
# Delete all untagged versions
ghcrctl delete version mkoepf/myimage --untagged

# Delete untagged versions older than 30 days
ghcrctl delete version mkoepf/myimage --untagged --older-than-days 30

# Delete versions matching a tag pattern older than a specific date
ghcrctl delete version mkoepf/myimage --tag-pattern ".*-rc.*" --older-than 2025-01-01

# Delete versions older than a specific date
ghcrctl delete version mkoepf/myimage --older-than "2025-01-01"

# Preview what would be deleted (dry-run)
ghcrctl delete version mkoepf/myimage --untagged --dry-run

# Skip confirmation for automated cleanup
ghcrctl delete version mkoepf/myimage --untagged --older-than-days 30 --force
```

**Available filters:**
- `--untagged` - Delete only untagged versions
- `--tagged` - Delete only tagged versions
- `--tag-pattern <regex>` - Delete versions with tags matching pattern
- `--older-than <date>` - Delete versions older than date (YYYY-MM-DD or RFC3339)
- `--newer-than <date>` - Delete versions newer than date
- `--older-than-days <N>` - Delete versions older than N days
- `--newer-than-days <N>` - Delete versions newer than N days

Filters can be combined using AND logic (all must match).

**Use cases:**
- Clean up old untagged versions to reduce storage costs
- Remove release candidates after final release
- Delete old development builds
- Automate cleanup in CI/CD pipelines

**Safety features:**
- Confirmation prompt (unless `--force`)
- Dry-run mode (`--dry-run`) to preview without deleting
- Gracefully handles no matching versions

**Requirements:**
- GITHUB_TOKEN with `write:packages` and `delete:packages` scope
- Must use Personal Access Token (not GitHub App installation token)

#### Delete an Entire Image

Delete a complete OCI image including the root index, all platform manifests, and attestations (SBOM, provenance):

```bash
# Delete by tag (most common)
ghcrctl delete image mkoepf/myimage --tag v1.0.0

# Delete by digest
ghcrctl delete image mkoepf/myimage --digest sha256:abc123...

# Delete image containing a specific version
ghcrctl delete image mkoepf/myimage --version 12345678

# Skip confirmation (--force or -y)
ghcrctl delete image mkoepf/myimage --tag v1.0.0 --force
ghcrctl delete image mkoepf/myimage --tag v1.0.0 -y

# Preview what would be deleted
ghcrctl delete image mkoepf/myimage --tag v1.0.0 --dry-run
```

Requires a selector: `--tag`, `--digest`, or `--version`.

**What gets deleted:**

For a multi-arch image with attestations, this command discovers and deletes:
1. All **exclusive** attestations (SBOM, provenance) - deleted first
2. All **exclusive** platform manifests (linux/amd64, linux/arm64, etc.)
3. The root image index or manifest - deleted last

**Shared manifests are preserved:** If a platform manifest or attestation is referenced by multiple images (e.g., two tags share the same builds), those shared artifacts are NOT deleted. They remain available for the other images that still reference them.

**Use cases:**
- Remove an entire release (tag)
- Clean up complete multi-arch images with all artifacts
- Delete all versions associated with a specific build

**Safety features:**
- Automatic graph discovery - you specify the tag, tool finds all related versions
- Deletion order - children (attestations, platforms) deleted before root
- Confirmation prompt (unless `--force`)
- Dry-run mode to preview

**Requirements:**
- GITHUB_TOKEN with `write:packages` and `delete:packages` scope
- Must use Personal Access Token (not GitHub App installation token)

**IMPORTANT:** Deletion is permanent and cannot be undone (except within 30 days via the GitHub web UI if the package namespace is still available).

#### Delete an Entire Package

Delete a complete package including all versions:

```bash
# Delete a package (will prompt for confirmation)
ghcrctl delete package mkoepf/myimage

# Skip confirmation
ghcrctl delete package mkoepf/myimage --force
```

This command deletes the package and ALL its versions permanently. Use this when you need to remove an entire package or when you cannot delete the last tagged version individually.

**Use cases:**
- Remove an entire package that is no longer needed
- Clean up test or temporary packages
- Delete packages when individual version deletion fails

**Safety features:**
- Shows version count before deletion (total, tagged, untagged)
- Requires typing the package name to confirm (not just y/n)
- Use `--force` only for automated scripts where you accept the risk

**Requirements:**
- GITHUB_TOKEN with `write:packages` and `delete:packages` scope
- Must use Personal Access Token (not GitHub App installation token)

**IMPORTANT:** This deletes the entire package and all versions. This action is permanent and cannot be undone (except within 30 days via the GitHub web UI if the package namespace is still available).

### Shell Completion

ghcrctl supports shell completion with dynamic image name suggestions.

**Setup:**

```bash
# Bash (Linux)
ghcrctl completion bash > /etc/bash_completion.d/ghcrctl

# Bash (macOS with Homebrew)
ghcrctl completion bash > $(brew --prefix)/etc/bash_completion.d/ghcrctl

# Zsh
mkdir -p ~/.zsh/completions
ghcrctl completion zsh > ~/.zsh/completions/_ghcrctl
# Add to ~/.zshrc: fpath=(~/.zsh/completions $fpath); autoload -Uz compinit && compinit

# Fish
ghcrctl completion fish > ~/.config/fish/completions/ghcrctl.fish

# PowerShell
ghcrctl completion powershell > ghcrctl.ps1
# Source this file from your PowerShell profile
```

**Usage:**

After setup, press TAB to complete commands and image names:

```bash
ghcrctl list ver<TAB>              # completes to: ghcrctl list versions
ghcrctl list versions mkoepf/<TAB> # shows: mkoepf/myimage, mkoepf/otherapp, ...
```

Dynamic package completion requires `GITHUB_TOKEN` to be exported in your shell.

### Getting Help

```bash
ghcrctl --help
ghcrctl list --help
ghcrctl list packages --help
ghcrctl list images --help
ghcrctl list versions --help
ghcrctl stats --help
ghcrctl get --help
ghcrctl get labels --help
ghcrctl get sbom --help
ghcrctl get provenance --help
ghcrctl tag --help
ghcrctl delete --help
ghcrctl delete version --help
ghcrctl delete image --help
ghcrctl delete package --help
ghcrctl completion --help
```

### API Call Logging

Enable detailed logging of all API calls for performance analysis and debugging:

```bash
ghcrctl <command> --log-api-calls
```

This flag logs all HTTP requests to stderr as JSON, including:
- Timestamp and duration
- API category (github, oci, other)
- HTTP method, URL, and path
- Response status and sizes
- Source code location (file:line:function)

**Example output:**
```json
{"timestamp":"2025-01-15T10:30:45Z","category":"github","method":"GET","url":"https://api.github.com/users/myorg/packages","path":"/users/myorg/packages","status":200,"duration_ms":245,"request_bytes":0,"response_bytes":1523,"caller":"client.go:42:ListPackages"}
```

**Use cases:**
- Performance analysis: Identify slow API calls
- Debugging: Trace API request flow through code
- Auditing: Track which operations make API calls
- Optimization: Find opportunities to reduce API calls

**Example: Analyzing list images command performance:**
```bash
./ghcrctl list images mkoepf/myimage --log-api-calls 2>api-calls.log
# Analyze the log to see which API calls take longest
jq -r 'select(.duration_ms > 100) | "\(.duration_ms)ms - \(.method) \(.path)"' api-calls.log
```

### Practical Examples

**CI/CD cleanup script:**
```bash
# Clean up old untagged images in CI (preview first with --dry-run)
ghcrctl delete version myorg/myapp --untagged --older-than-days 30 --dry-run

# Execute cleanup (use --force in CI to skip confirmation)
ghcrctl delete version myorg/myapp --untagged --older-than-days 30 --force
```

**Quick audit - count versions:**
```bash
# Count total versions in a package
ghcrctl list versions myorg/myapp --json | jq 'length'

# Count untagged versions
ghcrctl list versions myorg/myapp --untagged --json | jq 'length'
```

**Verify a release has attestations:**
```bash
# Check if the latest release has SBOM and provenance
ghcrctl list images myorg/myapp --tag latest

# Look for "sbom, provenance" in the TYPE column of children
```

## Development Status

This project is under active development following an iterative approach:

- ✅ **Iteration 1**: Project setup & configuration foundation
- ✅ **Iteration 2**: GitHub authentication & basic connection
- ✅ **Iteration 3**: List container images
- ✅ **Iteration 4**: ORAS integration & tag resolution
- ✅ **Iteration 5**: OCI graph discovery
- ✅ **Iteration 6**: Tagging Functionality
- ✅ **Iteration 7**: Labeling Functionality
- ✅ **Iteration 8**: Basic Deletion with Safety
- ✅ **Iteration 9**: Advanced Deletion Operations (Graph deletion)
- ⏳ **Iteration 10**: Interactive Mode & Polish

See [spec/plan.md](spec/plan.md) for the detailed development plan.

## Testing

### Quick Test

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test ./... -cover

# Run code quality checks (format, vet, tests, security scans)
./scripts/code_quality.sh
```

### Integration Testing

This project includes integration tests that verify functionality against real
GitHub Container Registry images.

#### How Integration Tests Work

Integration tests require real GHCR images with attestations. These are built
and pushed to ghcr by the [prepare-integration-test
workflow](.github/workflows/prepare-integration-test.yml). The workflow is
triggered manually and does not need to be re-run, unless new tests require 
more or different images.


#### Authentication Requirements

Integration tests use the `GITHUB_TOKEN` environment variable for
authentication:

**In GitHub Actions CI:**
```yaml
env:
  GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

The `GITHUB_TOKEN` automatically provided by GitHub Actions has access to
packages within the repository scope.

**For Local Development:**

Since the test images are packages of the ghcrctl repository, you need a token
with `read:packages` scope for the repo `https://github.com/mkoepf/ghcrct`:

**Option 1: Fine-Grained Personal Access Token (Recommended)**
1. Create a fine-grained PAT at https://github.com/settings/tokens?type=beta
2. Set **Repository access** to "Only select repositories" → `mkoepf/ghcrctl`
3. Under **Permissions** → **Repository permissions**, grant:
   - `packages: read` (to read test images)
   - `packages: write` (to change tags, labels)
4. Export the token:
   ```bash
   export GITHUB_TOKEN=ghp...
   ```

**Option 2: GitHub App Installation Token**
Configure a GitHub App with `packages:read` and `packages:write` permission for
the repository. Note that even with `packages:write`, an installation token is
not sufficient to modify / add tags in GHCR. This is because GitHub API does not
support adding tags and installation tokens are not supported (yet?) by GHCR's
OCI registration API. 

**Running Integration Tests:**
```bash
# With GITHUB_TOKEN set, integration tests run as part of all tests
go test ./...

# Run only integration tests
go test ./... -run Integration

# Without GITHUB_TOKEN, integration tests are skipped (allowed locally)
unset GITHUB_TOKEN
go test ./...  # Integration tests will be skipped
```

**CI Policy:**
In GitHub Actions CI, **all tests must run** - if any tests are skipped, the build will fail. This ensures that integration tests always run in CI where `GITHUB_TOKEN` is available. 

#### Limitations

The scoped token used in the integration tests allows to test

- Tag resolution (latest, semantic versions, multiple tags)
- SBOM and provenance discovery (both buildx and cosign formats)
- Multi-layer attestation detection (Docker buildx pattern)
- Cosign signature and attestation discovery via tag patterns
- Platform manifest extraction (multi-arch images)
- Versions command with verbose output
- SBOM command end-to-end functionality
- Provenance command end-to-end functionality
- JSON and table/tree output formats

However, the following tests are **not possible**:

1. **Listing all user/org packages** - `ghcrctl packages` requires broader `read:packages` access
2. **Cross-repository operations** - Can only access packages within the ghcrctl repository
3. **Package deletion** - Would require `write:packages` and `delete:packages` permissions
4. **Private registry access** - Tests only work with packages in the ghcrctl repository scope

These limitations are intentional to maintain security - integration tests use
minimally-scoped tokens that can safely run in CI without risking access to
other repositories or packages.

For testing broader functionality, use unit tests with mocked responses or
manual testing with appropriate credentials.

## Architecture

- **CLI Layer**: Built with [Cobra](https://github.com/spf13/cobra) with shell completion support
- **GHCR API**: Using [go-github](https://github.com/google/go-github) for package management
- **OCI Layer**: Using [ORAS Go SDK](https://oras.land/docs/category/oras-go-library) for tag resolution and artifact discovery
  - Supports OCI Referrers API and fallback to referrers tag schema
  - Discovers Docker buildx attestations stored in image indexes
  - Handles multi-layer attestation manifests (SBOM + provenance in single manifest)
  - Discovers cosign signatures (`.sig` tags) and attestations (`.att` tags)
  - Resolves attestation types from predicate annotations (SPDX, CycloneDX, SLSA, etc.)

## License

See [LICENSE](LICENSE) file for details.
