package discovery

import (
	"context"
	"testing"

	"github.com/mhk/ghcrctl/internal/gh"
	"github.com/mhk/ghcrctl/internal/oras"
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

// mockOrasClient is a mock implementation of OrasClient for testing
type mockOrasClient struct {
	platforms map[string][]oras.PlatformInfo // digest -> platforms
	referrers map[string][]oras.ReferrerInfo // digest -> referrers
}

func (m *mockOrasClient) GetPlatformManifests(ctx context.Context, image, digest string) ([]oras.PlatformInfo, error) {
	if platforms, ok := m.platforms[digest]; ok {
		return platforms, nil
	}
	return []oras.PlatformInfo{}, nil
}

func (m *mockOrasClient) DiscoverReferrers(ctx context.Context, image, digest string) ([]oras.ReferrerInfo, error) {
	if referrers, ok := m.referrers[digest]; ok {
		return referrers, nil
	}
	return []oras.ReferrerInfo{}, nil
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

func TestBuildGraph_MultiArchWithAttestations(t *testing.T) {
	// Setup: Multi-arch image with 2 platforms and attestations
	rootDigest := "sha256:1111111111111111111111111111111111111111111111111111111111111111"
	amd64Digest := "sha256:2222222222222222222222222222222222222222222222222222222222222222"
	arm64Digest := "sha256:3333333333333333333333333333333333333333333333333333333333333333"
	sbomDigest := "sha256:4444444444444444444444444444444444444444444444444444444444444444"
	provDigest := "sha256:5555555555555555555555555555555555555555555555555555555555555555"

	versions := []gh.PackageVersionInfo{
		{ID: 100, Name: rootDigest, Tags: []string{"v1.0.0", "latest"}, CreatedAt: "2025-01-15 10:00:00"},
		{ID: 101, Name: amd64Digest, Tags: []string{}, CreatedAt: "2025-01-15 10:00:01"},
		{ID: 102, Name: arm64Digest, Tags: []string{}, CreatedAt: "2025-01-15 10:00:02"},
		{ID: 103, Name: sbomDigest, Tags: []string{}, CreatedAt: "2025-01-15 10:00:03"},
		{ID: 104, Name: provDigest, Tags: []string{}, CreatedAt: "2025-01-15 10:00:04"},
	}

	mockGH := &mockGHClient{versions: versions}
	mockOras := &mockOrasClient{
		platforms: map[string][]oras.PlatformInfo{
			rootDigest: {
				{Digest: amd64Digest, Platform: "linux/amd64", OS: "linux", Architecture: "amd64", Size: 1000},
				{Digest: arm64Digest, Platform: "linux/arm64", OS: "linux", Architecture: "arm64", Size: 1100},
			},
		},
		referrers: map[string][]oras.ReferrerInfo{
			rootDigest: {
				{Digest: sbomDigest, ArtifactType: "sbom", Size: 5000},
				{Digest: provDigest, ArtifactType: "provenance", Size: 3000},
			},
		},
	}

	cache := NewVersionCacheFromSlice(versions)
	builder := NewGraphBuilderWithOras(context.Background(), mockGH, mockOras, "ghcr.io/test/image", "test", "user", "image")

	graph, err := builder.BuildGraph(rootDigest, cache)
	if err != nil {
		t.Fatalf("BuildGraph() error = %v", err)
	}

	// Verify root
	if graph.RootVersion.ID != 100 {
		t.Errorf("Root version ID = %d, want 100", graph.RootVersion.ID)
	}
	if graph.Type != "index" {
		t.Errorf("Graph type = %s, want index", graph.Type)
	}
	if len(graph.RootVersion.Tags) != 2 {
		t.Errorf("Root tags count = %d, want 2", len(graph.RootVersion.Tags))
	}

	// Verify children count: 2 platforms + 2 attestations = 4
	if len(graph.Children) != 4 {
		t.Errorf("Children count = %d, want 4", len(graph.Children))
	}

	// Verify platforms are present
	platformCount := 0
	for _, child := range graph.Children {
		if child.ArtifactType == "platform" {
			platformCount++
			if child.Platform != "linux/amd64" && child.Platform != "linux/arm64" {
				t.Errorf("Unexpected platform: %s", child.Platform)
			}
		}
	}
	if platformCount != 2 {
		t.Errorf("Platform count = %d, want 2", platformCount)
	}

	// Verify attestations are present
	attestationCount := 0
	for _, child := range graph.Children {
		if child.ArtifactType == "sbom" || child.ArtifactType == "provenance" {
			attestationCount++
		}
	}
	if attestationCount != 2 {
		t.Errorf("Attestation count = %d, want 2", attestationCount)
	}
}

func TestBuildGraph_SingleArchImage(t *testing.T) {
	// Setup: Single-arch image (no platforms in index)
	rootDigest := "sha256:1111111111111111111111111111111111111111111111111111111111111111"
	sbomDigest := "sha256:2222222222222222222222222222222222222222222222222222222222222222"

	versions := []gh.PackageVersionInfo{
		{ID: 100, Name: rootDigest, Tags: []string{"latest"}, CreatedAt: "2025-01-15 10:00:00"},
		{ID: 101, Name: sbomDigest, Tags: []string{}, CreatedAt: "2025-01-15 10:00:01"},
	}

	mockGH := &mockGHClient{versions: versions}
	mockOras := &mockOrasClient{
		platforms: map[string][]oras.PlatformInfo{}, // No platforms
		referrers: map[string][]oras.ReferrerInfo{
			rootDigest: {
				{Digest: sbomDigest, ArtifactType: "sbom", Size: 5000},
			},
		},
	}

	cache := NewVersionCacheFromSlice(versions)
	builder := NewGraphBuilderWithOras(context.Background(), mockGH, mockOras, "ghcr.io/test/image", "test", "user", "image")

	graph, err := builder.BuildGraph(rootDigest, cache)
	if err != nil {
		t.Fatalf("BuildGraph() error = %v", err)
	}

	// Verify root
	if graph.RootVersion.ID != 100 {
		t.Errorf("Root version ID = %d, want 100", graph.RootVersion.ID)
	}
	if graph.Type != "manifest" {
		t.Errorf("Graph type = %s, want manifest", graph.Type)
	}

	// Verify only 1 child (sbom)
	if len(graph.Children) != 1 {
		t.Errorf("Children count = %d, want 1", len(graph.Children))
	}
	if graph.Children[0].ArtifactType != "sbom" {
		t.Errorf("Child artifact type = %s, want sbom", graph.Children[0].ArtifactType)
	}
}

func TestBuildGraph_StandaloneImage(t *testing.T) {
	// Setup: Image with no platforms and no referrers
	rootDigest := "sha256:1111111111111111111111111111111111111111111111111111111111111111"

	versions := []gh.PackageVersionInfo{
		{ID: 100, Name: rootDigest, Tags: []string{"latest"}, CreatedAt: "2025-01-15 10:00:00"},
	}

	mockGH := &mockGHClient{versions: versions}
	mockOras := &mockOrasClient{
		platforms: map[string][]oras.PlatformInfo{},
		referrers: map[string][]oras.ReferrerInfo{},
	}

	cache := NewVersionCacheFromSlice(versions)
	builder := NewGraphBuilderWithOras(context.Background(), mockGH, mockOras, "ghcr.io/test/image", "test", "user", "image")

	graph, err := builder.BuildGraph(rootDigest, cache)
	if err != nil {
		t.Fatalf("BuildGraph() error = %v", err)
	}

	// Verify root
	if graph.RootVersion.ID != 100 {
		t.Errorf("Root version ID = %d, want 100", graph.RootVersion.ID)
	}
	if graph.Type != "standalone" {
		t.Errorf("Graph type = %s, want standalone", graph.Type)
	}

	// Verify no children
	if len(graph.Children) != 0 {
		t.Errorf("Children count = %d, want 0", len(graph.Children))
	}
}
