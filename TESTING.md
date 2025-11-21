# Testing Guide - Iteration 1

This guide shows you how to test the currently implemented features of ghcrctl.

## Prerequisites

- Go 1.21 or later installed
- Terminal/command line access

---

## Build the Tool

From the project root directory:

```bash
go build -o ghcrctl .
```

This creates the `ghcrctl` binary in the current directory.

---

## Manual Testing

### 1. Test Help Command

```bash
./ghcrctl --help
```

**Expected Output:** Shows the main help text with available commands (config, help, completion).

### 2. Test Config Help

```bash
./ghcrctl config --help
```

**Expected Output:** Shows config subcommands (show, org).

### 3. Show Configuration (Empty State)

```bash
./ghcrctl config show
```

**Expected Output:**
```
No configuration found.
Set an organization with: ghcrctl config org <org-name>
Set a user with: ghcrctl config user <user-name>
```

### 4. Set an Organization Owner

```bash
./ghcrctl config org mycompany
```

**Expected Output:**
```
Successfully set owner to organization: mycompany
```

### 5. Show Configuration (After Setting Org)

```bash
./ghcrctl config show
```

**Expected Output:**
```
owner-name: mycompany
owner-type: org
```

### 6. Set a User Owner

```bash
./ghcrctl config user johndoe
```

**Expected Output:**
```
Successfully set owner to user: johndoe
```

### 7. Show Configuration (After Setting User)

```bash
./ghcrctl config show
```

**Expected Output:**
```
owner-name: johndoe
owner-type: user
```

### 8. Verify Config File

```bash
cat ~/.ghcrctl/config.yaml
```

**Expected Output:**
```yaml
owner-name: johndoe
owner-type: user
```

---

## Automated Tests

### Run All Tests

```bash
go test ./...
```

**Expected Output:** All tests pass (6 tests in internal/config).

### Run Tests with Verbose Output

```bash
go test ./... -v
```

Shows detailed test execution for each test case.

### Check Test Coverage

```bash
go test ./internal/config -cover
```

**Expected Output:** `coverage: 83.9% of statements` (or similar).

### Detailed Coverage Report

```bash
go test ./internal/config -coverprofile=coverage.out
go tool cover -html=coverage.out
```

Opens an HTML report showing which lines are covered by tests.

---

## Cleanup

### Remove Test Configuration

```bash
rm -rf ~/.ghcrctl
```

This removes the config directory and file created during testing.

### Remove Built Binary

```bash
rm ./ghcrctl
```

Or simply rebuild when needed.

---

## What's Implemented

- ✅ Configuration management (read/write to `~/.ghcrctl/config.yaml`)
- ✅ `config show` - Display current configuration (owner-name and owner-type)
- ✅ `config org <name>` - Set GHCR owner as an organization
- ✅ `config user <name>` - Set GHCR owner as a user
- ✅ Automatic config directory creation
- ✅ Input validation and error handling (validates org/user types)

## What's NOT Implemented Yet

- GitHub API integration (Iteration 2)
- Image listing (Iteration 3)
- OCI/ORAS operations (Iterations 4-5)
- Tagging/labeling (Iterations 6-7)
- Deletion operations (Iterations 8-9)

---

## Troubleshooting

### "command not found: ghcrctl"

Make sure to prefix with `./` when running from the current directory:
```bash
./ghcrctl config show
```

### "permission denied"

Make the binary executable:
```bash
chmod +x ghcrctl
```

### Tests fail

Ensure you have all dependencies:
```bash
go mod tidy
```

Then run tests again.
