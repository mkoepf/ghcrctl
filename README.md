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
- **Safe deletion** of package versions
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

### Authentication

Set your GitHub token as an environment variable:

```bash
export GITHUB_TOKEN=ghp_your_token_here
```

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

### Getting Help

```bash
ghcrctl --help
ghcrctl graph --help
ghcrctl sbom --help
ghcrctl provenance --help
ghcrctl config --help
```

## Development Status

This project is under active development following an iterative approach:

- ✅ **Iteration 1**: Project setup & configuration foundation
- ✅ **Iteration 2**: GitHub authentication & basic connection
- ✅ **Iteration 3**: List container images
- ✅ **Iteration 4**: ORAS integration & tag resolution
- ✅ **Iteration 5**: OCI graph discovery
- ⏳ **Iteration 6**: Tagging Functionality
- ⏳ **Iteration 7**: Labeling Functionality
- ⏳ **Iteration 8**: Basic Deletion with Safety
- ⏳ **Iteration 9**: Advanced Deletion Operations
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
4. Export the token:
   ```bash
   export GITHUB_TOKEN=github_pat_xxx...
   ```

**Option 2: GitHub App Installation Token**
Configure a GitHub App with `packages:read` permission for the repository.

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
In GitHub Actions CI, **all tests must run** - if any tests are skipped, the build will fail. This ensures that integration tests always run in CI where `GITHUB_TOKEN` is available. Locally, skipped tests are acceptable for development without credentials.

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

## Configuration File

Configuration is stored in `~/.ghcrctl/config.yaml`:

```yaml
owner-name: myorg
owner-type: org
```

## Contributing

This project follows a Test-Driven Development (TDD) approach with comprehensive test coverage (>80% target).

## License

See [LICENSE](LICENSE) file for details.
