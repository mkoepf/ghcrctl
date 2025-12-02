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

func TestParseImageRef(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantOwner string
		wantImage string
		wantTag   string
		wantErr   bool
	}{
		{
			name:      "owner/image",
			input:     "mkoepf/myimage",
			wantOwner: "mkoepf",
			wantImage: "myimage",
			wantTag:   "",
			wantErr:   false,
		},
		{
			name:      "owner/image:tag",
			input:     "mkoepf/myimage:v1.0",
			wantOwner: "mkoepf",
			wantImage: "myimage",
			wantTag:   "v1.0",
			wantErr:   false,
		},
		{
			name:      "owner/image:latest",
			input:     "myorg/app:latest",
			wantOwner: "myorg",
			wantImage: "app",
			wantTag:   "latest",
			wantErr:   false,
		},
		{
			name:      "missing owner",
			input:     "myimage",
			wantOwner: "",
			wantImage: "",
			wantTag:   "",
			wantErr:   true,
		},
		{
			name:      "empty string",
			input:     "",
			wantOwner: "",
			wantImage: "",
			wantTag:   "",
			wantErr:   true,
		},
		{
			name:      "only slash",
			input:     "/",
			wantOwner: "",
			wantImage: "",
			wantTag:   "",
			wantErr:   true,
		},
		{
			name:      "empty owner",
			input:     "/myimage",
			wantOwner: "",
			wantImage: "",
			wantTag:   "",
			wantErr:   true,
		},
		{
			name:      "empty image",
			input:     "mkoepf/",
			wantOwner: "",
			wantImage: "",
			wantTag:   "",
			wantErr:   true,
		},
		{
			name:      "empty tag after colon",
			input:     "mkoepf/myimage:",
			wantOwner: "",
			wantImage: "",
			wantTag:   "",
			wantErr:   true,
		},
		{
			name:      "image with dashes",
			input:     "mkoepf/my-image-name:v1.2.3",
			wantOwner: "mkoepf",
			wantImage: "my-image-name",
			wantTag:   "v1.2.3",
			wantErr:   false,
		},
		{
			name:      "image with underscores",
			input:     "my_org/my_image:tag_1",
			wantOwner: "my_org",
			wantImage: "my_image",
			wantTag:   "tag_1",
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			owner, image, tag, err := parseImageRef(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Errorf("parseImageRef(%q) expected error, got none", tt.input)
				}
				return
			}

			if err != nil {
				t.Errorf("parseImageRef(%q) unexpected error: %v", tt.input, err)
				return
			}

			if owner != tt.wantOwner {
				t.Errorf("parseImageRef(%q) owner = %q, want %q", tt.input, owner, tt.wantOwner)
			}
			if image != tt.wantImage {
				t.Errorf("parseImageRef(%q) image = %q, want %q", tt.input, image, tt.wantImage)
			}
			if tag != tt.wantTag {
				t.Errorf("parseImageRef(%q) tag = %q, want %q", tt.input, tag, tt.wantTag)
			}
		})
	}
}
