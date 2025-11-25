# ghcrctl

[![CI](https://github.com/mkoepf/ghcrctl/actions/workflows/ci.yml/badge.svg)](https://github.com/mkoepf/ghcrctl/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/mkoepf/ghcrctl)](https://goreportcard.com/report/github.com/mkoepf/ghcrctl)
[![codecov](https://codecov.io/gh/mkoepf/ghcrctl/branch/main/graph/badge.svg)](https://codecov.io/gh/mkoepf/ghcrctl)

A command-line tool for interacting with GitHub Container Registry (GHCR).

## Features

ghcrctl provides functionality for:

- **Exploring images** and their OCI artifact graph (multi-arch platforms, SBOM, provenance)
- **Viewing SBOM** (Software Bill of Materials) attestations
- **Viewing provenance** attestations (SLSA)
- **Managing GHCR version metadata** (labels, tags)
- **Safe deletion** of package versions and complete OCI graphs
- **Configuration** of owner/org and authentication

## Installation

### From Source

```bash
git clone https://github.com/mhk/ghcrctl.git
cd ghcrctl
go build -o ghcrctl .
```

## Usage

### Configuration

Set your GitHub organization:

```bash
ghcrctl config org mycompany
```

Or set a user:

```bash
ghcrctl config user myusername
```

View current configuration:

```bash
ghcrctl config show
```

#### Configuration File

Configuration is stored in `~/.ghcrctl/config.yaml`:

```yaml
owner-name: myorg
owner-type: org
```

#### Environment Variables (Recommended for Parallel Sessions)

You can override the config file by setting environment variables. This is particularly useful for working with multiple owners in parallel terminal sessions:

```bash
# Work with an organization
export GHCRCTL_OWNER=mycompany
export GHCRCTL_OWNER_TYPE=org
ghcrctl images

# In another terminal, work with your personal account
export GHCRCTL_OWNER=myusername
export GHCRCTL_OWNER_TYPE=user
ghcrctl images
```

**Priority order:**
1. Environment variables (`GHCRCTL_OWNER`, `GHCRCTL_OWNER_TYPE`) - highest priority
2. Config file (`~/.ghcrctl/config.yaml`) - fallback

**Default behavior:** If only `GHCRCTL_OWNER` is set, `GHCRCTL_OWNER_TYPE` defaults to `user`.

**Quick one-off commands:**
```bash
# Run a single command with different owner without changing config
GHCRCTL_OWNER=otherorg GHCRCTL_OWNER_TYPE=org ghcrctl images
```

### Authentication

Set your GitHub Personal Access Token (PAT) as an environment variable:

```bash
export GITHUB_TOKEN=ghp_your_token_here
```

**Required token type:** Classic Personal Access Token (PAT) or Fine-grained PAT
**Required scopes:**
- `read:packages` - for read operations (list, graph, sbom, provenance)
- `write:packages` - for write operations (tag command)

**Note:** GitHub App installation tokens (`ghs_*` prefix) are not supported for write operations to GHCR via the OCI registry API.

### List Container Images

List all container images for the configured owner:

```bash
ghcrctl images
```

Get output in JSON format:

```bash
ghcrctl images --json
```

### Display OCI Artifact Graph

Display the complete OCI artifact graph for an image, showing all related package versions:

```bash
ghcrctl graph myimage
```

This command reveals the complete structure of your container image, explaining all those "untagged" package versions you see in GHCR:

**Performance note:** When querying untagged child artifacts (like platform manifests or attestations) by version ID or digest, the tool must search through all package versions to find the parent graph. This is currently the only reliable method, as neither the OCI `subject` field (not populated by Docker Buildx) nor the GitHub Package API provide parent-child relationship metadata. The search stops as soon as the parent is found, but for packages with many versions, this operation can be slow.

**What it shows:**
- **Image Index** (manifest list) or **Manifest** (single-arch)
- **Platform Manifests** (references) - Each architecture creates an untagged version (linux/amd64, linux/arm64, etc.)
- **Attestations** (referrers) - SBOM and provenance attestations
- **Version breakdown** - Shows which versions are tagged vs. untagged

**Example: Multi-arch image with attestations**
```
OCI Artifact Graph for myimage

Image Index: sha256:01af50cc8b0d33e...
  Tags: [latest]
  Version ID: 585861918
  │
  ├─ Platform Manifests (references):
  │    ├─ linux/amd64
  │    │  Digest: sha256:62f946a8267d...
  │    │  Size: 669 bytes
  │    └─ linux/arm64
  │       Digest: sha256:89c3b5f1a432...
  │       Size: 672 bytes
  │
  └─ Attestations (referrers):
         ├─ sbom
         │  Digest: sha256:9a1636d22702...
         └─ provenance
            Digest: sha256:9a1636d22702...

Summary:
  Platforms: 2
  SBOM: true
  Provenance: true
  Total versions: 5 (1 tagged, 4 untagged)
```

**Understanding the output:**
- **References** (inside image index): Platform-specific manifests that the multi-arch image consists of
- **Referrers** (external): Attestations that reference the image (SBOM, provenance, signatures)
- **Total versions**: Complete count explaining all package versions in GHCR for this image

Specify a tag (default is `latest`):

```bash
ghcrctl graph myimage --tag v1.0.0
```

Get output in JSON format:

```bash
ghcrctl graph myimage --json
```

### List Package Versions

List all versions of a package with their complete OCI artifact relationships:

```bash
ghcrctl versions myimage
```

This command shows all package versions in GHCR organized by their OCI relationships:

**What it shows:**
- **All versions** - Both tagged and untagged versions
- **Hierarchical structure** - Root artifacts with their children (platforms, attestations)
- **Version metadata** - ID, type, digest, tags, and creation time
- **Graph relationships** - How versions relate to each other

**Example output:**
```
Versions for myimage:

  VERSION ID  TYPE         DIGEST        TAGS                          CREATED
  ----------  -----------  ------------  ----------------------------  -------------------
┌ 585861918   index        01af50cc8b0d  [v1.0.0, latest]              2025-01-15 10:30:45
├ 585861919   linux/amd64  62f946a8267d  []                            2025-01-15 10:30:44
├ 585861920   linux/arm64  89c3b5f1a432  []                            2025-01-15 10:30:44
├ 585861921   sbom         9a1636d22702  []                            2025-01-15 10:30:46
└ 585861922   provenance   9a1636d22702  []                            2025-01-15 10:30:46

┌ 585850123   index        abc123def456  [v0.9.0]                      2025-01-14 15:20:10
├ 585850124   linux/amd64  def456abc123  []                            2025-01-14 15:20:09
└ 585850125   linux/arm64  789xyz123456  []                            2025-01-14 15:20:09

Total: 8 version(s) in 2 graph(s)
```

**Filter by tag:**
```bash
# Show only versions with a specific tag
ghcrctl versions myimage --tag v1.0.0

# This is optimized - only builds graph for the filtered version
# Reduces API calls by 92% compared to listing all versions
```

**JSON output:**
```bash
ghcrctl versions myimage --json
```

**Performance optimization:**
When using `--tag` to filter, the command only discovers graph relationships for matching versions, significantly reducing API calls and execution time. Without the filter, all tagged versions are processed.

**Understanding version types:**
- `index` - Multi-arch image manifest list (references platform manifests)
- `linux/amd64`, `linux/arm64` - Platform-specific manifests
- `sbom`, `provenance` - Attestation artifacts
- `untagged` - Standalone version with no relationships

**Use cases:**
- Audit all versions of an image
- Understand which versions are tagged vs untagged
- Find orphaned versions for cleanup
- Verify attestations exist for all builds
- Quick lookup of version IDs for deletion

### View SBOM (Software Bill of Materials)

Display the SBOM attestation for a container image:

```bash
ghcrctl sbom myimage
```

The command automatically handles multiple SBOMs:
- **One SBOM found**: Displays it automatically
- **Multiple SBOMs found**: Lists them and prompts you to select one
- **No SBOM found**: Clear error message

**Options:**

```bash
# Use a specific tag (default: latest)
ghcrctl sbom myimage --tag v1.0.0

# Select a specific SBOM by digest
ghcrctl sbom myimage --digest abc123def456

# Show all SBOMs
ghcrctl sbom myimage --all

# Output as raw JSON
ghcrctl sbom myimage --json
```

**Example with multiple SBOMs:**
```
Multiple SBOMs found for myimage

Use --digest <digest> to select one, or --all to show all:

  1. sha256:abc123def456...
     Type: application/vnd.in-toto+json
  2. sha256:789xyz123456...
     Type: application/spdx+json

Example: ghcrctl sbom myimage --digest abc123def456
```

**Supported formats:**
- SPDX (JSON)
- CycloneDX (JSON)
- Syft native format
- Docker buildx attestations (in-toto DSSE envelopes)

### View Provenance Attestation

Display the provenance attestation for a container image:

```bash
ghcrctl provenance myimage
```

Provenance attestations contain build information including:
- Builder details (GitHub Actions, GitLab CI, etc.)
- Source repository and commit
- Build invocation and parameters
- SLSA provenance level

**Options:**

```bash
# Use a specific tag (default: latest)
ghcrctl provenance myimage --tag v1.0.0

# Select specific provenance by digest
ghcrctl provenance myimage --digest abc123def456

# Show all provenance documents
ghcrctl provenance myimage --all

# Output as raw JSON
ghcrctl provenance myimage --json
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
ghcrctl tag myimage v1.0.0 latest
```

This command creates a new tag reference pointing to the same image digest as the existing tag, using the OCI registry API. It works like `docker tag` but operates directly on GHCR.

**Requirements:**
- GITHUB_TOKEN with `write:packages` scope
- Must use Personal Access Token (not GitHub App installation token)

**Example use cases:**
- Promote a version to `latest`: `ghcrctl tag myapp v2.1.0 latest`
- Add semantic version alias: `ghcrctl tag myapp v1.2.3 v1.2`
- Tag for environment: `ghcrctl tag myapp sha256:abc123... production`

### Delete Package Versions

Safely delete package versions or complete OCI artifact graphs from GHCR.

#### Delete a Single Version

Delete an individual package version by version ID or digest:

```bash
# Delete by version ID
ghcrctl delete version myimage 12345678

# Delete by digest
ghcrctl delete version myimage --digest sha256:abc123...

# Skip confirmation prompt
ghcrctl delete version myimage 12345678 --force

# Preview what would be deleted (dry-run)
ghcrctl delete version myimage 12345678 --dry-run
```

**Use cases:**
- Remove specific untagged versions (e.g., orphaned attestations)
- Clean up individual failed builds
- Delete a single platform manifest

**Requirements:**
- GITHUB_TOKEN with `write:packages` and `delete:packages` scope
- Must use Personal Access Token (not GitHub App installation token)

#### Delete an Entire Graph

Delete a complete OCI artifact graph including the root image, all platform manifests, and attestations (SBOM, provenance):

```bash
# Delete by tag (most common)
ghcrctl delete graph myimage v1.0.0

# Delete by digest
ghcrctl delete graph myimage --digest sha256:abc123...

# Delete graph containing a specific version
ghcrctl delete graph myimage --version 12345678

# Skip confirmation
ghcrctl delete graph myimage v1.0.0 --force

# Preview what would be deleted
ghcrctl delete graph myimage v1.0.0 --dry-run
```

**What gets deleted:**

For a multi-arch image with attestations, this command discovers and deletes:
1. All attestations (SBOM, provenance) - deleted first
2. All platform manifests (linux/amd64, linux/arm64, etc.)
3. The root image index or manifest - deleted last

Example output:
```bash
$ ghcrctl delete graph myimage v1.0.0

Preparing to delete complete OCI graph:
  Image: myimage
  Tag:   v1.0.0

Root (Image): sha256:01af50c...
  Tags: [v1.0.0]
  Version ID: 585861918

Platforms (2):
  - linux/amd64 (version 585861919)
  - linux/arm64 (version 585861920)

Attestations (2):
  - sbom (version 585861921)
  - provenance (version 585861922)

Total: 5 version(s) will be deleted

Are you sure you want to delete this graph? [y/N]:
```

**Use cases:**
- Remove an entire release (tag)
- Clean up complete multi-arch images with all artifacts
- Delete all versions associated with a specific build

**Safety features:**
- Automatic graph discovery - you specify the tag, tool finds all related versions
- Deletion order - children (attestations, platforms) deleted before root
- Confirmation prompt (unless `--force`)
- Dry-run mode to preview
- Clear summary of what will be deleted

**Requirements:**
- GITHUB_TOKEN with `write:packages` and `delete:packages` scope
- Must use Personal Access Token (not GitHub App installation token)

**IMPORTANT:** Deletion is permanent and cannot be undone (except within 30 days via the GitHub web UI if the package namespace is still available).

### Getting Help

```bash
ghcrctl --help
ghcrctl config --help
ghcrctl images --help
ghcrctl graph --help
ghcrctl versions --help
ghcrctl sbom --help
ghcrctl provenance --help
ghcrctl tag --help
ghcrctl delete --help
ghcrctl delete version --help
ghcrctl delete graph --help
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

**Example: Analyzing graph command performance:**
```bash
./ghcrctl graph myimage --log-api-calls 2>api-calls.log
# Analyze the log to see which API calls take longest
jq -r 'select(.duration_ms > 100) | "\(.duration_ms)ms - \(.method) \(.path)"' api-calls.log
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

See [TESTING.md](TESTING.md) for comprehensive testing instructions.

### Quick Test

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test ./... -cover

# Run linters
golangci-lint run ./...
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
- SBOM and provenance discovery
- Multi-layer attestation detection (Docker buildx pattern)
- Platform manifest extraction (multi-arch images)
- Graph command with tree-style output
- SBOM command end-to-end functionality
- Provenance command end-to-end functionality
- JSON and table/tree output formats

However, the following tests are **not possible**:

1. **Listing all user/org packages** - `ghcrctl images` requires broader `read:packages` access
2. **Cross-repository operations** - Can only access packages within the ghcrctl repository
3. **Package deletion** - Would require `write:packages` and `delete:packages` permissions
4. **Private registry access** - Tests only work with packages in the ghcrctl repository scope

These limitations are intentional to maintain security - integration tests use
minimally-scoped tokens that can safely run in CI without risking access to
other repositories or packages.

For testing broader functionality, use unit tests with mocked responses or
manual testing with appropriate credentials.

## Architecture

- **CLI Layer**: Built with [Cobra](https://github.com/spf13/cobra)
- **Configuration**: Managed with [Viper](https://github.com/spf13/viper)
- **GHCR API**: Using [go-github](https://github.com/google/go-github) for package management
- **OCI Layer**: Using [ORAS Go SDK](https://oras.land/docs/category/oras-go-library) for tag resolution and artifact discovery
  - Supports OCI Referrers API and fallback to referrers tag schema
  - Discovers Docker buildx attestations stored in image indexes
  - Handles multi-layer attestation manifests (SBOM + provenance in single manifest)

## License

See [LICENSE](LICENSE) file for details.
