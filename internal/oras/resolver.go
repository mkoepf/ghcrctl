package oras

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
)

// ResolveTag resolves an image tag to its digest using ORAS
// image should be in format: registry/owner/repo (e.g., ghcr.io/owner/repo)
// tag is the tag name (e.g., "latest", "v1.0.0")
// Returns the digest in format sha256:... or an error
func ResolveTag(ctx context.Context, image, tag string) (string, error) {
	// Validate inputs
	if image == "" {
		return "", fmt.Errorf("image cannot be empty")
	}
	if tag == "" {
		return "", fmt.Errorf("tag cannot be empty")
	}

	// Parse image reference to get registry and path
	registry, path, err := parseImageReference(image)
	if err != nil {
		return "", err
	}

	// Create repository reference
	repo, err := remote.NewRepository(fmt.Sprintf("%s/%s", registry, path))
	if err != nil {
		return "", fmt.Errorf("failed to create repository reference: %w", err)
	}

	// Configure authentication for GHCR
	// Use GitHub token if available
	repo.Client = &auth.Client{
		Cache: auth.NewCache(),
	}

	// Resolve the tag to a descriptor
	descriptor, err := repo.Resolve(ctx, tag)
	if err != nil {
		return "", fmt.Errorf("failed to resolve tag '%s': %w", tag, err)
	}

	// Return the digest
	digest := descriptor.Digest.String()

	// Validate digest format
	if !validateDigestFormat(digest) {
		return "", fmt.Errorf("invalid digest format returned: %s", digest)
	}

	return digest, nil
}

// parseImageReference parses an image reference into registry and path components
// Expected format: registry/owner/repo or registry/owner/org/repo
// Returns: registry, path, error
func parseImageReference(image string) (string, string, error) {
	if image == "" {
		return "", "", fmt.Errorf("invalid image format: image cannot be empty")
	}

	// Split by first slash to separate registry from path
	parts := strings.SplitN(image, "/", 2)
	if len(parts) < 2 {
		return "", "", fmt.Errorf("invalid image format: must be in format registry/owner/repo")
	}

	registry := parts[0]
	path := parts[1]

	// Validate registry looks like a domain
	if !strings.Contains(registry, ".") {
		return "", "", fmt.Errorf("invalid image format: registry must be a domain (e.g., ghcr.io)")
	}

	// Validate path is not empty
	if path == "" {
		return "", "", fmt.Errorf("invalid image format: path cannot be empty")
	}

	return registry, path, nil
}

// validateDigestFormat validates that a digest string is in the correct format
// Expected format: sha256:1234567890abcdef... (64 hex characters)
func validateDigestFormat(digest string) bool {
	if digest == "" {
		return false
	}

	// Check for sha256: prefix
	if !strings.HasPrefix(digest, "sha256:") {
		return false
	}

	// Extract hash part
	hash := strings.TrimPrefix(digest, "sha256:")

	// Must be exactly 64 hex characters
	if len(hash) != 64 {
		return false
	}

	// Validate hex characters
	hexPattern := regexp.MustCompile("^[0-9a-f]{64}$")
	return hexPattern.MatchString(hash)
}
