# ghcrctl -- CLI Requirements & Specification (No TUI)

## 1. Purpose

`ghcrctl` is a command-line tool for interacting with GitHub Container
Registry (GHCR), providing: 

- Exploration of images and their OCI artifact graph (image, SBOM, provenance). 
- Management of GHCR version metadata (labels, tags). 
- Safe deletion of package versions. 
- Configuration of owner/org and authentication.

It is assumed that GitHub tokens are present. Authentication is out of scope,
for now!

------------------------------------------------------------------------

## 2. High-Level Goals

-   Provide a unified CLI interface that:
    -   Inspects GHCR packages.
    -   Resolves OCI graphs via ORAS API.
    -   Manages GHCR metadata via GitHub REST API.
-   Enable retention workflows (e.g., marking versions to be kept).
-   Support interactive mode using simple stdin questions (yes/no,
    select from list).
-   Keep output machine-readable (optional `--json` flag).
-   Provide predictable scripting and CI integration.

------------------------------------------------------------------------

## 3. Architecture Overview

### Components

1.  **CLI Layer (Cobra)**
    -   Commands: `images`, `graph`, `tag`, `label`, `delete`, `config`.
    -   Optional interactive prompts (non-TUI) with default lists
        printed.
2.  **GHCR API Layer**
    -   Implemented using the GitHub REST API via go-github.
    -   Manages package versions, metadata, and deletion.
3.  **OCI Layer (ORAS SDK)**
    -   Resolves tag → digest.
    -   Discovers SBOM & provenance referrers.
    -   Traverses OCI artifact graph.
4.  **Configuration System**
    -   Based on Viper.
    -   Stores owner/org configuration, token (optional), and defaults.
5.  **Logging and Output**
    -   Human-readable output for interactive mode.
    -   JSON for automation.

------------------------------------------------------------------------

## 4. CLI Commands

### 4.1 `ghcrctl config`

Manage configuration stored in `~/.ghcrctl/config.yaml`.

#### Subcommands

-   `ghcrctl config show`\
    Prints configuration.
-   `ghcrctl config org <org-or-user>`\
    Sets the GHCR owner.

------------------------------------------------------------------------

### 4.2 `ghcrctl images`

List container images owned by the configured org/user.

#### Behavior

-   Display list in alphabetical order.
-   In interactive mode (`--interactive`), prompt:

```{=html}
<!-- -->
```
    Select image:
    1) vectorstore
    2) embedder
    3) e2e-testrunner
    Selection:

#### Options

-   `--json` outputs structured list.

------------------------------------------------------------------------

### 4.3 `ghcrctl graph <image>`

Display the OCI artifact graph for an image.

#### Behavior

-   Resolve `<image>:latest` unless tag given.
-   Map tag → digest via ORAS.
-   Discover referrers (sbom, provenance).
-   Retrieve GHCR version IDs matching digests.
-   Print graph:

```{=html}
<!-- -->
```
    Image Digest: sha256:aaa (version 1001, tags: ["v1", "stable"])
    SBOM Digest: sha256:bbb (version 1002)
    Provenance: sha256:ccc (version 1003)

#### Options

-   `--tag <tag>`\
-   `--json`\
-   `--all` (show older digests)

------------------------------------------------------------------------

### 4.4 `ghcrctl tag <image> <existing-tag> <new-tag>`

Add a GHCR tag to an existing version.

#### Behavior

-   Resolve `<existing-tag>` to digest (ORAS).
-   Map digest → GHCR versionID.
-   PATCH metadata to add `<new-tag>`.

------------------------------------------------------------------------

### 4.5 `ghcrctl label <image> <tag> key=value`

Apply a label to: 
- the image version 
- its SBOM artifact 
- its provenance artifact

#### Behavior

1.  Resolve tag → digest.
2.  Get referrers (sbom + provenance).
3.  Map each digest → GHCR versionID.
4.  PATCH metadata for all related versions.

#### Options

-   `--json` output of all patches applied.

------------------------------------------------------------------------

### 4.6 `ghcrctl delete <image>`

Delete versions or whole OCI graph.

#### Subcommands

##### `ghcrctl delete <image> <version-id>`

Deletes a single GHCR version.

##### `ghcrctl delete <image> --untagged`

Deletes all untagged GHCR versions.

##### `ghcrctl delete <image> --older-than <days>`

Deletes versions older than N days.

##### `ghcrctl delete graph <image> <tag>`

Deletes: - the image - its SBOM - its provenance

#### Safety

Always ask:

    Are you sure? [y/N]:

unless `--force`.

------------------------------------------------------------------------

## 5. Configuration File

### Location

`~/.ghcrctl/config.yaml`

### Example

``` yaml
owner: opsonine
```

------------------------------------------------------------------------

## 6. Internal Modules

### `internal/config`

-   Loads and saves config.
-   Exposes:
    -   `GetOwner()`
    -   `SetOwner()`

### `internal/gh`

-   Wrapper around GitHub REST (go-github).
-   Functions:
    -   `ListPackages()`
    -   `ListVersions(package)`
    -   `DeleteVersion(id)`
    -   `PatchMetadata(id, metadata)`
    -   `FindVersionIDByDigest(digest)`
    -   `FindVersionIDByTag(tag)`

### `internal/oras`

-   Resolve tag → digest.
-   Discover referrers.
-   Validate OCI manifests.

### `internal/graph`

-   Build and represent graph struct from:
    -   digest
    -   referrers
    -   GHCR versions

### `internal/prompts`

Simple text-based prompts using stdin: - yes/no - selection from
numbered list

------------------------------------------------------------------------

## 7. Error Handling Requirements

-   Missing owner triggers interactive prompt.
-   Missing token triggers error with instructions.
-   GHCR permission issues must be explained clearly.
-   ORAS resolution failures show image + tag.
-   Deleting protected versions must be blocked unless `--force`.

------------------------------------------------------------------------

## 8. Security Requirements

-   Commands must not print secrets.
-   Dry-run mode for destructive operations.
-   Prevent deleting SBOM/provenance alone (must delete graph).

------------------------------------------------------------------------

## 9. JSON Output Requirements

All commands accept `--json` to print machine-readable output.

Examples: - `images` - `graph` - `tag` - `label` - `delete`

------------------------------------------------------------------------

------------------------------------------------------------------------

## 11. Summary

`ghcrctl` provides a consistent and safe CLI for: - GHCR metadata
management - OCI graph resolution - Labeling, tagging, deleting, and
pruning - Configuration-driven workflows

It avoids TUI complexity while remaining interactive through simple
prompts and predictable command behavior.