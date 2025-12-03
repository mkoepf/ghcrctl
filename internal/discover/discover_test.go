package discover

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/mkoepf/ghcrctl/internal/gh"
)

// mockChildDiscoverer implements childDiscoverer for testing
type mockChildDiscoverer struct {
	discoverFunc func(ctx context.Context, image, digest string, allTags []string) ([]string, error)
}

func (m *mockChildDiscoverer) discoverChildren(ctx context.Context, image, digest string, allTags []string) ([]string, error) {
	return m.discoverFunc(ctx, image, digest, allTags)
}

func TestDiscoverPackage_Basic(t *testing.T) {
	mockResolver := &mockResolver{
		resolveFunc: func(ctx context.Context, image, digest string) ([]string, error) {
			switch digest {
			case "sha256:index1":
				return []string{"index"}, nil
			case "sha256:platform1":
				return []string{"linux/amd64"}, nil
			default:
				return []string{"manifest"}, nil
			}
		},
	}

	mockDiscoverer := &mockChildDiscoverer{
		discoverFunc: func(ctx context.Context, image, digest string, allTags []string) ([]string, error) {
			if digest == "sha256:index1" {
				return []string{"sha256:platform1"}, nil
			}
			return nil, nil
		},
	}

	versions := []gh.PackageVersionInfo{
		{ID: 1, Name: "sha256:index1", Tags: []string{"v1.0.0"}, CreatedAt: "2025-01-15"},
		{ID: 2, Name: "sha256:platform1", Tags: nil, CreatedAt: "2025-01-15"},
	}

	discoverer := &PackageDiscoverer{
		resolver:        mockResolver,
		childDiscoverer: mockDiscoverer,
	}

	results, err := discoverer.DiscoverPackage(context.Background(), "ghcr.io/test/image", versions, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// Find index version
	var indexVersion *VersionInfo
	for i := range results {
		if results[i].Digest == "sha256:index1" {
			indexVersion = &results[i]
			break
		}
	}

	if indexVersion == nil {
		t.Fatal("index version not found")
	}

	if len(indexVersion.OutgoingRefs) != 1 {
		t.Errorf("expected 1 outgoing ref, got %d", len(indexVersion.OutgoingRefs))
	}
}

func TestDiscoverPackage_Parallel(t *testing.T) {
	// Track concurrent calls to verify parallelism
	var concurrentCalls int32
	var maxConcurrent int32
	var mu sync.Mutex

	mockResolver := &mockResolver{
		resolveFunc: func(ctx context.Context, image, digest string) ([]string, error) {
			mu.Lock()
			concurrentCalls++
			if concurrentCalls > maxConcurrent {
				maxConcurrent = concurrentCalls
			}
			mu.Unlock()

			time.Sleep(50 * time.Millisecond) // Simulate slow API call

			mu.Lock()
			concurrentCalls--
			mu.Unlock()

			return []string{"manifest"}, nil
		},
	}

	mockDiscoverer := &mockChildDiscoverer{
		discoverFunc: func(ctx context.Context, image, digest string, allTags []string) ([]string, error) {
			return nil, nil
		},
	}

	// Create 10 versions to test parallelism
	versions := make([]gh.PackageVersionInfo, 10)
	for i := 0; i < 10; i++ {
		versions[i] = gh.PackageVersionInfo{
			ID:        int64(i + 1),
			Name:      fmt.Sprintf("sha256:digest%d", i),
			CreatedAt: "2025-01-15",
		}
	}

	discoverer := &PackageDiscoverer{
		resolver:        mockResolver,
		childDiscoverer: mockDiscoverer,
	}

	_, err := discoverer.DiscoverPackage(context.Background(), "ghcr.io/test/image", versions, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// With parallel execution, multiple goroutines should run concurrently
	if maxConcurrent < 2 {
		t.Errorf("expected parallel execution (maxConcurrent >= 2), but got %d", maxConcurrent)
	}
}

func TestDiscoverPackage_ResolverFailure(t *testing.T) {
	t.Parallel()

	// When OCI registry is unreachable, version type should be marked as 'unknown'
	mockResolver := &mockResolver{
		resolveFunc: func(ctx context.Context, image, digest string) ([]string, error) {
			return nil, fmt.Errorf("registry unreachable")
		},
	}

	mockDiscoverer := &mockChildDiscoverer{
		discoverFunc: func(ctx context.Context, image, digest string, allTags []string) ([]string, error) {
			return nil, nil
		},
	}

	versions := []gh.PackageVersionInfo{
		{ID: 1, Name: "sha256:abc123", Tags: []string{"v1.0.0"}, CreatedAt: "2025-01-15"},
	}

	discoverer := &PackageDiscoverer{
		resolver:        mockResolver,
		childDiscoverer: mockDiscoverer,
	}

	results, err := discoverer.DiscoverPackage(context.Background(), "ghcr.io/test/image", versions, nil)
	if err != nil {
		t.Fatalf("DiscoverPackage should not fail even when resolver fails: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	// When resolver fails, type should be "unknown"
	if len(results[0].Types) != 1 || results[0].Types[0] != "unknown" {
		t.Errorf("expected type 'unknown' when resolver fails, got %v", results[0].Types)
	}
}

func TestDiscoverPackage_IncomingRefsInferred(t *testing.T) {
	mockResolver := &mockResolver{
		resolveFunc: func(ctx context.Context, image, digest string) ([]string, error) {
			switch digest {
			case "sha256:index1":
				return []string{"index"}, nil
			case "sha256:platform1":
				return []string{"linux/amd64"}, nil
			default:
				return []string{"manifest"}, nil
			}
		},
	}

	mockDiscoverer := &mockChildDiscoverer{
		discoverFunc: func(ctx context.Context, image, digest string, allTags []string) ([]string, error) {
			if digest == "sha256:index1" {
				return []string{"sha256:platform1"}, nil
			}
			return nil, nil
		},
	}

	versions := []gh.PackageVersionInfo{
		{ID: 1, Name: "sha256:index1", Tags: []string{"v1.0.0"}, CreatedAt: "2025-01-15"},
		{ID: 2, Name: "sha256:platform1", Tags: nil, CreatedAt: "2025-01-15"},
	}

	discoverer := &PackageDiscoverer{
		resolver:        mockResolver,
		childDiscoverer: mockDiscoverer,
	}

	results, err := discoverer.DiscoverPackage(context.Background(), "ghcr.io/test/image", versions, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Find platform version and check incoming refs
	var platformVersion *VersionInfo
	for i := range results {
		if results[i].Digest == "sha256:platform1" {
			platformVersion = &results[i]
			break
		}
	}

	if platformVersion == nil {
		t.Fatal("platform version not found")
	}

	if len(platformVersion.IncomingRefs) != 1 || platformVersion.IncomingRefs[0] != "sha256:index1" {
		t.Errorf("expected incoming ref from index1, got %v", platformVersion.IncomingRefs)
	}
}
