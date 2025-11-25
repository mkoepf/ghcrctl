package gh

import (
	"context"
	"testing"
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
				if err == nil {
					t.Errorf("Expected error containing '%s', got nil", tt.errMsg)
				}
			} else {
				if err != nil && err.Error() == "" {
					t.Error("Error message should not be empty")
				}
			}
		})
	}
}
