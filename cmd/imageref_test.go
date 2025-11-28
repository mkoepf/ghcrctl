package cmd

import "testing"

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
