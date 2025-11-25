package oras

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"

	"github.com/mhk/ghcrctl/internal/logging"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/credentials"
)

// Package-level caches to avoid redundant API calls
var (
	// Auth client cache to avoid redundant token fetches
	authClientCache     *auth.Client
	authClientCacheMu   sync.RWMutex
	authClientCacheInit sync.Once

	// Manifest descriptor cache to avoid redundant Resolve/Fetch calls
	// Key: digest string (e.g., "sha256:abc123...")
	// Value: descriptor from repo.Resolve()
	manifestDescCache   map[string]ocispec.Descriptor
	manifestDescCacheMu sync.RWMutex

	// Manifest content cache to avoid redundant Fetch calls
	// Key: digest string
	// Value: parsed OCI index
	manifestIndexCache   map[string]*ocispec.Index
	manifestIndexCacheMu sync.RWMutex
)

func init() {
	manifestDescCache = make(map[string]ocispec.Descriptor)
	manifestIndexCache = make(map[string]*ocispec.Index)
}

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

	// Configure authentication for GHCR using GitHub token
	if err := configureAuth(ctx, repo); err != nil {
		return "", fmt.Errorf("failed to configure authentication: %w", err)
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

// ReferrerInfo contains information about a discovered referrer artifact
type ReferrerInfo struct {
	Digest       string // Digest of the referrer artifact
	ArtifactType string // Type of the referrer (sbom, provenance, etc.)
	MediaType    string // Original OCI media type
}

// PlatformInfo contains information about a platform-specific manifest
type PlatformInfo struct {
	Platform     string // Platform string (e.g., "linux/amd64", "linux/arm64")
	Digest       string // Digest of the platform-specific manifest
	Size         int64  // Size of the manifest in bytes
	OS           string // Operating system
	Architecture string // CPU architecture
	Variant      string // Optional variant (e.g., "v7" for ARM)
}

// DiscoverReferrers discovers all referrer artifacts for a given image digest
// Returns a list of referrers (SBOM, provenance, etc.) or an error
func DiscoverReferrers(ctx context.Context, image, digest string) ([]ReferrerInfo, error) {
	// Validate inputs
	if image == "" {
		return nil, fmt.Errorf("image cannot be empty")
	}
	if digest == "" {
		return nil, fmt.Errorf("digest cannot be empty")
	}
	if !validateDigestFormat(digest) {
		return nil, fmt.Errorf("invalid digest format: %s", digest)
	}

	// Parse image reference
	registry, path, err := parseImageReference(image)
	if err != nil {
		return nil, err
	}

	// Create repository reference
	repo, err := remote.NewRepository(fmt.Sprintf("%s/%s", registry, path))
	if err != nil {
		return nil, fmt.Errorf("failed to create repository reference: %w", err)
	}

	// Configure authentication for GHCR using GitHub token
	if err := configureAuth(ctx, repo); err != nil {
		return nil, fmt.Errorf("failed to configure authentication: %w", err)
	}

	// Resolve the digest to get a full descriptor (uses cache to avoid duplicate HEAD requests)
	desc, err := cachedResolve(ctx, repo, digest)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve digest: %w", err)
	}

	referrers := []ReferrerInfo{}

	// GHCR-specific optimization: GHCR does not support OCI 1.1 Referrers API
	// Docker buildx stores attestations in the image index, not as OCI referrers
	// For GHCR, we only need to check attestations within the image index
	// This skips the 3 failed API calls (OCI 1.1 attempt → redirect → 404 fallback)
	if desc.MediaType == "application/vnd.docker.distribution.manifest.list.v2+json" ||
		desc.MediaType == "application/vnd.oci.image.index.v1+json" {
		// Fetch and parse the image index to find attestations (uses cache)
		indexReferrers, err := discoverAttestationsInIndex(ctx, repo, desc)
		if err != nil {
			// Non-fatal: image may not have attestations
			fmt.Fprintf(os.Stderr, "Warning: failed to discover attestations in image index: %v\n", err)
		} else {
			referrers = append(referrers, indexReferrers...)
		}
	}

	// Note: We skip the OCI 1.1 Referrers API check (repo.Referrers) because:
	// 1. GHCR does not implement OCI 1.1 Referrers API (always returns 404)
	// 2. Docker buildx attestations are stored in the image index (checked above)
	// 3. This eliminates 3 unnecessary API calls: OCI 1.1 attempt (303) → GHCR endpoint (404) → legacy tag (404)

	return referrers, nil
}

// discoverAttestationsInIndex fetches an image index and extracts attestation manifests
// Docker buildx stores SBOM and provenance as manifests within the image index
// Now uses cached fetch to avoid redundant GET requests
func discoverAttestationsInIndex(ctx context.Context, repo *remote.Repository, indexDesc ocispec.Descriptor) ([]ReferrerInfo, error) {
	// Fetch and parse the image index (uses cache to avoid duplicate GET requests)
	index, err := cachedFetchIndex(ctx, repo, indexDesc)
	if err != nil {
		return nil, err // Error already wrapped by cachedFetchIndex
	}

	referrers := []ReferrerInfo{}

	// Look for attestation manifests in the index
	for _, manifest := range index.Manifests {

		// Attestations have specific characteristics:
		// 1. Platform is often unknown/unknown
		// 2. Annotations like vnd.docker.reference.type: attestation-manifest
		// 3. ArtifactType or MediaType indicates attestation type

		isAttestation := false

		// Check annotations for attestation marker
		if manifest.Annotations != nil {
			if refType, ok := manifest.Annotations["vnd.docker.reference.type"]; ok {
				if refType == "attestation-manifest" {
					isAttestation = true
				}
			}
		}

		// Check if platform is unknown (common for attestations)
		if manifest.Platform != nil {
			if manifest.Platform.OS == "unknown" && manifest.Platform.Architecture == "unknown" {
				isAttestation = true
			}
		}

		// Check for in-toto attestation media types
		if strings.Contains(manifest.MediaType, "in-toto") ||
			strings.Contains(manifest.ArtifactType, "in-toto") {
			isAttestation = true
		}

		if isAttestation {
			// Fetch the attestation manifest itself to determine types
			// Note: A single manifest can contain MULTIPLE attestation types (e.g., SBOM + provenance)
			attestationTypes, err := determineAttestationTypesFromManifest(ctx, repo, manifest)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to determine attestation type for %s: %v\n", manifest.Digest.String(), err)
				attestationTypes = []string{"unknown"}
			}

			// Create a separate ReferrerInfo for each attestation type in this manifest
			for _, artifactType := range attestationTypes {
				referrers = append(referrers, ReferrerInfo{
					Digest:       manifest.Digest.String(),
					ArtifactType: artifactType,
					MediaType:    manifest.MediaType,
				})
			}
		}
	}

	return referrers, nil
}

// determineAttestationTypesFromManifest fetches and examines an attestation manifest to determine all its types
// Returns a slice because a single manifest can contain multiple attestation types (e.g., SBOM + provenance)
func determineAttestationTypesFromManifest(ctx context.Context, repo *remote.Repository, indexManifest ocispec.Descriptor) ([]string, error) {
	// Fetch the full attestation manifest
	manifestBytes, err := repo.Fetch(ctx, indexManifest)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch manifest: %w", err)
	}
	defer manifestBytes.Close()

	// Read and parse the manifest
	var manifest ocispec.Manifest
	if err := json.NewDecoder(manifestBytes).Decode(&manifest); err != nil {
		return nil, fmt.Errorf("failed to decode manifest: %w", err)
	}

	foundTypes := make(map[string]bool)

	// Check manifest annotations for in-toto predicate type
	if manifest.Annotations != nil {
		if predType, ok := manifest.Annotations["in-toto.io/predicate-type"]; ok {
			// SBOM types
			if strings.Contains(predType, "spdx") || strings.Contains(predType, "cyclonedx") {
				foundTypes["sbom"] = true
			}
			// Provenance types
			if strings.Contains(predType, "slsa") || strings.Contains(predType, "provenance") {
				foundTypes["provenance"] = true
			}
		}
	}

	// Check config annotations
	if manifest.Config.Annotations != nil {
		if predType, ok := manifest.Config.Annotations["in-toto.io/predicate-type"]; ok {
			if strings.Contains(predType, "spdx") || strings.Contains(predType, "cyclonedx") {
				foundTypes["sbom"] = true
			}
			if strings.Contains(predType, "slsa") || strings.Contains(predType, "provenance") {
				foundTypes["provenance"] = true
			}
		}
	}

	// Check layer annotations - IMPORTANT: Check ALL layers!
	// Docker buildx can store multiple attestation types in separate layers
	for _, layer := range manifest.Layers {
		if layer.Annotations != nil {
			if predType, ok := layer.Annotations["in-toto.io/predicate-type"]; ok {
				// Track what we find
				if strings.Contains(predType, "spdx") || strings.Contains(predType, "cyclonedx") {
					foundTypes["sbom"] = true
				}
				if strings.Contains(predType, "slsa") || strings.Contains(predType, "provenance") {
					foundTypes["provenance"] = true
				}
			}
		}
	}

	// Fetch and check the config blob for more details (if we haven't found types yet)
	if len(foundTypes) == 0 && manifest.Config.MediaType != "" {
		configBytes, err := repo.Fetch(ctx, manifest.Config)
		if err == nil {
			defer configBytes.Close()

			// Read config as JSON to look for predicate type
			var configData map[string]interface{}
			if json.NewDecoder(configBytes).Decode(&configData) == nil {
				// Check for predicate type in config
				if predType, ok := configData["predicateType"].(string); ok {
					if strings.Contains(predType, "spdx") || strings.Contains(predType, "cyclonedx") {
						foundTypes["sbom"] = true
					}
					if strings.Contains(predType, "slsa") || strings.Contains(predType, "provenance") {
						foundTypes["provenance"] = true
					}
				}
			}
		}
	}

	// Convert map to slice
	if len(foundTypes) == 0 {
		return []string{"unknown"}, nil
	}

	types := make([]string, 0, len(foundTypes))
	for attestType := range foundTypes {
		types = append(types, attestType)
	}

	return types, nil
}

// determineArtifactType maps OCI media types to human-readable artifact types
func determineArtifactType(mediaType string) string {
	if mediaType == "" {
		return "unknown"
	}

	// SBOM types
	if strings.Contains(mediaType, "spdx") || strings.Contains(mediaType, "cyclonedx") {
		return "sbom"
	}

	// Provenance types
	if strings.Contains(mediaType, "in-toto") || strings.Contains(mediaType, "slsa") {
		return "provenance"
	}

	// Attestation/signature types
	if strings.Contains(mediaType, "cosign") || strings.Contains(mediaType, "sigstore") {
		return "attestation"
	}

	return "unknown"
}

// parseDigest parses a digest string into an OCI descriptor
// digestStr should be in format: sha256:1234567890abcdef...
func parseDigest(digestStr string) (ocispec.Descriptor, error) {
	if !validateDigestFormat(digestStr) {
		return ocispec.Descriptor{}, fmt.Errorf("invalid digest format: %s", digestStr)
	}

	// Create a descriptor with the digest
	// For referrers API, we only need the Digest field
	desc := ocispec.Descriptor{
		Digest: digest.Digest(digestStr),
	}

	return desc, nil
}

// GetPlatformManifests extracts platform-specific manifests from an image index
// Returns a list of platform manifests, or empty list if the image is single-arch
func GetPlatformManifests(ctx context.Context, image, digest string) ([]PlatformInfo, error) {
	// Validate inputs
	if image == "" {
		return nil, fmt.Errorf("image cannot be empty")
	}
	if digest == "" {
		return nil, fmt.Errorf("digest cannot be empty")
	}
	if !validateDigestFormat(digest) {
		return nil, fmt.Errorf("invalid digest format: %s", digest)
	}

	// Parse image reference
	registry, path, err := parseImageReference(image)
	if err != nil {
		return nil, err
	}

	// Create repository reference
	repo, err := remote.NewRepository(fmt.Sprintf("%s/%s", registry, path))
	if err != nil {
		return nil, fmt.Errorf("failed to create repository reference: %w", err)
	}

	// Configure authentication
	if err := configureAuth(ctx, repo); err != nil {
		return nil, fmt.Errorf("failed to configure authentication: %w", err)
	}

	// Resolve the digest to get the full descriptor (uses cache to avoid duplicate HEAD requests)
	desc, err := cachedResolve(ctx, repo, digest)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve digest: %w", err)
	}

	// Check if this is a manifest list/image index
	if desc.MediaType != "application/vnd.docker.distribution.manifest.list.v2+json" &&
		desc.MediaType != "application/vnd.oci.image.index.v1+json" {
		// Single-arch image (just a manifest, not an index)
		// Return empty list - no platform manifests
		return []PlatformInfo{}, nil
	}

	// Fetch and parse the image index (uses cache to avoid duplicate GET requests)
	index, err := cachedFetchIndex(ctx, repo, desc)
	if err != nil {
		return nil, err // Error already wrapped by cachedFetchIndex
	}

	platforms := []PlatformInfo{}

	// Extract platform manifests
	for _, manifest := range index.Manifests {
		// Skip attestation manifests (platform unknown/unknown)
		if manifest.Platform != nil {
			if manifest.Platform.OS == "unknown" && manifest.Platform.Architecture == "unknown" {
				continue
			}
		}

		// Check for attestation annotation
		if manifest.Annotations != nil {
			if refType, ok := manifest.Annotations["vnd.docker.reference.type"]; ok {
				if refType == "attestation-manifest" {
					continue
				}
			}
		}

		// Skip if no platform info (likely an attestation)
		if manifest.Platform == nil {
			continue
		}

		// Build platform string
		platformStr := manifest.Platform.OS + "/" + manifest.Platform.Architecture
		if manifest.Platform.Variant != "" {
			platformStr += "/" + manifest.Platform.Variant
		}

		platforms = append(platforms, PlatformInfo{
			Platform:     platformStr,
			Digest:       manifest.Digest.String(),
			Size:         manifest.Size,
			OS:           manifest.Platform.OS,
			Architecture: manifest.Platform.Architecture,
			Variant:      manifest.Platform.Variant,
		})
	}

	return platforms, nil
}

// FetchArtifactContent fetches the full content of an artifact (SBOM, provenance, etc.) by digest
// Returns the parsed content as a map which can be marshaled to JSON
// The content includes all layers/blobs in the attestation manifest
func FetchArtifactContent(ctx context.Context, image, digestStr string) ([]map[string]interface{}, error) {
	// Validate inputs
	if image == "" {
		return nil, fmt.Errorf("image cannot be empty")
	}
	if digestStr == "" {
		return nil, fmt.Errorf("digest cannot be empty")
	}
	if !validateDigestFormat(digestStr) {
		return nil, fmt.Errorf("invalid digest format: %s", digestStr)
	}

	// Parse image reference
	registry, path, err := parseImageReference(image)
	if err != nil {
		return nil, err
	}

	// Create repository reference
	repo, err := remote.NewRepository(fmt.Sprintf("%s/%s", registry, path))
	if err != nil {
		return nil, fmt.Errorf("failed to create repository reference: %w", err)
	}

	// Configure authentication
	if err := configureAuth(ctx, repo); err != nil {
		return nil, fmt.Errorf("failed to configure authentication: %w", err)
	}

	// Resolve the digest to get the full descriptor (with media type)
	// ORAS Resolve can accept both tags and digests
	desc, err := repo.Resolve(ctx, digestStr)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve digest: %w", err)
	}

	// Fetch the manifest
	manifestBytes, err := repo.Fetch(ctx, desc)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch manifest: %w", err)
	}
	defer manifestBytes.Close()

	// Parse the manifest
	var manifest ocispec.Manifest
	if err := json.NewDecoder(manifestBytes).Decode(&manifest); err != nil {
		return nil, fmt.Errorf("failed to decode manifest: %w", err)
	}

	// Collect all attestation blobs
	var attestations []map[string]interface{}

	// Fetch each layer (attestations are stored in layers)
	for _, layer := range manifest.Layers {
		// Fetch the layer blob
		layerBytes, err := repo.Fetch(ctx, layer)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to fetch layer %s: %v\n", layer.Digest.String(), err)
			continue
		}
		defer layerBytes.Close()

		// Parse as JSON
		var attestation map[string]interface{}
		if err := json.NewDecoder(layerBytes).Decode(&attestation); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to decode layer %s as JSON: %v\n", layer.Digest.String(), err)
			continue
		}

		attestations = append(attestations, attestation)
	}

	if len(attestations) == 0 {
		return nil, fmt.Errorf("no attestation content found in artifact")
	}

	return attestations, nil
}

// FetchImageConfig fetches the image config blob which contains labels and other metadata
func FetchImageConfig(ctx context.Context, image, digestStr string) (*ocispec.Image, error) {
	// Validate inputs
	if image == "" {
		return nil, fmt.Errorf("image cannot be empty")
	}
	if digestStr == "" {
		return nil, fmt.Errorf("digest cannot be empty")
	}
	if !validateDigestFormat(digestStr) {
		return nil, fmt.Errorf("invalid digest format: %s", digestStr)
	}

	// Parse image reference
	registry, path, err := parseImageReference(image)
	if err != nil {
		return nil, err
	}

	// Create repository reference
	repo, err := remote.NewRepository(fmt.Sprintf("%s/%s", registry, path))
	if err != nil {
		return nil, fmt.Errorf("failed to create repository reference: %w", err)
	}

	// Configure authentication
	if err := configureAuth(ctx, repo); err != nil {
		return nil, fmt.Errorf("failed to configure authentication: %w", err)
	}

	// Resolve the digest to get the full descriptor
	desc, err := repo.Resolve(ctx, digestStr)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve digest: %w", err)
	}

	// Fetch the manifest
	manifestReader, err := repo.Fetch(ctx, desc)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch manifest: %w", err)
	}
	defer manifestReader.Close()

	// Read manifest into memory so we can parse it multiple times
	var manifestData []byte
	manifestData, err = io.ReadAll(manifestReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest: %w", err)
	}

	// Check if this is an Image Index (multi-arch) or a simple Manifest
	// Try to parse as Index first
	var index ocispec.Index
	if err := json.Unmarshal(manifestData, &index); err == nil && index.MediaType == ocispec.MediaTypeImageIndex {
		// This is an Image Index - get the first platform manifest
		if len(index.Manifests) == 0 {
			return nil, fmt.Errorf("image index has no manifests")
		}

		// Fetch the first platform manifest
		platformDesc := index.Manifests[0]
		platformManifestBytes, err := repo.Fetch(ctx, platformDesc)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch platform manifest: %w", err)
		}
		defer platformManifestBytes.Close()

		// Parse platform manifest
		var platformManifest ocispec.Manifest
		if err := json.NewDecoder(platformManifestBytes).Decode(&platformManifest); err != nil {
			return nil, fmt.Errorf("failed to decode platform manifest: %w", err)
		}

		// Fetch the config blob from the platform manifest
		configBytes, err := repo.Fetch(ctx, platformManifest.Config)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch config blob: %w", err)
		}
		defer configBytes.Close()

		// Parse the config as an OCI Image config
		var imageConfig ocispec.Image
		if err := json.NewDecoder(configBytes).Decode(&imageConfig); err != nil {
			return nil, fmt.Errorf("failed to decode image config: %w", err)
		}

		return &imageConfig, nil
	}

	// Not an index, treat as regular manifest
	var manifest ocispec.Manifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		return nil, fmt.Errorf("failed to decode manifest: %w", err)
	}

	// Fetch the config blob
	configBytes, err := repo.Fetch(ctx, manifest.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch config blob: %w", err)
	}
	defer configBytes.Close()

	// Parse the config as an OCI Image config
	var imageConfig ocispec.Image
	if err := json.NewDecoder(configBytes).Decode(&imageConfig); err != nil {
		return nil, fmt.Errorf("failed to decode image config: %w", err)
	}

	return &imageConfig, nil
}

// CopyTag copies a tag by fetching the manifest from the source tag and pushing it with the destination tag
// This effectively creates a new tag pointing to the same digest as the source tag
func CopyTag(ctx context.Context, image, sourceTag, destTag string) error {
	// Validate inputs
	if image == "" {
		return fmt.Errorf("image cannot be empty")
	}
	if sourceTag == "" {
		return fmt.Errorf("source tag cannot be empty")
	}
	if destTag == "" {
		return fmt.Errorf("destination tag cannot be empty")
	}

	// Parse image reference
	registry, path, err := parseImageReference(image)
	if err != nil {
		return err
	}

	// Create repository reference
	repo, err := remote.NewRepository(fmt.Sprintf("%s/%s", registry, path))
	if err != nil {
		return fmt.Errorf("failed to create repository reference: %w", err)
	}

	// Configure authentication
	if err := configureAuth(ctx, repo); err != nil {
		return fmt.Errorf("failed to configure authentication: %w", err)
	}

	// Resolve the source tag to get its descriptor
	sourceDesc, err := repo.Resolve(ctx, sourceTag)
	if err != nil {
		return fmt.Errorf("failed to resolve source tag '%s': %w", sourceTag, err)
	}

	// Tag the descriptor with the destination tag
	// This creates a new tag reference pointing to the same digest
	err = repo.Tag(ctx, sourceDesc, destTag)
	if err != nil {
		return fmt.Errorf("failed to tag with '%s': %w", destTag, err)
	}

	return nil
}

// getOrCreateAuthClient returns a cached auth client or creates a new one
// This ensures token caching across multiple ORAS operations, avoiding redundant auth cycles
func getOrCreateAuthClient(ctx context.Context) *auth.Client {
	// Fast path: check if we have a cached client
	authClientCacheMu.RLock()
	if authClientCache != nil {
		client := authClientCache
		authClientCacheMu.RUnlock()
		return client
	}
	authClientCacheMu.RUnlock()

	// Slow path: create and cache the client (only done once)
	authClientCacheInit.Do(func() {
		authClientCacheMu.Lock()
		defer authClientCacheMu.Unlock()

		// Create HTTP client with optional logging
		var httpClient *http.Client
		if logging.IsLoggingEnabled(ctx) {
			httpClient = &http.Client{
				Transport: logging.NewLoggingRoundTripper(http.DefaultTransport, os.Stderr),
			}
		}

		// Get GitHub token from environment
		token := os.Getenv("GITHUB_TOKEN")
		if token == "" {
			// No token - create anonymous auth client
			authClientCache = &auth.Client{
				Cache:  auth.NewCache(),
				Client: httpClient,
			}
			return
		}

		// Configure credential store with GitHub token
		store := credentials.NewMemoryStore()

		// Store credentials for ghcr.io
		cred := auth.Credential{
			Username: "oauth2", // ghcr.io uses oauth2 as username
			Password: token,
		}

		// Store credentials (ignoring errors in initialization)
		_ = store.Put(context.Background(), "ghcr.io", cred)

		// Create auth client with credential store, cache, and logging
		// The cache persists tokens across requests, eliminating redundant auth cycles
		authClientCache = &auth.Client{
			Cache:      auth.NewCache(),
			Credential: credentials.Credential(store),
			Client:     httpClient,
		}
	})

	authClientCacheMu.RLock()
	client := authClientCache
	authClientCacheMu.RUnlock()
	return client
}

// configureAuth configures authentication for GHCR using GitHub token
// Now uses a cached auth client to avoid redundant token fetches
func configureAuth(ctx context.Context, repo *remote.Repository) error {
	repo.Client = getOrCreateAuthClient(ctx)
	return nil
}

// cachedResolve resolves a digest and caches the result to avoid redundant HEAD requests
// This eliminates duplicate repo.Resolve() calls across GetPlatformManifests and DiscoverReferrers
func cachedResolve(ctx context.Context, repo *remote.Repository, digestStr string) (ocispec.Descriptor, error) {
	// Fast path: check cache
	manifestDescCacheMu.RLock()
	if desc, found := manifestDescCache[digestStr]; found {
		manifestDescCacheMu.RUnlock()
		return desc, nil
	}
	manifestDescCacheMu.RUnlock()

	// Slow path: resolve and cache
	desc, err := repo.Resolve(ctx, digestStr)
	if err != nil {
		return ocispec.Descriptor{}, err
	}

	// Cache the result
	manifestDescCacheMu.Lock()
	manifestDescCache[digestStr] = desc
	manifestDescCacheMu.Unlock()

	return desc, nil
}

// cachedFetchIndex fetches and parses an image index, caching the result
// This eliminates duplicate repo.Fetch() calls across GetPlatformManifests and DiscoverReferrers
func cachedFetchIndex(ctx context.Context, repo *remote.Repository, desc ocispec.Descriptor) (*ocispec.Index, error) {
	digestStr := desc.Digest.String()

	// Fast path: check cache
	manifestIndexCacheMu.RLock()
	if index, found := manifestIndexCache[digestStr]; found {
		manifestIndexCacheMu.RUnlock()
		return index, nil
	}
	manifestIndexCacheMu.RUnlock()

	// Slow path: fetch, parse, and cache
	indexBytes, err := repo.Fetch(ctx, desc)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch image index: %w", err)
	}
	defer indexBytes.Close()

	// Read the index content
	var index ocispec.Index
	if err := json.NewDecoder(indexBytes).Decode(&index); err != nil {
		return nil, fmt.Errorf("failed to decode image index: %w", err)
	}

	// Cache the result
	manifestIndexCacheMu.Lock()
	manifestIndexCache[digestStr] = &index
	manifestIndexCacheMu.Unlock()

	return &index, nil
}
