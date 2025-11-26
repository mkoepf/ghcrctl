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
			name:     "empty versions list",
			versions: []gh.PackageVersionInfo{},
			wantErr:  false,
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
			if len(cache.ByDigest) != len(tt.versions) {
				t.Errorf("ByDigest cache size = %d, want %d", len(cache.ByDigest), len(tt.versions))
			}

			if len(cache.ByID) != len(tt.versions) {
				t.Errorf("ByID cache size = %d, want %d", len(cache.ByID), len(tt.versions))
			}

			// Verify each version is in both caches
			for _, ver := range tt.versions {
				if v, ok := cache.ByDigest[ver.Name]; !ok {
					t.Errorf("Version %s not found in ByDigest cache", ver.Name)
				} else if v.ID != ver.ID {
					t.Errorf("ByDigest cache: got ID %d, want %d", v.ID, ver.ID)
				}

				if v, ok := cache.ByID[ver.ID]; !ok {
					t.Errorf("Version ID %d not found in ByID cache", ver.ID)
				} else if v.Name != ver.Name {
					t.Errorf("ByID cache: got Name %s, want %s", v.Name, ver.Name)
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
	v, ok := cache.ByDigest["sha256:abc"]
	if !ok {
		t.Error("Expected to find sha256:abc in ByDigest cache")
	} else if v.ID != 100 {
		t.Errorf("ByDigest lookup: got ID %d, want 100", v.ID)
	}

	// Test lookup by ID
	v, ok = cache.ByID[200]
	if !ok {
		t.Error("Expected to find ID 200 in ByID cache")
	} else if v.Name != "sha256:def" {
		t.Errorf("ByID lookup: got Name %s, want sha256:def", v.Name)
	}

	// Test missing entries
	_, ok = cache.ByDigest["sha256:missing"]
	if ok {
		t.Error("Should not find missing digest in cache")
	}

	_, ok = cache.ByID[999]
	if ok {
		t.Error("Should not find missing ID in cache")
	}
}

func TestNewVersionCacheFromSlice(t *testing.T) {
	tests := []struct {
		name     string
		versions []gh.PackageVersionInfo
	}{
		{
			name:     "empty versions list",
			versions: []gh.PackageVersionInfo{},
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
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache := NewVersionCacheFromSlice(tt.versions)

			if cache == nil {
				t.Fatal("NewVersionCacheFromSlice() returned nil cache")
			}

			// Verify cache has correct number of entries
			if len(cache.ByDigest) != len(tt.versions) {
				t.Errorf("ByDigest cache size = %d, want %d", len(cache.ByDigest), len(tt.versions))
			}

			if len(cache.ByID) != len(tt.versions) {
				t.Errorf("ByID cache size = %d, want %d", len(cache.ByID), len(tt.versions))
			}

			// Verify each version is in both caches
			for _, ver := range tt.versions {
				if v, ok := cache.ByDigest[ver.Name]; !ok {
					t.Errorf("Version %s not found in ByDigest cache", ver.Name)
				} else if v.ID != ver.ID {
					t.Errorf("ByDigest cache: got ID %d, want %d", v.ID, ver.ID)
				}

				if v, ok := cache.ByID[ver.ID]; !ok {
					t.Errorf("Version ID %d not found in ByID cache", ver.ID)
				} else if v.Name != ver.Name {
					t.Errorf("ByID cache: got Name %s, want %s", v.Name, ver.Name)
				}
			}
		})
	}
}

func TestSortByIDProximity(t *testing.T) {
	versions := []gh.PackageVersionInfo{
		{ID: 100, Name: "v100"},
		{ID: 110, Name: "v110"},
		{ID: 90, Name: "v90"},
		{ID: 200, Name: "v200"},
		{ID: 105, Name: "v105"},
	}

	// Sort by proximity to ID 100
	sorted := sortByIDProximity(versions, 100)

	// Expected order: 100 (distance 0), 105 (distance 5), 110 (distance 10), 90 (distance 10), 200 (distance 100)
	// Note: 110 and 90 both have distance 10, so order between them is not guaranteed
	if sorted[0].ID != 100 {
		t.Errorf("First element should be ID 100, got %d", sorted[0].ID)
	}
	if sorted[1].ID != 105 {
		t.Errorf("Second element should be ID 105, got %d", sorted[1].ID)
	}
	// Last should be furthest
	if sorted[len(sorted)-1].ID != 200 {
		t.Errorf("Last element should be ID 200, got %d", sorted[len(sorted)-1].ID)
	}

	// Original slice should not be modified
	if versions[0].ID != 100 {
		t.Error("Original slice should not be modified")
	}
}
