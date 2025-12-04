package discover

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/credentials"

	"github.com/mkoepf/ghcrctl/internal/logging"
)

// typeResolver resolves OCI artifact types.
type typeResolver interface {
	resolveVersionInfo(ctx context.Context, image, digest string) ([]string, int64, error)
}

// orasResolver implements typeResolver using ORAS library.
type orasResolver struct {
	authClient *auth.Client
	authOnce   sync.Once
}

// newOrasResolver creates a new orasResolver.
func newOrasResolver() *orasResolver {
	return &orasResolver{}
}

// resolveVersionInfo resolves the type(s) and size of an OCI artifact by digest.
func (r *orasResolver) resolveVersionInfo(ctx context.Context, image, digest string) ([]string, int64, error) {
	if image == "" {
		return nil, 0, fmt.Errorf("image cannot be empty")
	}
	if digest == "" {
		return nil, 0, fmt.Errorf("digest cannot be empty")
	}
	if !ValidateDigestFormat(digest) {
		return nil, 0, fmt.Errorf("invalid digest format: %s", digest)
	}

	registry, path, err := ParseImageReference(image)
	if err != nil {
		return nil, 0, err
	}

	repo, err := remote.NewRepository(fmt.Sprintf("%s/%s", registry, path))
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create repository: %w", err)
	}

	r.configureAuth(ctx, repo)

	desc, err := repo.Resolve(ctx, digest)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to resolve digest: %w", err)
	}

	size := desc.Size

	// Check if index
	if desc.MediaType == ocispec.MediaTypeImageIndex ||
		desc.MediaType == "application/vnd.docker.distribution.manifest.list.v2+json" {
		return []string{"index"}, size, nil
	}

	// Fetch manifest to determine type
	manifestBytes, err := repo.Fetch(ctx, desc)
	if err != nil {
		return nil, size, fmt.Errorf("failed to fetch manifest: %w", err)
	}
	defer manifestBytes.Close()

	var manifest ocispec.Manifest
	if err := json.NewDecoder(manifestBytes).Decode(&manifest); err != nil {
		return nil, size, fmt.Errorf("failed to decode manifest: %w", err)
	}

	// Check for signature
	if isSignature(&manifest) {
		return []string{"signature"}, size, nil
	}

	// Check for attestation
	if isAttestation(&manifest) {
		roles := determineAttestationRoles(&manifest)
		if len(roles) == 0 {
			return []string{"attestation"}, size, nil
		}
		return roles, size, nil
	}

	// It's a platform manifest - get os/arch from config
	configBytes, err := repo.Fetch(ctx, manifest.Config)
	if err != nil {
		return []string{"manifest"}, size, nil
	}
	defer configBytes.Close()

	var imageConfig ocispec.Image
	if err := json.NewDecoder(configBytes).Decode(&imageConfig); err != nil {
		return []string{"manifest"}, size, nil
	}

	platform := imageConfig.OS + "/" + imageConfig.Architecture
	if imageConfig.Variant != "" {
		platform += "/" + imageConfig.Variant
	}
	return []string{platform}, size, nil
}

func (r *orasResolver) configureAuth(ctx context.Context, repo *remote.Repository) {
	r.authOnce.Do(func() {
		var httpClient *http.Client
		if logging.IsLoggingEnabled(ctx) {
			httpClient = &http.Client{
				Transport: logging.NewLoggingRoundTripper(http.DefaultTransport, os.Stderr),
			}
		}

		token := os.Getenv("GITHUB_TOKEN")
		if token == "" {
			r.authClient = &auth.Client{
				Cache:  auth.NewCache(),
				Client: httpClient,
			}
			return
		}

		store := credentials.NewMemoryStore()
		cred := auth.Credential{
			Username: "oauth2",
			Password: token,
		}
		_ = store.Put(context.Background(), "ghcr.io", cred)

		r.authClient = &auth.Client{
			Cache:      auth.NewCache(),
			Credential: credentials.Credential(store),
			Client:     httpClient,
		}
	})
	repo.Client = r.authClient
}

func isSignature(manifest *ocispec.Manifest) bool {
	for _, layer := range manifest.Layers {
		if layer.MediaType == "application/vnd.dev.cosign.simplesigning.v1+json" {
			return true
		}
	}
	return false
}

func isAttestation(manifest *ocispec.Manifest) bool {
	if strings.Contains(manifest.Config.MediaType, "in-toto") {
		return true
	}
	for _, layer := range manifest.Layers {
		if strings.Contains(layer.MediaType, "in-toto") {
			return true
		}
		if layer.Annotations != nil {
			if _, ok := layer.Annotations["in-toto.io/predicate-type"]; ok {
				return true
			}
			if _, ok := layer.Annotations["predicateType"]; ok {
				return true
			}
		}
	}
	return false
}

func determineAttestationRoles(manifest *ocispec.Manifest) []string {
	roleSet := make(map[string]bool)

	for _, layer := range manifest.Layers {
		if layer.Annotations != nil {
			if predType, ok := layer.Annotations["in-toto.io/predicate-type"]; ok {
				roleSet[predicateToRole(predType)] = true
			}
			if predType, ok := layer.Annotations["predicateType"]; ok {
				roleSet[predicateToRole(predType)] = true
			}
		}
	}

	var roles []string
	for role := range roleSet {
		roles = append(roles, role)
	}
	return roles
}

func predicateToRole(predicateType string) string {
	lower := strings.ToLower(predicateType)
	if strings.Contains(lower, "spdx") || strings.Contains(lower, "cyclonedx") {
		return "sbom"
	}
	if strings.Contains(lower, "slsa") || strings.Contains(lower, "provenance") {
		return "provenance"
	}
	if strings.Contains(lower, "vuln") {
		return "vuln-scan"
	}
	if strings.Contains(lower, "openvex") || strings.Contains(lower, "vex") {
		return "vex"
	}
	return "attestation"
}
