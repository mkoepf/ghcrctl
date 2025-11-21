package gh

import (
	"context"
	"fmt"
	"os"

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

// ValidateToken validates the token by making a test API call
func (c *Client) ValidateToken(ctx context.Context) error {
	// Try to get the authenticated user as a simple validation
	_, _, err := c.client.Users.Get(ctx, "")
	if err != nil {
		return fmt.Errorf("token validation failed: %w", err)
	}

	return nil
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
