package cmd

import "testing"

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
				if err == nil {
					t.Errorf("parsePackageRef(%q) expected error containing %q, got none", tt.input, tt.errContains)
					return
				}
				if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("parsePackageRef(%q) error = %q, want error containing %q", tt.input, err.Error(), tt.errContains)
				}
				return
			}

			if err != nil {
				t.Errorf("parsePackageRef(%q) unexpected error: %v", tt.input, err)
				return
			}

			if owner != tt.wantOwner {
				t.Errorf("parsePackageRef(%q) owner = %q, want %q", tt.input, owner, tt.wantOwner)
			}
			if pkg != tt.wantPackage {
				t.Errorf("parsePackageRef(%q) package = %q, want %q", tt.input, pkg, tt.wantPackage)
			}
		})
	}
}
