# Plan: Unified OCI Artifact Type Recognition

This document outlines the plan to ensure ghcrctl correctly recognizes and displays all OCI artifact types that may appear on GHCR, using consistent OCI terminology.

## Scope

**Target registry**: GitHub Container Registry (GHCR) only.

**Goal**: Recognize and display artifact types consistently with OCI terminology, and discover artifacts stored via both buildx and cosign methods.

## Current State

### What ghcrctl discovers

ghcrctl discovers artifacts stored **within the image index** (Docker buildx style):
- ✅ SBOM attestations (in-toto format via buildx)
- ✅ Provenance attestations (SLSA format via buildx)
- ✅ Platform manifests (multi-arch images)

### What ghcrctl cannot yet discover

**Cosign tag-based artifacts** (stored outside the image index):
- ❌ Cosign signatures (stored at `sha256-<digest>.sig` tag)
- ❌ Cosign attestations (stored at `sha256-<digest>.att` tag)

### GHCR API Limitation

GHCR does **not** support the OCI 1.1 Referrers API:
- Requests to `/v2/<name>/referrers/<digest>` return 303 redirect → 404

This is why cosign uses the **tag fallback schema** on GHCR.

---

## Artifact Storage Methods on GHCR

### Method 1: Docker Buildx (Inside Image Index)

Buildx stores attestations as manifests **inside the image index**:

```
Image Index (sha256:abc123...)
├── linux/amd64 manifest
├── linux/arm64 manifest
├── sbom attestation (platform: unknown/unknown)
└── provenance attestation (platform: unknown/unknown)
```

**Discovery**: Parse image index → find manifests with `platform: unknown/unknown` or annotation `vnd.docker.reference.type: attestation-manifest`

### Method 2: Cosign Tag Fallback (Separate Manifests)

Cosign stores artifacts as **separate manifests** with predictable tag names:

```
ghcr.io/owner/image:latest                           → Image Index
ghcr.io/owner/image:sha256-<digest>.sig              → Signature
ghcr.io/owner/image:sha256-<digest>.att              → Attestation
```

**Discovery**: List all tags → pattern match `sha256-<digest>.sig` and `sha256-<digest>.att` → resolve to version IDs

---

## Implementation Plan

### Phase 1: Artifact Type Mapping ✅ DONE

Map OCI media types to human-readable display names:

```go
// determineArtifactType in internal/oras/resolver.go
func determineArtifactType(mediaType string) string {
    // SBOM types
    if strings.Contains(mediaType, "spdx") || strings.Contains(mediaType, "cyclonedx") {
        return "sbom"
    }
    // Provenance types
    if strings.Contains(mediaType, "in-toto") || strings.Contains(mediaType, "slsa") {
        return "provenance"
    }
    // Signature types (cosign, sigstore)
    if strings.Contains(mediaType, "cosign") || strings.Contains(mediaType, "sigstore") {
        return "signature"
    }
    // Vulnerability scan types (SARIF)
    if strings.Contains(mediaType, "sarif") {
        return "vuln-scan"
    }
    return "unknown"
}
```

**Status**: Implemented in commit `49169e9`.

### Phase 2: Display Color Coding ✅ DONE

Add color support for new artifact types:

| Type | Color | Used for |
|------|-------|----------|
| index | Cyan | Multi-arch image indexes |
| platform | Green | Platform-specific manifests (linux/amd64, etc.) |
| sbom | Yellow | SBOM attestations |
| provenance | Yellow | Provenance attestations |
| attestation | Yellow | Generic attestations |
| signature | Magenta | Cosign/sigstore signatures |
| vuln-scan | Magenta | Vulnerability scan results |

**Status**: Implemented in commit `74d52f4`.

### Phase 3: Cosign Tag Discovery 🔲 TODO

Implement discovery of cosign artifacts via tag pattern matching.

#### 3.1 Add Tag Pattern Matching Function

```go
// internal/oras/resolver.go

// CosignTagPattern defines patterns for cosign artifacts
var cosignTagPatterns = map[string]string{
    ".sig": "signature",
    ".att": "attestation",
}

// DiscoverCosignArtifacts finds cosign artifacts by matching tag patterns
// Returns artifacts discovered via sha256-<digest>.sig/.att tags
func DiscoverCosignArtifacts(ctx context.Context, image string, tags []string, parentDigest string) ([]ReferrerInfo, error) {
    // 1. Compute expected tag prefix: sha256-<digest without prefix>
    // 2. For each tag, check if it matches sha256-<parentDigest>.(sig|att)
    // 3. For matches, resolve the tag to get artifact info
    // 4. Return list of discovered artifacts
}
```

#### 3.2 Integrate into Graph Building

Update `DiscoverReferrers` or create parallel discovery:

```go
func DiscoverAllReferrers(ctx context.Context, image, digest string, allTags []string) ([]ReferrerInfo, error) {
    var referrers []ReferrerInfo

    // Method 1: Buildx-style (inside image index)
    buildxReferrers, _ := discoverAttestationsInIndex(ctx, image, digest)
    referrers = append(referrers, buildxReferrers...)

    // Method 2: Cosign tag fallback
    cosignReferrers, _ := DiscoverCosignArtifacts(ctx, image, allTags, digest)
    referrers = append(referrers, cosignReferrers...)

    return referrers, nil
}
```

#### 3.3 Update VersionChild (if needed)

Consider adding discovery source tracking:

```go
type VersionChild struct {
    Version  gh.PackageVersionInfo
    Type     oras.ArtifactType
    Size     int64
    RefCount int
    Source   string  // "buildx" or "cosign-tag" - for debugging/display
}
```

### Phase 4: Test Image with Cosign Signature 🔲 TODO

Create a test image with cosign signature for integration testing:

```yaml
# In .github/workflows/prepare_integration_test.yml
- name: Sign image with Cosign
  env:
    COSIGN_PASSWORD: ""
  run: |
    cosign generate-key-pair
    cosign sign --key cosign.key --yes --tlog-upload=false \
      ghcr.io/${{ github.repository_owner }}/ghcrctl-test-signed@${{ steps.push.outputs.digest }}
```

### Phase 5: Documentation ✅ DONE

Document cosign signature storage behavior:
- See [docs/cosign-signature-storage.md](../docs/cosign-signature-storage.md)

---

## Expected Output After Implementation

```
Versions for my-image:

  VERSION ID  TYPE         DIGEST        TAGS                            CREATED
  ----------  -----------  ------------  ------------------------------  -------------------
┌ 585861918   index        01af50cc8b0d  [latest]                        2025-01-15 10:30:45
├ 585861919   linux/amd64  62f946a8267d  []                              2025-01-15 10:30:44
├ 585861921   sbom         9a1636d22702  []                              2025-01-15 10:30:46
├ 585861922   provenance   ba978d8b2184  []                              2025-01-15 10:30:46
├ 585861923   signature    abc123def456  [sha256-01af50cc...sig]         2025-01-15 10:31:00  ← cosign
└ 585861924   attestation  def456abc123  [sha256-01af50cc...att]         2025-01-15 10:32:00  ← cosign
```

---

## Summary

| Phase | Description | Status |
|-------|-------------|--------|
| 1 | Artifact type mapping (media type → display name) | ✅ Done |
| 2 | Display color coding for new types | ✅ Done |
| 3 | Cosign tag discovery | 🔲 To implement |
| 4 | Test image with cosign signature | 🔲 To implement |
| 5 | Documentation | ✅ Done |

## References

- [Cosign Signature Spec](https://github.com/sigstore/cosign/blob/main/specs/SIGNATURE_SPEC.md)
- [Building towards OCI v1.1 support in cosign](https://www.chainguard.dev/unchained/building-towards-oci-v1-1-support-in-cosign)
- [docs/cosign-signature-storage.md](../docs/cosign-signature-storage.md)
