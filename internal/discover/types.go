package discover

// VersionInfo contains information about a package version with reference relationships.
type VersionInfo struct {
	ID           int64    `json:"id"`
	Digest       string   `json:"digest"`
	Tags         []string `json:"tags"`
	Types        []string `json:"types"`
	Size         int64    `json:"size"`
	OutgoingRefs []string `json:"outgoing_refs"`
	IncomingRefs []string `json:"incoming_refs"`
	CreatedAt    string   `json:"created_at"`
}

// IsReferrer returns true if this version is a signature or attestation type.
func (v VersionInfo) IsReferrer() bool {
	for _, t := range v.Types {
		switch t {
		case "signature", "sbom", "provenance", "vuln-scan", "vex", "attestation":
			return true
		}
	}
	return false
}

// IsRoot returns true if this version should be displayed as a root in tree view.
// Rules:
// 1. Index is always a root
// 2. Referrers are roots only if orphaned (no connection to existing versions)
// 3. Everything else is root only if not connected to an index
func (v VersionInfo) IsRoot(allVersions map[string]VersionInfo) bool {
	// Rule 1: Index is always a root
	for _, t := range v.Types {
		if t == "index" {
			return true
		}
	}

	// Rule 2: Referrers are roots only if orphaned (no connections at all)
	// Check both outgoing AND incoming refs
	if v.IsReferrer() {
		for _, outDigest := range v.OutgoingRefs {
			if _, ok := allVersions[outDigest]; ok {
				return false
			}
		}
		for _, inDigest := range v.IncomingRefs {
			if _, ok := allVersions[inDigest]; ok {
				return false
			}
		}
		return true
	}

	// Rule 3: Other types are roots only if not connected to an index
	for _, outDigest := range v.OutgoingRefs {
		if outVer, ok := allVersions[outDigest]; ok {
			for _, t := range outVer.Types {
				if t == "index" {
					return false
				}
			}
		}
	}
	for _, inDigest := range v.IncomingRefs {
		if inVer, ok := allVersions[inDigest]; ok {
			for _, t := range inVer.Types {
				if t == "index" {
					return false
				}
			}
		}
	}
	return true
}
