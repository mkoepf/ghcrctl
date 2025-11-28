# Cosign Artifact Storage on GHCR

This document describes how cosign stores signatures and attestations on container registries, with specific focus on GitHub Container Registry (GHCR) limitations.

## Overview

Cosign supports two methods for storing artifacts:

1. **OCI 1.1 Referrers API** - The modern approach using the `/v2/<name>/referrers/<digest>` endpoint
2. **Tag Fallback Schema** - A legacy approach using specially-named tags

## GHCR Limitation

**GHCR does not support the OCI 1.1 Referrers API.**

When querying the referrers endpoint:
```bash
curl -H "Authorization: Bearer $TOKEN" \
  "https://ghcr.io/v2/<owner>/<image>/referrers/sha256:<digest>"
```

GHCR returns:
- `303 See Other` redirect to `https://github.com/-/v2/packages/...`
- Followed by `404 Not Found`

This means cosign falls back to the **tag-based storage scheme** on GHCR.

## Tag Fallback Schema

When cosign cannot use the Referrers API, it stores artifacts using predictable tag names:

| Artifact Type | Tag Pattern | Description |
|---------------|-------------|-------------|
| Signature | `sha256-<digest>.sig` | Cryptographic signature of the image |
| Attestation | `sha256-<digest>.att` | In-toto attestation (SBOM, vuln scan, provenance, etc.) |

### Example

For an image with digest `sha256:3580c7c803924ef3b6a15e45b1defc8f9a2bb5f0a61f0d0f0e8c7ce458c6b08c`:

- **Signature**: `ghcr.io/<owner>/<image>:sha256-3580c7c803924ef3b6a15e45b1defc8f9a2bb5f0a61f0d0f0e8c7ce458c6b08c.sig`
- **Attestation**: `ghcr.io/<owner>/<image>:sha256-3580c7c803924ef3b6a15e45b1defc8f9a2bb5f0a61f0d0f0e8c7ce458c6b08c.att`

### How it appears in GHCR

Each cosign artifact becomes a **separate package version** with:
- Its own version ID
- Its own digest
- A tag matching the pattern above (e.g., `sha256-3580c7c8....sig`)

**Important**: GHCR does not expose any relationship between the artifact and the signed image. The relationship is only implied by the tag name containing the parent digest.

## Signatures vs Attestations

### Signatures (`.sig` tags)

Created with `cosign sign`:
```bash
cosign sign --key cosign.key ghcr.io/owner/image@sha256:...
```

- Simple cryptographic signature
- Proves image was signed by key holder
- Media type: `application/vnd.dev.cosign.simplesigning.v1+json`

### Attestations (`.att` tags)

Created with `cosign attest`:
```bash
cosign attest --type <type> --predicate <file> ghcr.io/owner/image@sha256:...
```

All attestation types share the same `.att` tag. The **predicate type** inside the attestation distinguishes them:

| Type Flag | Predicate URI | Use Case |
|-----------|---------------|----------|
| `vuln` | `https://cosign.sigstore.dev/attestation/vuln/v1` | Vulnerability scan results |
| `spdx` / `spdxjson` | `https://spdx.dev/Document` | SPDX SBOM |
| `cyclonedx` | CycloneDX schema | CycloneDX SBOM |
| `slsaprovenance` | `https://slsa.dev/provenance/v0.2` | SLSA provenance |
| `openvex` | OpenVEX schema | Vulnerability exploitability |
| `custom` | User-defined URI | Custom attestations |

### Determining Attestation Type

To determine what type of attestation is in a `.att` tag:
1. Fetch the attestation manifest
2. Parse the in-toto envelope in the layer
3. Read the `predicateType` field

## Comparison with Docker Buildx Attestations

| Aspect | Docker Buildx | Cosign (Tag Fallback) |
|--------|---------------|----------------------|
| Storage location | Inside image index | Separate manifest with special tag |
| Parent relationship | Explicit (manifests array) | Implicit (tag name contains parent digest) |
| Discovery method | Parse image index | Pattern match on tags |
| GHCR version relationship | Child of index version | Independent version |
| Attestation types | SBOM, provenance only | Any in-toto predicate |

## Discovery Approaches

### Current: Buildx-style (implemented in ghcrctl)

ghcrctl discovers attestations by:
1. Fetching the image index
2. Parsing the `manifests` array
3. Identifying attestations by platform `unknown/unknown` or annotation `vnd.docker.reference.type: attestation-manifest`

### Future: Tag-based (not yet implemented)

To discover cosign artifacts, ghcrctl would need to:
1. List all tags for an image
2. Pattern match for `sha256-<digest>.sig` and `sha256-<digest>.att`
3. Extract the parent digest from the tag name
4. For `.att` tags: fetch and parse to determine predicate type
5. Associate the artifact version with its parent image

## References

- [Cosign Signature Spec](https://github.com/sigstore/cosign/blob/main/specs/SIGNATURE_SPEC.md)
- [Cosign Vulnerability Attestation Spec](https://github.com/sigstore/cosign/blob/main/specs/COSIGN_VULN_ATTESTATION_SPEC.md)
- [Trivy Vulnerability Scan Attestation](https://trivy.dev/v0.62/tutorials/signing/vuln-attestation/)
- [Building towards OCI v1.1 support in cosign](https://www.chainguard.dev/unchained/building-towards-oci-v1-1-support-in-cosign)
- [Cosign 2.0 Released](https://blog.sigstore.dev/cosign-2-0-released/)
- [OCI 1.1 Referrers API Support Issue](https://github.com/sigstore/cosign/issues/4335)
