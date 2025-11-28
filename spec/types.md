# Plan: Support Additional OCI Artifact Types

This document outlines the plan to expand ghcrctl's support for OCI artifact types beyond buildx-embedded attestations.

## Current State

ghcrctl currently discovers artifacts stored **within the image index** (Docker buildx style):
- ✅ SBOM attestations (in-toto format via buildx)
- ✅ Provenance attestations (SLSA format via buildx)
- ✅ Platform manifests (multi-arch)

**Not supported:**
- ❌ Cosign signatures
- ❌ Notation signatures
- ❌ Vulnerability scan results
- ❌ External SBOMs attached post-build
- ❌ Any artifact attached via OCI Referrers API

## Goal

Support all common OCI artifact types by implementing the OCI 1.1 Referrers API.

---

## Phase 1: Create Test Images with Additional Artifact Types

### 1.1 Update `prepare_integration_test.yml`

Add workflow steps to create test images with:

#### Cosign Signature
```yaml
- name: Sign image with Cosign
  env:
    COSIGN_EXPERIMENTAL: "true"
  run: |
    cosign sign --yes ghcr.io/${{ github.repository_owner }}/ghcrctl-test-signed:latest
```

#### Vulnerability Scan Result (Trivy)
```yaml
- name: Attach vulnerability scan
  run: |
    trivy image --format cosign-vuln --output vuln.json ghcr.io/${{ github.repository_owner }}/ghcrctl-test-with-sbom:latest
    cosign attest --predicate vuln.json --type vuln ghcr.io/${{ github.repository_owner }}/ghcrctl-test-with-sbom:latest
```

#### External SBOM (Syft via Cosign)
```yaml
- name: Generate and attach SBOM with Syft
  run: |
    syft ghcr.io/${{ github.repository_owner }}/ghcrctl-test-no-sbom:latest -o spdx-json > sbom.json
    cosign attest --predicate sbom.json --type spdxjson ghcr.io/${{ github.repository_owner }}/ghcrctl-test-no-sbom:latest
```

### 1.2 New Test Images

| Image Name | Artifact Types | Purpose |
|------------|----------------|---------|
| `ghcrctl-test-signed` | Cosign signature | Test signature discovery |
| `ghcrctl-test-with-vuln` | Vuln scan + SBOM + provenance | Test multiple referrers |
| `ghcrctl-test-external-sbom` | SBOM via Cosign (not buildx) | Test Referrers API SBOM |

---

## Phase 2: Implement OCI Referrers API Support

### 2.1 Add Referrers Discovery to `internal/oras/resolver.go`

```go
// DiscoverReferrers queries the OCI Referrers API for artifacts attached to a digest.
// Returns artifacts discovered via /v2/<name>/referrers/<digest>
func DiscoverReferrers(ctx context.Context, imageRef string, digest string) ([]ReferrerInfo, error) {
    // 1. Try OCI Referrers API: GET /v2/<name>/referrers/<digest>
    // 2. Fall back to tag schema: GET /v2/<name>/manifests/sha256-<digest>
    // 3. Parse response as OCI Index
    // 4. Return list of referrers with artifactType
}

type ReferrerInfo struct {
    Digest       string
    ArtifactType string // e.g., "application/vnd.dev.cosign.simplesigning.v1+json"
    Annotations  map[string]string
}
```

### 2.2 Artifact Type Mapping

Map OCI artifact types to display names:

```go
var artifactTypeNames = map[string]string{
    "application/vnd.dev.cosign.simplesigning.v1+json": "signature",
    "application/vnd.in-toto+json":                      "attestation",
    "application/spdx+json":                             "sbom",
    "application/vnd.cyclonedx+json":                    "sbom",
    "application/sarif+json":                            "vuln-scan",
    "application/vnd.dev.sigstore.verificationmaterial": "signature",
}
```

### 2.3 Integrate into Graph Building

Update `buildVersionGraphs` in `cmd/versions.go`:

```go
func buildVersionGraphs(ctx context.Context, fullImage string, ...) ([]VersionGraph, error) {
    // Existing: discover children from index manifests array

    // NEW: Also discover referrers for the root digest
    referrers, err := oras.DiscoverReferrers(ctx, fullImage, rootDigest)
    if err == nil {
        for _, ref := range referrers {
            // Add referrer as child with appropriate type
            child := VersionChild{
                Version:      lookupVersionByDigest(ref.Digest),
                ArtifactType: mapArtifactType(ref.ArtifactType),
                IsReferrer:   true, // NEW field to distinguish from index children
            }
            graph.Children = append(graph.Children, child)
        }
    }
}
```

### 2.4 Update VersionChild Structure

```go
type VersionChild struct {
    Version      gh.PackageVersionInfo
    ArtifactType string
    Platform     string
    RefCount     int
    IsReferrer   bool   // NEW: true if discovered via Referrers API
    Subject      string // NEW: digest this artifact refers to (for referrers)
}
```

---

## Phase 3: Display and Delete Support

### 3.1 Display Referrers in `versions` Output

```
  VERSION ID  TYPE         DIGEST        TAGS  CREATED
  ----------  -----------  ------------  ----  -------------------
┌ 585861918   index        01af50cc8b0d  []    2025-01-15 10:30:45
├ 585861919   linux/amd64  62f946a8267d  []    2025-01-15 10:30:44
├ 585861921   sbom         9a1636d22702  []    2025-01-15 10:30:46
├ 585861922   provenance   9a1636d22702  []    2025-01-15 10:30:46
├ 585861923   signature    abc123def456  []    2025-01-15 10:31:00  ← NEW (via Referrers API)
└ 585861924   vuln-scan    def456abc123  []    2025-01-15 10:32:00  ← NEW (via Referrers API)
```

### 3.2 Include Referrers in `delete graph`

When deleting a graph, also delete referrers that point to versions being deleted:

```go
func collectVersionsToDelete(graph *VersionGraph) []int64 {
    ids := []int64{graph.RootVersion.ID}

    for _, child := range graph.Children {
        // Include both index children and referrers
        ids = append(ids, child.Version.ID)
    }

    return ids
}
```

### 3.3 Color Coding for Referrers

Add new color for referrer-based artifacts (to distinguish from index children):

```go
// In internal/display/color.go
colorReferrer = color.New(color.FgMagenta) // For signature, vuln-scan, etc.
```

---

## Phase 4: Testing

### 4.1 Unit Tests

- `TestDiscoverReferrers` - Mock Referrers API responses
- `TestArtifactTypeMapping` - Verify type name resolution
- `TestGraphBuildingWithReferrers` - Combined index + referrers

### 4.2 Integration Tests

- Verify signature discovery on `ghcrctl-test-signed`
- Verify vuln scan discovery on `ghcrctl-test-with-vuln`
- Verify external SBOM discovery on `ghcrctl-test-external-sbom`
- Verify `delete graph` removes referrers

---

## Implementation Order

1. **Phase 1.1**: Update workflow to create signed test image
2. **Phase 2.1**: Implement `DiscoverReferrers` function
3. **Phase 2.2**: Add artifact type mapping
4. **Phase 2.3**: Integrate into graph building
5. **Phase 3.1**: Update display to show referrers
6. **Phase 1.2**: Add remaining test images (vuln, external SBOM)
7. **Phase 3.2**: Update delete to handle referrers
8. **Phase 4**: Add tests

---

## Notes

### GHCR Referrers API Support

GitHub Container Registry supports the OCI Referrers API as of 2024. Verify with:
```bash
curl -H "Authorization: Bearer $GITHUB_TOKEN" \
  https://ghcr.io/v2/<owner>/<image>/referrers/sha256:<digest>
```

### Fallback Tag Schema

For registries without Referrers API, cosign uses tag-based fallback:
- Tag format: `sha256-<digest>.sig` for signatures
- Tag format: `sha256-<digest>.att` for attestations

ghcrctl should support both discovery methods.

### Shared Referrers

Referrers can potentially be shared (e.g., same signature for multiple digests). The `RefCount` mechanism should apply to referrers as well.
