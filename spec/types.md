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

**Supported attestation types**: SBOM (SPDX), Provenance (SLSA) only

### Method 2: Cosign Tag Fallback (Separate Manifests)

Cosign stores artifacts as **separate manifests** with predictable tag names:

```
ghcr.io/owner/image:latest                           → Image Index
ghcr.io/owner/image:sha256-<digest>.sig              → Signature
ghcr.io/owner/image:sha256-<digest>.att              → Attestation(s)
```

**Discovery**: List all tags → pattern match `sha256-<digest>.sig` and `sha256-<digest>.att` → resolve to version IDs

**Key difference**: All attestation types share the same `.att` tag. The **predicate type** inside distinguishes them.

---

## Complete Attestation Type Catalog

### Signatures (`.sig` tags only)

| Type | Media Type | Description |
|------|------------|-------------|
| signature | `application/vnd.dev.cosign.simplesigning.v1+json` | Cosign cryptographic signature |
| signature | `application/vnd.dev.sigstore.verificationmaterial` | Sigstore verification material |

### Attestations (`.att` tags or buildx index)

All attestations use the in-toto envelope format. The `predicateType` field determines the specific type:

| Display Type | Predicate Type URI | Created By |
|--------------|-------------------|------------|
| sbom | `https://spdx.dev/Document` | buildx, cosign attest --type spdx |
| sbom | CycloneDX schema URI | cosign attest --type cyclonedx |
| provenance | `https://slsa.dev/provenance/v0.2` | buildx, cosign attest --type slsaprovenance |
| provenance | `https://slsa.dev/provenance/v1` | cosign attest --type slsaprovenance1 |
| vuln-scan | `https://cosign.sigstore.dev/attestation/vuln/v1` | cosign attest --type vuln |
| vex | `https://openvex.dev/ns` | vexctl, cosign attest --type openvex |
| attestation | `https://cosign.sigstore.dev/attestation/v1` | cosign attest --type custom |
| attestation | Any custom URI | cosign attest --type <custom-uri> |

### Predicate Type to Display Type Mapping

```go
func predicateTypeToDisplayType(predicateType string) string {
    switch {
    case strings.Contains(predicateType, "spdx") || strings.Contains(predicateType, "cyclonedx"):
        return "sbom"
    case strings.Contains(predicateType, "slsa") || strings.Contains(predicateType, "provenance"):
        return "provenance"
    case strings.Contains(predicateType, "vuln"):
        return "vuln-scan"
    case strings.Contains(predicateType, "openvex") || strings.Contains(predicateType, "vex"):
        return "vex"
    default:
        return "attestation"
    }
}
```

---

## Implementation Plan

### Phase 1: Artifact Type Mapping ✅ DONE

Map OCI media types to human-readable display names.

**Status**: Implemented in commit `49169e9`.

### Phase 2: Display Color Coding ✅ DONE

| Type | Color | Used for |
|------|-------|----------|
| index | Cyan | Multi-arch image indexes |
| platform | Green | Platform-specific manifests (linux/amd64, etc.) |
| sbom | Yellow | SBOM attestations |
| provenance | Yellow | Provenance attestations |
| attestation | Yellow | Generic/custom attestations |
| signature | Magenta | Cosign/sigstore signatures |
| vuln-scan | Magenta | Vulnerability scan results |
| vex | Magenta | VEX documents (to be added) |

**Status**: Implemented in commit `74d52f4`.

### Phase 3: Cosign Tag Discovery 🔲 TODO

Implement discovery of cosign artifacts via tag pattern matching.

#### 3.1 Signature Discovery (Simple)

```go
// For .sig tags - type is always "signature"
func discoverSignatures(tags []string, parentDigest string) []ReferrerInfo {
    prefix := "sha256-" + strings.TrimPrefix(parentDigest, "sha256:")
    sigTag := prefix + ".sig"

    for _, tag := range tags {
        if tag == sigTag {
            return []ReferrerInfo{{
                Digest:       resolveTagToDigest(sigTag),
                ArtifactType: "signature",
            }}
        }
    }
    return nil
}
```

#### 3.2 Attestation Discovery (Complex)

For `.att` tags, we must fetch and parse to determine type:

```go
// For .att tags - must inspect predicateType
func discoverAttestations(ctx context.Context, image string, tags []string, parentDigest string) []ReferrerInfo {
    prefix := "sha256-" + strings.TrimPrefix(parentDigest, "sha256:")
    attTag := prefix + ".att"

    for _, tag := range tags {
        if tag == attTag {
            // Fetch attestation manifest
            manifest := fetchManifest(ctx, image, attTag)

            // Parse in-toto envelope from first layer
            envelope := parseInTotoEnvelope(manifest.Layers[0])

            // Map predicate type to display type
            displayType := predicateTypeToDisplayType(envelope.PayloadType)

            return []ReferrerInfo{{
                Digest:       manifest.Digest,
                ArtifactType: displayType,
            }}
        }
    }
    return nil
}
```

#### 3.3 Unified Discovery Function

```go
func DiscoverAllReferrers(ctx context.Context, image, digest string, allTags []string) ([]ReferrerInfo, error) {
    var referrers []ReferrerInfo

    // Method 1: Buildx-style (inside image index)
    buildxReferrers, _ := discoverAttestationsInIndex(ctx, image, digest)
    referrers = append(referrers, buildxReferrers...)

    // Method 2: Cosign signatures (.sig tags)
    sigReferrers := discoverSignatures(allTags, digest)
    referrers = append(referrers, sigReferrers...)

    // Method 3: Cosign attestations (.att tags) - requires fetch
    attReferrers, _ := discoverAttestations(ctx, image, allTags, digest)
    referrers = append(referrers, attReferrers...)

    return referrers, nil
}
```

### Phase 4: Add VEX Color Support 🔲 TODO

Add "vex" to the color mapping:

```go
case lower == "vex":
    return colorVex.Sprint(versionType)  // Magenta
```

### Phase 5: Test Images 🔲 TODO

Create test images with various artifact types:

```yaml
# Signature
cosign sign --key cosign.key ghcr.io/.../image@sha256:...

# Vulnerability scan attestation
trivy image --format cosign-vuln -o vuln.json ghcr.io/.../image
cosign attest --type vuln --predicate vuln.json ghcr.io/.../image@sha256:...

# VEX attestation
vexctl create --product ghcr.io/.../image@sha256:... --vuln CVE-2024-1234 --status not_affected
cosign attest --type openvex --predicate vex.json ghcr.io/.../image@sha256:...
```

### Phase 6: Documentation ✅ DONE

- See [docs/cosign-signature-storage.md](../docs/cosign-signature-storage.md)

---

## Expected Output After Implementation

```
Versions for my-image:

  VERSION ID  TYPE         DIGEST        TAGS                            CREATED
  ----------  -----------  ------------  ------------------------------  -------------------
┌ 585861918   index        01af50cc8b0d  [latest]                        2025-01-15 10:30:45
├ 585861919   linux/amd64  62f946a8267d  []                              2025-01-15 10:30:44
├ 585861921   sbom         9a1636d22702  []                              2025-01-15 10:30:46  ← buildx
├ 585861922   provenance   ba978d8b2184  []                              2025-01-15 10:30:46  ← buildx
├ 585861923   signature    abc123def456  [sha256-01af50cc...sig]         2025-01-15 10:31:00  ← cosign
├ 585861924   vuln-scan    def456abc123  [sha256-01af50cc...att]         2025-01-15 10:32:00  ← cosign
└ 585861925   vex          fed789abc012  [sha256-01af50cc...att]         2025-01-15 10:33:00  ← cosign
```

---

## Summary

| Phase | Description | Status |
|-------|-------------|--------|
| 1 | Artifact type mapping (media type → display name) | ✅ Done |
| 2 | Display color coding for new types | ✅ Done |
| 3 | Cosign tag discovery (sig + att) | 🔲 To implement |
| 4 | Add VEX color support | 🔲 To implement |
| 5 | Test images with various artifacts | 🔲 To implement |
| 6 | Documentation | ✅ Done |

## References

- [Cosign Signature Spec](https://github.com/sigstore/cosign/blob/main/specs/SIGNATURE_SPEC.md)
- [Cosign Vulnerability Attestation Spec](https://github.com/sigstore/cosign/blob/main/specs/COSIGN_VULN_ATTESTATION_SPEC.md)
- [In-Toto Attestations - Sigstore](https://docs.sigstore.dev/cosign/verifying/attestation/)
- [OpenVEX - OpenSSF](https://openssf.org/projects/openvex/)
- [vexctl - OpenVEX tool](https://github.com/openvex/vexctl)
- [Building towards OCI v1.1 support in cosign](https://www.chainguard.dev/unchained/building-towards-oci-v1-1-support-in-cosign)
- [docs/cosign-signature-storage.md](../docs/cosign-signature-storage.md)
