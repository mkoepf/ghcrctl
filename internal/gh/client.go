package gh

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/google/go-github/v58/github"
)

// Client wraps the GitHub API client
type Client struct {
	client *github.Client
	token  string
}

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
	if token == "" {
		return nil, fmt.Errorf("token cannot be empty")
	}

	// Create GitHub client with authentication
	client := github.NewClient(nil).WithAuthToken(token)

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
	sortPackages(allPackages)

	return allPackages, nil
}

// sortPackages sorts a slice of package names alphabetically
func sortPackages(packages []string) {
	// Simple bubble sort (good enough for small lists)
	n := len(packages)
	for i := 0; i < n-1; i++ {
		for j := 0; j < n-i-1; j++ {
			if packages[j] > packages[j+1] {
				packages[j], packages[j+1] = packages[j+1], packages[j]
			}
		}
	}
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

// AddTagToVersion adds a new tag to an existing package version
// It retrieves the current tags for the version and adds the new tag to the list
func (c *Client) AddTagToVersion(ctx context.Context, owner, ownerType, packageName string, versionID int64, newTag string) error {
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
	if versionID == 0 {
		return fmt.Errorf("version ID cannot be zero")
	}
	if newTag == "" {
		return fmt.Errorf("new tag cannot be empty")
	}

	// First, get the current version to retrieve existing tags
	var version *github.PackageVersion
	var err error

	if ownerType == "org" {
		version, _, err = c.client.Organizations.PackageGetVersion(ctx, owner, "container", packageName, versionID)
	} else {
		version, _, err = c.client.Users.PackageGetVersion(ctx, owner, "container", packageName, versionID)
	}

	if err != nil {
		return fmt.Errorf("failed to get package version: %w", err)
	}

	// Build list of tags including the new one
	existingTags := []string{}
	if version.Metadata != nil && version.Metadata.Container != nil && version.Metadata.Container.Tags != nil {
		existingTags = version.Metadata.Container.Tags
	}

	// Check if tag already exists
	for _, tag := range existingTags {
		if tag == newTag {
			return fmt.Errorf("tag '%s' already exists on this version", newTag)
		}
	}

	// Add new tag
	updatedTags := append(existingTags, newTag)

	// Prepare the update request body
	type updateRequest struct {
		Metadata struct {
			Container struct {
				Tags []string `json:"tags"`
			} `json:"container"`
		} `json:"metadata"`
	}

	reqBody := updateRequest{}
	reqBody.Metadata.Container.Tags = updatedTags

	// Build the URL for the PATCH request
	var url string
	if ownerType == "org" {
		url = fmt.Sprintf("orgs/%s/packages/container/%s/versions/%d", owner, packageName, versionID)
	} else {
		url = fmt.Sprintf("user/packages/container/%s/versions/%d", packageName, versionID)
	}

	// Create the PATCH request
	req, err := c.client.NewRequest("PATCH", url, reqBody)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Execute the request
	_, err = c.client.Do(ctx, req, nil)
	if err != nil {
		return fmt.Errorf("failed to update package version: %w", err)
	}

	return nil
}
