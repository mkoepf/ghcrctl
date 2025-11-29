package discover

import (
	"context"
	"testing"
)

// MockResolver implements TypeResolver for testing
type MockResolver struct {
	resolveFunc func(ctx context.Context, image, digest string) ([]string, error)
}

func (m *MockResolver) ResolveVersionType(ctx context.Context, image, digest string) ([]string, error) {
	return m.resolveFunc(ctx, image, digest)
}

func (m *MockResolver) ResolveVersionInfo(ctx context.Context, image, digest string) ([]string, int64, error) {
	types, err := m.resolveFunc(ctx, image, digest)
	return types, 1024, err // Return a default size of 1024 for testing
}

func TestResolveVersionType_Index(t *testing.T) {
	resolver := &MockResolver{
		resolveFunc: func(ctx context.Context, image, digest string) ([]string, error) {
			return []string{"index"}, nil
		},
	}

	types, err := resolver.ResolveVersionType(context.Background(), "ghcr.io/test/image", "sha256:abc123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(types) != 1 || types[0] != "index" {
		t.Errorf("expected [index], got %v", types)
	}
}

func TestResolveVersionType_Platform(t *testing.T) {
	resolver := &MockResolver{
		resolveFunc: func(ctx context.Context, image, digest string) ([]string, error) {
			return []string{"linux/amd64"}, nil
		},
	}

	types, err := resolver.ResolveVersionType(context.Background(), "ghcr.io/test/image", "sha256:abc123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(types) != 1 || types[0] != "linux/amd64" {
		t.Errorf("expected [linux/amd64], got %v", types)
	}
}

func TestResolveVersionType_MultipleAttestations(t *testing.T) {
	resolver := &MockResolver{
		resolveFunc: func(ctx context.Context, image, digest string) ([]string, error) {
			return []string{"sbom", "provenance"}, nil
		},
	}

	types, err := resolver.ResolveVersionType(context.Background(), "ghcr.io/test/image", "sha256:abc123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(types) != 2 {
		t.Errorf("expected 2 types, got %d", len(types))
	}
}
