package gh

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"

	"github.com/google/go-github/v58/github"
	"github.com/mkoepf/ghcrctl/internal/logging"
)

// Client wraps the GitHub API client
type Client struct {
	client *github.Client
	token  string
}

// packageDeleter defines the interface for package deletion operations.
type packageDeleter interface {
	DeletePackageVersion(ctx context.Context, owner, ownerType, packageName string, versionID int64) error
}

// packageVersionLister defines the interface for listing package versions.
type packageVersionLister interface {
	ListPackageVersions(ctx context.Context, owner, ownerType, packageName string) ([]PackageVersionInfo, error)
	GetVersionIDByDigest(ctx context.Context, owner, ownerType, packageName, digest string) (int64, error)
	GetVersionTags(ctx context.Context, owner, ownerType, packageName string, versionID int64) ([]string, error)
}

// packageClient combines all package-related operations.
type packageClient interface {
	packageDeleter
	packageVersionLister
}

// Ensure *Client implements packageClient
var _ packageClient = (*Client)(nil)

// GetToken retrieves the GitHub token from the GITHUB_TOKEN environment variable
func GetToken() (string, error) {
	token, exists := os.LookupEnv("GITHUB_TOKEN")
	if !exists {
		return "", fmt.Errorf("GITHUB_TOKEN environment variable not set")
	}

	if token == "" {
		return "", fmt.Errorf("GITHUB_TOKEN environment variable is empty")
	}

	return token, nil
}

// NewClient creates a new GitHub API client with the provided token
func NewClient(token string) (*Client, error) {
	return NewClientWithContext(context.Background(), token)
}

// NewClientWithContext creates a new GitHub API client with the provided token and context
// If logging is enabled in the context, API calls will be logged
func NewClientWithContext(ctx context.Context, token string) (*Client, error) {
	if token == "" {
		return nil, fmt.Errorf("token cannot be empty")
	}

	// Create HTTP client with logging if enabled
	var httpClient *http.Client
	if logging.IsLoggingEnabled(ctx) {
		httpClient = &http.Client{
			Transport: logging.NewLoggingRoundTripper(http.DefaultTransport, os.Stderr),
		}
	}

	// Create GitHub client with authentication
	client := github.NewClient(httpClient).WithAuthToken(token)

	return &Client{
		client: client,
		token:  token,
	}, nil
}

// ListPackages lists all container packages for the specified owner
func (c *Client) ListPackages(ctx context.Context, owner string, ownerType string) ([]string, error) {
	// Validate inputs
	if owner == "" {
		return nil, fmt.Errorf("owner cannot be empty")
	}

	if ownerType != "org" && ownerType != "user" {
		return nil, fmt.Errorf("owner type must be 'org' or 'user', got '%s'", ownerType)
	}

	// Set up options for listing packages
	opts := &github.PackageListOptions{
		PackageType: github.String("container"),
		ListOptions: github.ListOptions{PerPage: 100},
	}

	var allPackages []string

	// List packages based on owner type
	for {
		var packages []*github.Package
		var resp *github.Response
		var err error

		if ownerType == "org" {
			packages, resp, err = c.client.Organizations.ListPackages(ctx, owner, opts)
		} else {
			packages, resp, err = c.client.Users.ListPackages(ctx, owner, opts)
		}

		if err != nil {
			// Check for 400 Invalid argument error (token type limitation)
			errMsg := err.Error()
			if strings.Contains(errMsg, "400") && strings.Contains(errMsg, "Invalid argument") {
				namespace := "user namespace"
				if ownerType == "org" {
					namespace = "organization"
				}
				return nil, fmt.Errorf("cannot list packages for %s %s. Your token might be either a fine-grained personal access token or repository-scoped.\nTo list all packages, you need the read:packages scope and the %s must be readable by you.\nYou might still have access to specific packages, e.g., 'ghcrctl graph <image-name>' might work with your token",
					ownerType, owner, namespace)
			}
			return nil, fmt.Errorf("failed to list packages: %w", err)
		}

		// Extract package names
		for _, pkg := range packages {
			if pkg.Name != nil {
				allPackages = append(allPackages, *pkg.Name)
			}
		}

		// Check if there are more pages
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	// Sort packages alphabetically
	sort.Strings(allPackages)

	return allPackages, nil
}

// GetVersionIDByDigest finds the GHCR version ID for a specific OCI digest
// This allows mapping from OCI digest to GitHub package version for deletion operations
func (c *Client) GetVersionIDByDigest(ctx context.Context, owner, ownerType, packageName, digest string) (int64, error) {
	// Validate inputs
	if owner == "" {
		return 0, fmt.Errorf("owner cannot be empty")
	}
	if ownerType != "org" && ownerType != "user" {
		return 0, fmt.Errorf("owner type must be 'org' or 'user', got '%s'", ownerType)
	}
	if packageName == "" {
		return 0, fmt.Errorf("package name cannot be empty")
	}
	if digest == "" {
		return 0, fmt.Errorf("digest cannot be empty")
	}

	// List all versions for this package
	opts := &github.PackageListOptions{
		PackageType: github.String("container"),
		State:       github.String("active"),
		ListOptions: github.ListOptions{PerPage: 100},
	}

	var allVersions []*github.PackageVersion
	for {
		var versions []*github.PackageVersion
		var resp *github.Response
		var err error

		if ownerType == "org" {
			versions, resp, err = c.client.Organizations.PackageGetAllVersions(ctx, owner, "container", packageName, opts)
		} else {
			versions, resp, err = c.client.Users.PackageGetAllVersions(ctx, owner, "container", packageName, opts)
		}

		if err != nil {
			return 0, fmt.Errorf("failed to list package versions: %w", err)
		}

		allVersions = append(allVersions, versions...)

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	// Search for version with matching digest
	for _, version := range allVersions {
		if version.Name != nil && *version.Name == digest {
			if version.ID != nil {
				return *version.ID, nil
			}
		}
	}

	return 0, fmt.Errorf("no version found with digest %s", digest)
}

// GetVersionTags fetches all tags for a specific package version ID
func (c *Client) GetVersionTags(ctx context.Context, owner, ownerType, packageName string, versionID int64) ([]string, error) {
	// Validate inputs
	if owner == "" {
		return nil, fmt.Errorf("owner cannot be empty")
	}
	if ownerType != "org" && ownerType != "user" {
		return nil, fmt.Errorf("owner type must be 'org' or 'user', got '%s'", ownerType)
	}
	if packageName == "" {
		return nil, fmt.Errorf("package name cannot be empty")
	}
	if versionID == 0 {
		return nil, fmt.Errorf("version ID cannot be zero")
	}

	// Fetch version details
	var version *github.PackageVersion
	var err error

	if ownerType == "org" {
		version, _, err = c.client.Organizations.PackageGetVersion(ctx, owner, "container", packageName, versionID)
	} else {
		version, _, err = c.client.Users.PackageGetVersion(ctx, owner, "container", packageName, versionID)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get package version: %w", err)
	}

	// Extract tags from metadata
	if version.Metadata != nil && version.Metadata.Container != nil {
		return version.Metadata.Container.Tags, nil
	}

	return []string{}, nil
}

// PackageVersionInfo contains information about a package version
type PackageVersionInfo struct {
	ID        int64
	Name      string
	Tags      []string
	CreatedAt string
	UpdatedAt string
}

// ListPackageVersions lists all versions of a package
func (c *Client) ListPackageVersions(ctx context.Context, owner, ownerType, packageName string) ([]PackageVersionInfo, error) {
	// Validate inputs
	if owner == "" {
		return nil, fmt.Errorf("owner cannot be empty")
	}
	if ownerType != "org" && ownerType != "user" {
		return nil, fmt.Errorf("owner type must be 'org' or 'user', got '%s'", ownerType)
	}
	if packageName == "" {
		return nil, fmt.Errorf("package name cannot be empty")
	}

	// Set up options for listing versions
	opts := &github.PackageListOptions{
		PackageType: github.String("container"),
		ListOptions: github.ListOptions{PerPage: 100},
	}

	var allVersions []PackageVersionInfo

	// List versions based on owner type
	for {
		var versions []*github.PackageVersion
		var resp *github.Response
		var err error

		if ownerType == "org" {
			versions, resp, err = c.client.Organizations.PackageGetAllVersions(ctx, owner, "container", packageName, opts)
		} else {
			versions, resp, err = c.client.Users.PackageGetAllVersions(ctx, owner, "container", packageName, opts)
		}

		if err != nil {
			return nil, fmt.Errorf("failed to list package versions: %w", err)
		}

		// Extract version info
		for _, ver := range versions {
			info := PackageVersionInfo{
				ID:   *ver.ID,
				Name: *ver.Name,
			}

			// Extract tags if available
			if ver.Metadata != nil && ver.Metadata.Container != nil {
				info.Tags = ver.Metadata.Container.Tags
			}

			// Extract timestamps if available
			if ver.CreatedAt != nil {
				info.CreatedAt = ver.CreatedAt.Format("2006-01-02 15:04:05")
			}
			if ver.UpdatedAt != nil {
				info.UpdatedAt = ver.UpdatedAt.Format("2006-01-02 15:04:05")
			}

			allVersions = append(allVersions, info)
		}

		// Check if there are more pages
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return allVersions, nil
}

// GetOwnerType determines whether the given owner is a user or organization
func (c *Client) GetOwnerType(ctx context.Context, owner string) (string, error) {
	if owner == "" {
		return "", fmt.Errorf("owner cannot be empty")
	}

	user, _, err := c.client.Users.Get(ctx, owner)
	if err != nil {
		return "", fmt.Errorf("failed to get owner info: %w", err)
	}

	if user.Type != nil && *user.Type == "Organization" {
		return "org", nil
	}
	return "user", nil
}

// DeletePackageVersion deletes a specific package version
func (c *Client) DeletePackageVersion(ctx context.Context, owner, ownerType, packageName string, versionID int64) error {
	// Validate inputs
	if owner == "" {
		return fmt.Errorf("owner cannot be empty")
	}
	if ownerType != "org" && ownerType != "user" {
		return fmt.Errorf("owner type must be 'org' or 'user', got '%s'", ownerType)
	}
	if packageName == "" {
		return fmt.Errorf("package name cannot be empty")
	}
	if versionID <= 0 {
		return fmt.Errorf("version ID must be positive, got %d", versionID)
	}

	// Delete the version based on owner type
	var err error
	if ownerType == "org" {
		_, err = c.client.Organizations.PackageDeleteVersion(ctx, owner, "container", packageName, versionID)
	} else {
		_, err = c.client.Users.PackageDeleteVersion(ctx, owner, "container", packageName, versionID)
	}

	if err != nil {
		return fmt.Errorf("failed to delete version: %w", err)
	}

	return nil
}

// IsLastTaggedVersionError checks if the error is due to trying to delete the last tagged version.
func IsLastTaggedVersionError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "cannot delete the last tagged version")
}

// DeletePackage deletes an entire package (not just a version)
func (c *Client) DeletePackage(ctx context.Context, owner, ownerType, packageName string) error {
	// Validate inputs
	if owner == "" {
		return fmt.Errorf("owner cannot be empty")
	}
	if ownerType != "org" && ownerType != "user" {
		return fmt.Errorf("owner type must be 'org' or 'user', got '%s'", ownerType)
	}
	if packageName == "" {
		return fmt.Errorf("package name cannot be empty")
	}

	// Delete the package based on owner type
	var err error
	if ownerType == "org" {
		_, err = c.client.Organizations.DeletePackage(ctx, owner, "container", packageName)
	} else {
		_, err = c.client.Users.DeletePackage(ctx, owner, "container", packageName)
	}

	if err != nil {
		return fmt.Errorf("failed to delete package: %w", err)
	}

	return nil
}
