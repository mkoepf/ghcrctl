package discover

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/mkoepf/ghcrctl/internal/gh"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	require.NoError(t, err)
	require.Len(t, results, 2)

	// Find index version
	var indexVersion *VersionInfo
	for i := range results {
		if results[i].Digest == "sha256:index1" {
			indexVersion = &results[i]
			break
		}
	}

	require.NotNil(t, indexVersion, "index version not found")
	assert.Len(t, indexVersion.OutgoingRefs, 1)
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
	require.NoError(t, err)

	// With parallel execution, multiple goroutines should run concurrently
	assert.GreaterOrEqual(t, maxConcurrent, int32(2), "expected parallel execution")
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
	require.NoError(t, err, "DiscoverPackage should not fail even when resolver fails")
	require.Len(t, results, 1)

	// When resolver fails, type should be "unknown"
	require.Len(t, results[0].Types, 1)
	assert.Equal(t, "unknown", results[0].Types[0])
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
	require.NoError(t, err)

	// Find platform version and check incoming refs
	var platformVersion *VersionInfo
	for i := range results {
		if results[i].Digest == "sha256:platform1" {
			platformVersion = &results[i]
			break
		}
	}

	require.NotNil(t, platformVersion, "platform version not found")
	require.Len(t, platformVersion.IncomingRefs, 1)
	assert.Equal(t, "sha256:index1", platformVersion.IncomingRefs[0])
}
