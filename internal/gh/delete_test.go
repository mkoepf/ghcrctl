package gh

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestDeletePackageVersion validates input parameters
func TestDeletePackageVersion(t *testing.T) {
	ctx := context.Background()
	client := &Client{} // Empty client for validation tests

	tests := []struct {
		name        string
		owner       string
		ownerType   string
		packageName string
		versionID   int64
		wantErr     bool
		errMsg      string
	}{
		{
			name:        "empty owner",
			owner:       "",
			ownerType:   "user",
			packageName: "mypackage",
			versionID:   12345,
			wantErr:     true,
			errMsg:      "owner cannot be empty",
		},
		{
			name:        "invalid owner type",
			owner:       "testowner",
			ownerType:   "invalid",
			packageName: "mypackage",
			versionID:   12345,
			wantErr:     true,
			errMsg:      "owner type must be",
		},
		{
			name:        "empty package name",
			owner:       "testowner",
			ownerType:   "user",
			packageName: "",
			versionID:   12345,
			wantErr:     true,
			errMsg:      "package name cannot be empty",
		},
		{
			name:        "zero version ID",
			owner:       "testowner",
			ownerType:   "user",
			packageName: "mypackage",
			versionID:   0,
			wantErr:     true,
			errMsg:      "version ID must be positive",
		},
		{
			name:        "negative version ID",
			owner:       "testowner",
			ownerType:   "user",
			packageName: "mypackage",
			versionID:   -1,
			wantErr:     true,
			errMsg:      "version ID must be positive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := client.DeletePackageVersion(ctx, tt.owner, tt.ownerType, tt.packageName, tt.versionID)

			if tt.wantErr {
				assert.Error(t, err)
			} else if err != nil {
				assert.NotEmpty(t, err.Error(), "Error message should not be empty")
			}
		})
	}
}

// TestDeletePackage validates input parameters for package deletion
func TestDeletePackage(t *testing.T) {
	ctx := context.Background()
	client := &Client{} // Empty client for validation tests

	tests := []struct {
		name        string
		owner       string
		ownerType   string
		packageName string
		wantErr     bool
		errMsg      string
	}{
		{
			name:        "empty owner",
			owner:       "",
			ownerType:   "user",
			packageName: "mypackage",
			wantErr:     true,
			errMsg:      "owner cannot be empty",
		},
		{
			name:        "invalid owner type",
			owner:       "testowner",
			ownerType:   "invalid",
			packageName: "mypackage",
			wantErr:     true,
			errMsg:      "owner type must be",
		},
		{
			name:        "empty package name",
			owner:       "testowner",
			ownerType:   "user",
			packageName: "",
			wantErr:     true,
			errMsg:      "package name cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := client.DeletePackage(ctx, tt.owner, tt.ownerType, tt.packageName)

			if tt.wantErr {
				assert.Error(t, err)
			}
		})
	}
}

// TestIsLastTaggedVersionError tests detection of the "last tagged version" error
func TestIsLastTaggedVersionError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "unrelated error",
			err:      errors.New("some other error"),
			expected: false,
		},
		{
			name:     "exact GitHub error message",
			err:      errors.New("You cannot delete the last tagged version of a package"),
			expected: true,
		},
		{
			name:     "wrapped error",
			err:      fmt.Errorf("failed to delete: %w", errors.New("cannot delete the last tagged version")),
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsLastTaggedVersionError(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}
