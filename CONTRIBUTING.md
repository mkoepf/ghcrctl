# Contributing to ghcrctl

## Development Setup

```bash
git clone https://github.com/mkoepf/ghcrctl.git
cd ghcrctl
go build -o ghcrctl .
```

Requires Go 1.23 or later.

## Development Guideline

Always include meaningful tests with your change.

That's really it.

## Running Tests

```bash
# All tests
go test ./...

# With coverage
go test ./... -cover
```

### Integration Tests

Integration tests run against real GHCR images and require `GITHUB_TOKEN`:

```bash
export GITHUB_TOKEN=ghp_...
go test ./... -run Integration
```

Without a token, integration tests are skipped locally but must run in CI.

### Mutating Tests

Tests that modify GHCR state (tag add, delete) are in `*_mutating_test.go` files:

```bash
go test ./... -tags mutating
```

## Code Quality Requirements

Before submitting:

- `gofmt` - Code must be formatted
- `go vet` - No suspicious constructs
- `gosec` - No security issues
- `govulncheck` - No known vulnerabilities
- `trivy` - No filesystem vulnerabilities
- All tests pass with no skips

There is a Mac-tested convenience script for running all these checks:

```bash
# Full quality checks (format, vet, tests, security scans)
./scripts/code_quality.sh
```

## Commit Messages

- Use concise, descriptive messages
- Focus on the "why" when helpful

## Pull Requests

1. Fork the repository
2. Create a feature branch from `main`
3. Make your changes
4. Ensure the pipeline passes (you can test this locally with `./scripts/code_quality.sh` 
5. Submit a PR with a clear description of the changes

## Reporting Issues

Open an issue at https://github.com/mkoepf/ghcrctl/issues with:

- What you expected to happen
- What actually happened
- Steps to reproduce
- ghcrctl version (`ghcrctl --version`)
