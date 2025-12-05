// Package testutil provides utilities for integration testing
package testutil

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mkoepf/ghcrctl/internal/gh"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/credentials"
)

// GenerateEphemeralName creates a unique package name with the given prefix
// and an 8-character random hex suffix.
func GenerateEphemeralName(prefix string) string {
	suffix := make([]byte, 4)
	_, err := rand.Read(suffix)
	if err != nil {
		// Fallback to less random but still unique-ish
		return fmt.Sprintf("%s-%08x", prefix, 0)
	}
	return fmt.Sprintf("%s-%s", prefix, hex.EncodeToString(suffix))
}

// CopyImage copies an image from source to destination repository.
// srcImage and dstImage should be in format: ghcr.io/owner/repo
func CopyImage(ctx context.Context, srcImage, srcTag, dstImage, dstTag string) error {
	// Parse source image
	srcRegistry, srcPath, err := parseImageReference(srcImage)
	if err != nil {
		return fmt.Errorf("invalid source image: %w", err)
	}

	// Parse destination image
	dstRegistry, dstPath, err := parseImageReference(dstImage)
	if err != nil {
		return fmt.Errorf("invalid destination image: %w", err)
	}

	// Create source repository
	srcRepo, err := remote.NewRepository(fmt.Sprintf("%s/%s", srcRegistry, srcPath))
	if err != nil {
		return fmt.Errorf("failed to create source repository: %w", err)
	}
	configureAuth(srcRepo)
	// Disable referrers API to avoid race condition in ORAS library
	// (GHCR doesn't support OCI referrers API anyway)
	// Error is only returned if called twice, which won't happen here
	_ = srcRepo.SetReferrersCapability(false)

	// Create destination repository
	dstRepo, err := remote.NewRepository(fmt.Sprintf("%s/%s", dstRegistry, dstPath))
	if err != nil {
		return fmt.Errorf("failed to create destination repository: %w", err)
	}
	configureAuth(dstRepo)
	// Disable referrers API to avoid race condition in ORAS library
	// Error is only returned if called twice, which won't happen here
	_ = dstRepo.SetReferrersCapability(false)

	// Copy the image with retry logic for rate limiting (429 errors).
	// Note: oras-go has an internal race condition in HTTP/2 auth handling
	// that triggers with -race flag, but it doesn't affect correctness.
	// The mutating tests are run without -race for this reason.
	var lastErr error
	for attempt := 0; attempt < 5; attempt++ {
		if attempt > 0 {
			// Exponential backoff: 100ms, 200ms, 400ms, 800ms
			backoff := time.Duration(100<<attempt) * time.Millisecond
			time.Sleep(backoff)
		}

		_, err = oras.Copy(ctx, srcRepo, srcTag, dstRepo, dstTag, oras.CopyOptions{})
		if err == nil {
			return nil
		}

		lastErr = err
		// Retry on rate limiting (429)
		if !strings.Contains(err.Error(), "429") && !strings.Contains(err.Error(), "toomanyrequests") {
			break
		}
	}

	return fmt.Errorf("failed to copy image: %w", lastErr)
}

// DeletePackage deletes an entire package from the registry.
// owner is the GitHub username or org name.
// packageName is the name of the package to delete.
func DeletePackage(ctx context.Context, owner, packageName string) error {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return fmt.Errorf("GITHUB_TOKEN not set")
	}

	client, err := gh.NewClient(token)
	if err != nil {
		return fmt.Errorf("failed to create GitHub client: %w", err)
	}

	// Determine owner type
	ownerType, err := client.GetOwnerType(ctx, owner)
	if err != nil {
		return fmt.Errorf("failed to get owner type: %w", err)
	}

	// Delete the package
	err = client.DeletePackage(ctx, owner, ownerType, packageName)
	if err != nil {
		return fmt.Errorf("failed to delete package: %w", err)
	}

	return nil
}

// parseImageReference parses an image reference into registry and path components.
func parseImageReference(image string) (string, string, error) {
	if image == "" {
		return "", "", fmt.Errorf("image cannot be empty")
	}

	parts := strings.SplitN(image, "/", 2)
	if len(parts) < 2 {
		return "", "", fmt.Errorf("invalid image format: must be registry/owner/repo")
	}

	registry := parts[0]
	path := parts[1]

	if !strings.Contains(registry, ".") {
		return "", "", fmt.Errorf("invalid registry: must be a domain")
	}

	return registry, path, nil
}

// configureAuth configures authentication for GHCR.
func configureAuth(repo *remote.Repository) {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return
	}

	store := credentials.NewMemoryStore()
	cred := auth.Credential{
		Username: "oauth2",
		Password: token,
	}
	_ = store.Put(context.Background(), "ghcr.io", cred)

	repo.Client = &auth.Client{
		Cache:      auth.NewCache(),
		Credential: credentials.Credential(store),
	}
}
