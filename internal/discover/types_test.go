package discover

import (
	"encoding/json"
	"testing"
)

func TestVersionInfo_JSONSerialization(t *testing.T) {
	v := VersionInfo{
		ID:           123456,
		Digest:       "sha256:abc123",
		Tags:         []string{"v1.0.0", "latest"},
		Types:        []string{"index"},
		OutgoingRefs: []string{"sha256:def456"},
		IncomingRefs: []string{"sha256:ghi789"},
		CreatedAt:    "2025-01-15 10:30:45",
	}

	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("failed to marshal VersionInfo: %v", err)
	}

	var decoded VersionInfo
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal VersionInfo: %v", err)
	}

	if decoded.ID != v.ID {
		t.Errorf("ID mismatch: got %d, want %d", decoded.ID, v.ID)
	}
	if decoded.Digest != v.Digest {
		t.Errorf("Digest mismatch: got %s, want %s", decoded.Digest, v.Digest)
	}
	if len(decoded.Tags) != len(v.Tags) {
		t.Errorf("Tags length mismatch: got %d, want %d", len(decoded.Tags), len(v.Tags))
	}
}

func TestVersionInfo_IsReferrer(t *testing.T) {
	tests := []struct {
		name     string
		types    []string
		expected bool
	}{
		{"index is not referrer", []string{"index"}, false},
		{"manifest is not referrer", []string{"manifest"}, false},
		{"platform is not referrer", []string{"linux/amd64"}, false},
		{"signature is referrer", []string{"signature"}, true},
		{"sbom is referrer", []string{"sbom"}, true},
		{"provenance is referrer", []string{"provenance"}, true},
		{"vuln-scan is referrer", []string{"vuln-scan"}, true},
		{"vex is referrer", []string{"vex"}, true},
		{"attestation is referrer", []string{"attestation"}, true},
		{"multi-type with signature", []string{"sbom", "provenance"}, true},
		{"empty types", []string{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := VersionInfo{Types: tt.types}
			if got := v.IsReferrer(); got != tt.expected {
				t.Errorf("IsReferrer() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestVersionInfo_IsRoot(t *testing.T) {
	// Create a version map for testing
	allVersions := map[string]VersionInfo{
		"sha256:index1": {
			Digest: "sha256:index1",
			Types:  []string{"index"},
		},
		"sha256:platform1": {
			Digest:       "sha256:platform1",
			Types:        []string{"linux/amd64"},
			IncomingRefs: []string{"sha256:index1"},
		},
		"sha256:sbom1": {
			Digest:       "sha256:sbom1",
			Types:        []string{"sbom"},
			OutgoingRefs: []string{"sha256:index1"},
		},
		"sha256:sbom_buildx": {
			Digest:       "sha256:sbom_buildx",
			Types:        []string{"sbom"},
			IncomingRefs: []string{"sha256:index1"}, // buildx-style: index points to sbom
		},
		"sha256:orphan_sig": {
			Digest:       "sha256:orphan_sig",
			Types:        []string{"signature"},
			OutgoingRefs: []string{"sha256:deleted"},
		},
		"sha256:standalone": {
			Digest: "sha256:standalone",
			Types:  []string{"manifest"},
		},
	}

	tests := []struct {
		name     string
		digest   string
		expected bool
	}{
		{"index is always root", "sha256:index1", true},
		{"platform connected to index is not root", "sha256:platform1", false},
		{"sbom referencing index is not root", "sha256:sbom1", false},
		{"sbom embedded in index is not root", "sha256:sbom_buildx", false},
		{"orphan signature is root", "sha256:orphan_sig", true},
		{"standalone manifest is root", "sha256:standalone", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := allVersions[tt.digest]
			if got := v.IsRoot(allVersions); got != tt.expected {
				t.Errorf("IsRoot() = %v, want %v", got, tt.expected)
			}
		})
	}
}
