# OCI Terminology Reference

This document explains the OCI (Open Container Initiative) terminology used in ghcrctl.

## Manifest Types (Media Types)

There are only **two OCI manifest media types**:

| Type | Media Type | Description |
|------|------------|-------------|
| **Index** | `application/vnd.oci.image.index.v1+json` | A "fat manifest" listing multiple platform-specific manifests |
| **Manifest** | `application/vnd.oci.image.manifest.v1+json` | A single image with config and layers |

## Relationship Types

### References (Parent → Child)

An **Image Index** contains a `manifests` array that **references** child manifests:

```
Index
├── references → Platform Manifest (linux/amd64)
├── references → Platform Manifest (linux/arm64)
├── references → Attestation Manifest (sbom)
└── references → Attestation Manifest (provenance)
```

- Direction: Parent points TO children
- Discovery: Parse the index's `manifests` array
- Use case: Multi-architecture images, buildx attestations

### Referrers (Child → Parent)

A manifest can declare a **subject** field pointing back to another manifest:

```
SBOM Artifact ──refers to──► Image (via subject field)
Signature     ──refers to──► Image (via subject field)
```

- Direction: Child points BACK to parent
- Discovery: OCI Referrers API (`/v2/<name>/referrers/<digest>`)
- Use case: OCI artifacts attached after image creation (signatures, external SBOMs)

## Child Roles

Children discovered within an index have different **roles**:

| Role | Description | Identified By |
|------|-------------|---------------|
| **Platform** | Architecture-specific image (linux/amd64, etc.) | `platform` field in index descriptor |
| **Attestation** | SBOM, provenance, or other metadata | `platform: unknown/unknown` + annotations |

### Attestations

Docker buildx stores attestations **both ways** - as references AND with referrer-like annotations:

1. **As References**: Listed in the index's `manifests` array (like platform manifests)
2. **With Referrer Annotations**: Each attestation has annotations linking it back to the platform manifest it describes

```
Index (manifests array contains all of these)
├── Platform Manifest (linux/amd64)
├── Platform Manifest (linux/arm64)
├── Attestation (sbom) ──annotation──► linux/amd64
├── Attestation (sbom) ──annotation──► linux/arm64
├── Attestation (provenance) ──annotation──► linux/amd64
└── Attestation (provenance) ──annotation──► linux/arm64
```

Attestation manifest properties:
- Media type: `application/vnd.oci.image.manifest.v1+json` (same as regular images)
- Platform: Set to `unknown/unknown` to prevent execution
- Annotations:
  - `vnd.docker.reference.type: attestation-manifest`
  - `vnd.docker.reference.digest: <digest of platform manifest>`
- Content: Layers contain `application/vnd.in-toto+json` blobs

This hybrid approach means attestations are discoverable both by parsing the index (forward reference) and by checking which attestations point to a given platform manifest (backward annotation).

Common attestation types:
- **sbom** - Software Bill of Materials (SPDX format)
- **provenance** - SLSA build provenance

## Why We Use a Single TYPE Column

The `versions` command displays a single TYPE column showing the artifact's **role**:

```
  VERSION ID  TYPE         DIGEST        TAGS      CREATED
  ----------  -----------  ------------  --------  -------------------
┌ 585861918   index        01af50cc8b0d  [v1.0.0]  2025-01-15 10:30:45
├ 585861919   linux/amd64  62f946a8267d  []        2025-01-15 10:30:44
├ 585861920   linux/arm64  89c3b5f1a432  []        2025-01-15 10:30:44
├ 585861921   sbom         9a1636d22702  []        2025-01-15 10:30:46
└ 585861922   provenance   9a1636d22702  []        2025-01-15 10:30:46

  592195820   manifest     8f27ad82912a  []        2025-11-27 23:31:52
```

Rationale:
1. **All children are manifests** - Showing "manifest" for every child would be redundant
2. **Role is more useful** - Users care whether it's a platform or attestation
3. **Simpler display** - One column instead of two (media type + role)
4. **Root distinction matters** - Only roots need index/manifest distinction

The TYPE column shows:
- `index` - Root is a multi-platform image index
- `manifest` - Root is a standalone single-platform image
- `linux/amd64`, etc. - Platform manifest referenced by index
- `sbom`, `provenance` - Attestation referenced by index

## References

- [OCI Image Spec - Manifest](https://github.com/opencontainers/image-spec/blob/main/manifest.md)
- [OCI Image Spec - Index](https://github.com/opencontainers/image-spec/blob/main/image-index.md)
- [OCI Distribution Spec 1.1 - Referrers](https://opencontainers.org/posts/blog/2024-03-13-image-and-distribution-1-1/)
- [Docker Attestation Storage](https://docs.docker.com/build/metadata/attestations/attestation-storage/)
- [ORAS Attached Artifacts](https://oras.land/docs/concepts/reftypes/)
