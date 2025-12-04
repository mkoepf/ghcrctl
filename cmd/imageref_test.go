package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParsePackageRef(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		input       string
		wantOwner   string
		wantPackage string
		wantErr     bool
		errContains string
	}{
		{
			name:        "valid owner/package",
			input:       "mkoepf/myimage",
			wantOwner:   "mkoepf",
			wantPackage: "myimage",
			wantErr:     false,
		},
		{
			name:        "valid org/package",
			input:       "my-org/my-package",
			wantOwner:   "my-org",
			wantPackage: "my-package",
			wantErr:     false,
		},
		{
			name:        "empty string",
			input:       "",
			wantErr:     true,
			errContains: "cannot be empty",
		},
		{
			name:        "inline tag rejected",
			input:       "owner/package:v1.0",
			wantErr:     true,
			errContains: "inline tags not supported",
		},
		{
			name:        "inline tag with latest",
			input:       "owner/package:latest",
			wantErr:     true,
			errContains: "inline tags not supported",
		},
		{
			name:        "no slash - missing owner",
			input:       "justpackage",
			wantErr:     true,
			errContains: "must be in format owner/package",
		},
		{
			name:        "empty owner",
			input:       "/mypackage",
			wantErr:     true,
			errContains: "owner cannot be empty",
		},
		{
			name:        "empty package",
			input:       "myowner/",
			wantErr:     true,
			errContains: "package cannot be empty",
		},
		{
			name:        "only slash",
			input:       "/",
			wantErr:     true,
			errContains: "owner cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			owner, pkg, err := parsePackageRef(tt.input)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.ErrorContains(t, err, tt.errContains)
				}
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantOwner, owner)
			assert.Equal(t, tt.wantPackage, pkg)
		})
	}
}
