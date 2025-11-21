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
