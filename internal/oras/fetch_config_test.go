package oras

import (
	"context"
	"testing"
)

// TestFetchImageConfig validates input parameters
func TestFetchImageConfig(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name    string
		image   string
		digest  string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "empty image",
			image:   "",
			digest:  "sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
			wantErr: true,
			errMsg:  "image cannot be empty",
		},
		{
			name:    "empty digest",
			image:   "ghcr.io/owner/repo",
			digest:  "",
			wantErr: true,
			errMsg:  "digest cannot be empty",
		},
		{
			name:    "invalid digest format",
			image:   "ghcr.io/owner/repo",
			digest:  "invalid",
			wantErr: true,
			errMsg:  "invalid digest format",
		},
		{
			name:    "invalid image format",
			image:   "invalid-image",
			digest:  "sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
			wantErr: true,
			errMsg:  "invalid image format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := FetchImageConfig(ctx, tt.image, tt.digest)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error containing '%s', got nil", tt.errMsg)
				} else if err.Error() == "" {
					t.Error("Error message should not be empty")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got %v", err)
				}
			}
		})
	}
}
