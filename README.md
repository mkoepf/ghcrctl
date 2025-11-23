# ghcrctl

[![CI](https://github.com/mkoepf/ghcrctl/actions/workflows/ci.yml/badge.svg)](https://github.com/mkoepf/ghcrctl/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/mkoepf/ghcrctl)](https://goreportcard.com/report/github.com/mkoepf/ghcrctl)
[![codecov](https://codecov.io/gh/mkoepf/ghcrctl/branch/main/graph/badge.svg)](https://codecov.io/gh/mkoepf/ghcrctl)

A command-line tool for interacting with GitHub Container Registry (GHCR).

## Features

ghcrctl provides functionality for:

- **Exploring images** and their OCI artifact graph (image, SBOM, provenance)
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

Display the OCI artifact graph for an image, including SBOM and provenance attestations:

```bash
ghcrctl graph myimage
```

This command discovers:
- **SBOM (Software Bill of Materials)** - SPDX and CycloneDX formats
- **Provenance** - SLSA provenance attestations
- **Multi-platform attestations** - Supports Docker buildx multi-layer attestations

Specify a tag (default is `latest`):

```bash
ghcrctl graph myimage --tag v1.0.0
```

Get output in JSON format:

```bash
ghcrctl graph myimage --json
```

**Example output:**
```
OCI Artifact Graph for myimage

Image:
  Digest: sha256:1234567890abcdef...
  Tags: [v1.0.0]
  Version ID: 12345

Referrers:

  sbom:
    Digest: sha256:abcdef1234567890...

  provenance:
    Digest: sha256:fedcba0987654321...

Summary:
  SBOM: true
  Provenance: true
  Total artifacts: 3
```

### Getting Help

```bash
ghcrctl --help
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

# Without GITHUB_TOKEN, integration tests are skipped
unset GITHUB_TOKEN
go test ./...  # Integration tests will be skipped
```

#### Limitations

The scoped token used in the integration tests allows to test

- Tag resolution (latest, semantic versions, multiple tags)
- SBOM and provenance discovery
- Multi-layer attestation detection (Docker buildx pattern)
- Graph command end-to-end functionality
- JSON and table output formats

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
