# Plan: Unified OCI Artifact Type Recognition

This document outlines the plan to ensure ghcrctl correctly recognizes and displays all OCI artifact types that may appear on GHCR, using consistent OCI terminology.

## Scope

**Target registry**: GitHub Container Registry (GHCR) only.

**Goal**: Recognize and display artifact types consistently with OCI terminology, even if ghcrctl cannot currently discover all artifact types through GHCR's APIs.

## Current State

### What ghcrctl discovers

ghcrctl discovers artifacts stored **within the image index** (Docker buildx style):
- ✅ SBOM attestations (in-toto format via buildx)
- ✅ Provenance attestations (SLSA format via buildx)
- ✅ Platform manifests (multi-arch images)

### What ghcrctl cannot discover (GHCR limitation)

GHCR does **not** support the OCI 1.1 Referrers API:
- Requests to `/v2/<name>/referrers/<digest>` return 303 redirect → 404

This means ghcrctl cannot discover:
- ❌ Cosign signatures (attached via `cosign sign`)
- ❌ Notation signatures
- ❌ Vulnerability scan results (attached via `cosign attest`)
- ❌ External SBOMs attached post-build (via cosign/oras)
- ❌ Any artifact attached via OCI Referrers API

### Type recognition vs discovery

Even though we can't discover all artifact types, we should recognize them correctly when:
1. Processing manifest media types
2. Displaying artifact information
3. Future GHCR API support

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

### Phase 3: Ensure Type Consistency in ResolveType

Update `ResolveType` in `internal/oras/types.go` to handle signature and vuln-scan types when determining attestation roles.

Current `determineAttestationRole` returns:
- "sbom" for spdx/cyclonedx predicates
- "provenance" for slsa/provenance predicates
- "vuln-scan" for vuln predicates
- "attestation" as default

**Action needed**: Verify that signature artifacts would be correctly identified if they appeared in an image index. Currently, cosign signatures don't appear in buildx-style indexes, so this is future-proofing.

### Phase 4: Documentation

Update documentation to explain:
1. What artifact types ghcrctl can discover (buildx-style only)
2. What artifact types ghcrctl recognizes for display
3. GHCR's lack of OCI 1.1 Referrers API support

---

## Future Considerations

### If GHCR adds Referrers API support

If GHCR implements the OCI 1.1 Referrers API in the future:

1. Update `DiscoverReferrers` to call the Referrers API endpoint
2. Merge discovered referrers with buildx-style attestations
3. Add `IsReferrer` field to `VersionChild` if needed to distinguish discovery method

### Fallback tag schema

Cosign uses a fallback tag schema for registries without Referrers API:
- `sha256-<digest>.sig` for signatures
- `sha256-<digest>.att` for attestations

This is **not** planned for implementation because:
1. These artifacts don't have GHCR version IDs
2. They can't be managed through GitHub's package API
3. Discovery would require listing all tags and pattern matching

---

## Summary

| Phase | Description | Status |
|-------|-------------|--------|
| 1 | Artifact type mapping (media type → display name) | ✅ Done |
| 2 | Display color coding for new types | ✅ Done |
| 3 | Verify ResolveType handles all types | 🔲 To verify |
| 4 | Documentation | 🔲 Pending |

The implementation is **forward-compatible**: when/if GHCR adds support for OCI 1.1 Referrers API or other artifact attachment methods, the type recognition and display code is ready.
