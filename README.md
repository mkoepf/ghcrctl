# ghcrctl

[![CI](https://github.com/mkoepf/ghcrctl/actions/workflows/ci.yml/badge.svg)](https://github.com/mkoepf/ghcrctl/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/mkoepf/ghcrctl)](https://goreportcard.com/report/github.com/mkoepf/ghcrctl)
[![codecov](https://codecov.io/gh/mkoepf/ghcrctl/branch/main/graph/badge.svg)](https://codecov.io/gh/mkoepf/ghcrctl)

A command-line tool for interacting with GitHub Container Registry (GHCR).

## Features

ghcrctl provides functionality for:

- **Exploring images** and their versions (multi-arch platforms, SBOM, provenance, signatures)
- **Viewing SBOM** (Software Bill of Materials) attestations
- **Viewing provenance** attestations (SLSA)
- **Discovering signatures** and attestations from both Docker buildx and cosign
- **Managing GHCR version metadata** (labels, tags)
- **Safe deletion** of package versions and complete OCI graphs
- **Configuration** of owner/org and authentication

## Installation

### From Source

```bash
git clone https://github.com/mkoepf/ghcrctl.git
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
- `read:packages` - for read operations (list, versions, sbom, provenance)
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

  VERSION ID       TYPE         DIGEST        TAGS                          CREATED
  ---------------  -----------  ------------  ----------------------------  -------------------
┌ 585861918        index        01af50cc8b0d  [v1.0.0, latest]              2025-01-15 10:30:45
├ 585861919        linux/amd64  62f946a8267d  []                            2025-01-15 10:30:44
├ 585861920        linux/arm64  89c3b5f1a432  []                            2025-01-15 10:30:44
├ 585861921        sbom         9a1636d22702  []                            2025-01-15 10:30:46
└ 585861922        provenance   9a1636d22702  []                            2025-01-15 10:30:46

┌ 585850123        index        abc123def456  [v0.9.0]                      2025-01-14 15:20:10
├ 585861919 (2*)   linux/amd64  62f946a8267d  []                            2025-01-15 10:30:44
└ 585861920 (2*)   linux/arm64  89c3b5f1a432  []                            2025-01-15 10:30:44

Total: 7 versions in 2 graphs. 2 versions appear in multiple graphs.
```

**Shared manifests:**

When multiple image indexes reference the same platform manifests (e.g., two tags pointing to the same underlying builds), those shared versions are marked with `(N*)` where N indicates how many graphs reference them. This is common when:
- Multiple tags point to the same multi-arch image
- An image is re-tagged without rebuilding
- Different index manifests share platform manifests

The summary line reports the count of distinct versions and how many appear in multiple graphs.

**Filter options:**
```bash
# Show only versions with a specific tag (optimized - only graphs this version)
ghcrctl versions myimage --tag v1.0.0

# Show only tagged versions
ghcrctl versions myimage --tagged

# Show only untagged versions
ghcrctl versions myimage --untagged

# Show versions matching a tag pattern (regex)
ghcrctl versions myimage --tag-pattern "^v1\\..*"

# Filter by specific version ID
ghcrctl versions myimage --version 585861918

# Filter by digest (full or short format)
ghcrctl versions myimage --digest sha256:01af50cc8b0d
ghcrctl versions myimage --digest 01af50cc8b0d  # short form from DIGEST column

# Show versions older than a specific date
ghcrctl versions myimage --older-than 2025-01-01

# Show versions newer than a specific date
ghcrctl versions myimage --newer-than 2025-11-01

# Show versions older than 30 days
ghcrctl versions myimage --older-than-days 30

# Combine filters: untagged versions older than 7 days
ghcrctl versions myimage --untagged --older-than-days 7
```

**JSON output:**
```bash
ghcrctl versions myimage --json
# or
ghcrctl versions myimage -o json
```

**Verbose output:**

For detailed information including full digests and sizes, use the `--verbose` or `-v` flag:

```bash
ghcrctl versions myimage --tag latest --verbose
# or
ghcrctl versions myimage --tag latest -v
```

Example verbose output:
```
Versions for myimage:

┌─ 585861918  index
│  Digest:  sha256:01af50cc8b0d33e7bb9137d6aa60274975475ee048e6610da62670a30466a824543
│  Tags:    [v1.0.0, latest]
│  Created: 2025-01-15 10:30:45
│
├─ 585861919  platform: linux/amd64
│  Digest:  sha256:62f946a8267d2795505edc2ee029c7d8f2b76cff34912259b55fe0ad94d612c0
│  Size:    669 bytes
│  Created: 2025-01-15 10:30:44
│
├─ 585861920  platform: linux/arm64
│  Digest:  sha256:89c3b5f1a4322795505edc2ee029c7d8f2b76cff34912259b55fe0ad94d612c0
│  Size:    672 bytes
│  Created: 2025-01-15 10:30:44
│
└─ 585861921  attestation: sbom, provenance
   Digest:  sha256:9a1636d227022795505edc2ee029c7d8f2b76cff34912259b55fe0ad94d612c0
   Size:    12.4 KB
   Created: 2025-01-15 10:30:46
```

The verbose view shows:
- Full SHA256 digests (not truncated)
- Artifact sizes in human-readable format
- Clear type labels (`platform:`, `attestation:`)
- Combined attestation types when same digest (e.g., `sbom, provenance`)

**Performance optimization:**
When using `--tag` to filter, the command only discovers graph relationships for matching versions, significantly reducing API calls and execution time. Without the filter, all tagged versions are processed. Other filters (--tagged, --tag-pattern, date filters) are applied after listing all versions.

**Understanding version types:**
- `index` - Multi-arch image manifest list (references platform manifests)
- `linux/amd64`, `linux/arm64` - Platform-specific manifests
- `sbom`, `provenance` - Attestation artifacts (from buildx or cosign)
- `signature` - Cosign signatures
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

# Delete by digest (full or short format)
ghcrctl delete version myimage --digest sha256:abc123...
ghcrctl delete version myimage --digest abc123  # short form from DIGEST column

# Skip confirmation prompt
ghcrctl delete version myimage 12345678 --force

# Preview what would be deleted (dry-run)
ghcrctl delete version myimage 12345678 --dry-run
```

The command shows how many graphs the version belongs to:
```
Preparing to delete package version:
  Image:      myimage
  Owner:      myorg (org)
  Version ID: 12345678
  Tags:       []
  Graphs:     2 graphs
```

If a version belongs to multiple graphs, deleting it will affect all those graphs.

**Use cases:**
- Remove specific untagged versions (e.g., orphaned attestations)
- Clean up individual failed builds
- Delete a single platform manifest

#### Bulk Delete Multiple Versions

Delete multiple versions at once using filters:

```bash
# Delete all untagged versions
ghcrctl delete version myimage --untagged

# Delete untagged versions older than 30 days
ghcrctl delete version myimage --untagged --older-than-days 30

# Delete versions matching a tag pattern older than a specific date
ghcrctl delete version myimage --tag-pattern ".*-rc.*" --older-than 2025-01-01

# Delete versions older than a specific date
ghcrctl delete version myimage --older-than "2025-01-01"

# Preview what would be deleted (dry-run)
ghcrctl delete version myimage --untagged --dry-run

# Skip confirmation for automated cleanup
ghcrctl delete version myimage --untagged --older-than-days 30 --force
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

**Example output:**
```bash
$ ghcrctl delete version myimage --untagged --older-than-days 30

Preparing to delete 5 package version(s):
  Image: myimage
  Owner: myorg (org)

  - ID: 12345678, Tags: [], Created: 2024-11-15 10:30:45
  - ID: 12345679, Tags: [], Created: 2024-11-16 14:22:10
  - ID: 12345680, Tags: [], Created: 2024-11-17 09:15:33
  - ID: 12345681, Tags: [], Created: 2024-11-18 16:45:22
  - ID: 12345682, Tags: [], Created: 2024-11-20 11:30:15

Are you sure you want to delete 5 version(s)? [y/N]:
```

**Use cases:**
- Clean up old untagged versions to reduce storage costs
- Remove release candidates after final release
- Delete old development builds
- Automate cleanup in CI/CD pipelines

**Safety features:**
- Preview of what will be deleted (shows up to 10 versions, then "...and N more")
- Confirmation prompt (unless `--force`)
- Dry-run mode (`--dry-run`) to preview without deleting
- Reports success/failure counts after deletion
- Gracefully handles no matching versions

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
1. All **exclusive** attestations (SBOM, provenance) - deleted first
2. All **exclusive** platform manifests (linux/amd64, linux/arm64, etc.)
3. The root image index or manifest - deleted last

**Shared manifests are preserved:** If a platform manifest or attestation is referenced by multiple graphs (e.g., two tags share the same builds), those shared artifacts are NOT deleted. They remain available for the other graphs that still reference them. Only when deleting the last graph that references a shared artifact will it be deleted.

Example output:
```bash
$ ghcrctl delete graph myimage v1.0.0

Preparing to delete complete OCI graph:
  Image: myimage
  Tag:   v1.0.0

Root (Image): sha256:01af50c...
  Tags: [v1.0.0]
  Version ID: 585861918

Platforms to delete (2):
  - linux/amd64 (version 585861919)
  - linux/arm64 (version 585861920)

Attestations to delete (2):
  - sbom (version 585861921)
  - provenance (version 585861922)

Total: 5 version(s) will be deleted

Are you sure you want to delete this graph? [y/N]:
```

**Example with shared manifests:**
```bash
$ ghcrctl delete graph myimage v0.9.0 --dry-run

Preparing to delete complete OCI graph:
  Image: myimage

Root (Image): sha256:abc123d...
  Version ID: 585850123

Shared artifacts (preserved, used by other graphs):
  - linux/amd64 (version 585861919, shared by 2 graphs)
  - linux/arm64 (version 585861920, shared by 2 graphs)

Total: 1 version(s) will be deleted

DRY RUN: No changes made
```

In this example, only the root index is deleted because the platform manifests are shared with another graph (v1.0.0).

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

**Example: Analyzing versions command performance:**
```bash
./ghcrctl versions myimage --verbose --log-api-calls 2>api-calls.log
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
- SBOM and provenance discovery (both buildx and cosign formats)
- Multi-layer attestation detection (Docker buildx pattern)
- Cosign signature and attestation discovery via tag patterns
- Platform manifest extraction (multi-arch images)
- Versions command with verbose output
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
  - Discovers cosign signatures (`.sig` tags) and attestations (`.att` tags)
  - Resolves attestation types from predicate annotations (SPDX, CycloneDX, SLSA, etc.)

## License

See [LICENSE](LICENSE) file for details.
