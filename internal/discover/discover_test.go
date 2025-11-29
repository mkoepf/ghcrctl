package discover

import (
	"context"
	"testing"

	"github.com/mkoepf/ghcrctl/internal/gh"
)

// MockChildDiscoverer implements ChildDiscoverer for testing
type MockChildDiscoverer struct {
	discoverFunc func(ctx context.Context, image, digest string, allTags []string) ([]string, error)
}

func (m *MockChildDiscoverer) DiscoverChildren(ctx context.Context, image, digest string, allTags []string) ([]string, error) {
	return m.discoverFunc(ctx, image, digest, allTags)
}

func TestDiscoverPackage_Basic(t *testing.T) {
	mockResolver := &MockResolver{
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

	mockDiscoverer := &MockChildDiscoverer{
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
		Resolver:        mockResolver,
		ChildDiscoverer: mockDiscoverer,
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

func TestDiscoverPackage_IncomingRefsInferred(t *testing.T) {
	mockResolver := &MockResolver{
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

	mockDiscoverer := &MockChildDiscoverer{
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
		Resolver:        mockResolver,
		ChildDiscoverer: mockDiscoverer,
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
