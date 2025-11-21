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
- ⏳ **Iteration 5**: OCI Graph Discovery
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

## Architecture

- **CLI Layer**: Built with [Cobra](https://github.com/spf13/cobra)
- **Configuration**: Managed with [Viper](https://github.com/spf13/viper)
- **GHCR API**: Using [go-github](https://github.com/google/go-github) for package management
- **OCI Layer**: Using [ORAS Go SDK](https://oras.land/docs/category/oras-go-library) for tag resolution

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
