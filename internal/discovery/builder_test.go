package discovery

import (
	"context"
	"testing"

	"github.com/mhk/ghcrctl/internal/gh"
)

// mockGHClient is a mock implementation of gh.Client for testing
type mockGHClient struct {
	versions []gh.PackageVersionInfo
	err      error
}

func (m *mockGHClient) ListPackageVersions(ctx context.Context, ownerType, owner, packageName string) ([]gh.PackageVersionInfo, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.versions, nil
}

func TestGetVersionCache(t *testing.T) {
	tests := []struct {
		name     string
		versions []gh.PackageVersionInfo
		wantErr  bool
	}{
		{
			name: "empty versions list",
			versions: []gh.PackageVersionInfo{},
			wantErr: false,
		},
		{
			name: "single version",
			versions: []gh.PackageVersionInfo{
				{
					ID:   123,
					Name: "sha256:abc123",
					Tags: []string{"v1.0"},
				},
			},
			wantErr: false,
		},
		{
			name: "multiple versions",
			versions: []gh.PackageVersionInfo{
				{
					ID:   123,
					Name: "sha256:abc123",
					Tags: []string{"v1.0"},
				},
				{
					ID:   456,
					Name: "sha256:def456",
					Tags: []string{"v2.0", "latest"},
				},
				{
					ID:   789,
					Name: "sha256:ghi789",
					Tags: []string{},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockGHClient{
				versions: tt.versions,
			}

			builder := &GraphBuilder{
				ctx:       context.Background(),
				ghClient:  mockClient,
				owner:     "testowner",
				ownerType: "users",
				imageName: "testimage",
			}

			cache, err := builder.GetVersionCache()

			if (err != nil) != tt.wantErr {
				t.Errorf("GetVersionCache() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil {
				return
			}

			if cache == nil {
				t.Fatal("GetVersionCache() returned nil cache")
			}

			// Verify cache has correct number of entries
			if len(cache.byDigest) != len(tt.versions) {
				t.Errorf("byDigest cache size = %d, want %d", len(cache.byDigest), len(tt.versions))
			}

			if len(cache.byID) != len(tt.versions) {
				t.Errorf("byID cache size = %d, want %d", len(cache.byID), len(tt.versions))
			}

			// Verify each version is in both caches
			for _, ver := range tt.versions {
				if v, ok := cache.byDigest[ver.Name]; !ok {
					t.Errorf("Version %s not found in byDigest cache", ver.Name)
				} else if v.ID != ver.ID {
					t.Errorf("byDigest cache: got ID %d, want %d", v.ID, ver.ID)
				}

				if v, ok := cache.byID[ver.ID]; !ok {
					t.Errorf("Version ID %d not found in byID cache", ver.ID)
				} else if v.Name != ver.Name {
					t.Errorf("byID cache: got Name %s, want %s", v.Name, ver.Name)
				}
			}
		})
	}
}

func TestVersionCacheLookups(t *testing.T) {
	versions := []gh.PackageVersionInfo{
		{
			ID:   100,
			Name: "sha256:abc",
			Tags: []string{"v1.0"},
		},
		{
			ID:   200,
			Name: "sha256:def",
			Tags: []string{"v2.0"},
		},
	}

	mockClient := &mockGHClient{versions: versions}
	builder := &GraphBuilder{
		ctx:       context.Background(),
		ghClient:  mockClient,
		owner:     "testowner",
		ownerType: "users",
		imageName: "testimage",
	}

	cache, err := builder.GetVersionCache()
	if err != nil {
		t.Fatalf("GetVersionCache() failed: %v", err)
	}

	// Test lookup by digest
	v, ok := cache.byDigest["sha256:abc"]
	if !ok {
		t.Error("Expected to find sha256:abc in byDigest cache")
	} else if v.ID != 100 {
		t.Errorf("byDigest lookup: got ID %d, want 100", v.ID)
	}

	// Test lookup by ID
	v, ok = cache.byID[200]
	if !ok {
		t.Error("Expected to find ID 200 in byID cache")
	} else if v.Name != "sha256:def" {
		t.Errorf("byID lookup: got Name %s, want sha256:def", v.Name)
	}

	// Test missing entries
	_, ok = cache.byDigest["sha256:missing"]
	if ok {
		t.Error("Should not find missing digest in cache")
	}

	_, ok = cache.byID[999]
	if ok {
		t.Error("Should not find missing ID in cache")
	}
}
