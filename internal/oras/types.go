package oras

// ArtifactType represents the canonical type of an OCI artifact.
// It provides consistent type determination regardless of how the artifact was discovered.
type ArtifactType struct {
	ManifestType string // "index" or "manifest"
	Role         string // "index", "platform", "sbom", "provenance", "signature", "vuln-scan", "attestation", "artifact"
	Platform     string // e.g., "linux/amd64" if Role is "platform"
}

// DisplayType returns the user-facing type string for display in tables and output.
// For platform manifests, it returns the platform string (e.g., "linux/amd64").
// For other types, it returns the role directly.
func (t ArtifactType) DisplayType() string {
	switch t.Role {
	case "platform":
		return t.Platform
	case "index":
		return "index"
	default:
		return t.Role
	}
}

// IsAttestation returns true if this artifact is an attestation type
// (sbom, provenance, vuln-scan, or generic attestation).
func (t ArtifactType) IsAttestation() bool {
	switch t.Role {
	case "sbom", "provenance", "attestation", "vuln-scan":
		return true
	default:
		return false
	}
}

// IsPlatform returns true if this artifact is a platform-specific manifest.
func (t ArtifactType) IsPlatform() bool {
	return t.Role == "platform"
}
