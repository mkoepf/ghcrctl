package cmd

import (
	"fmt"
	"strings"
)

// parseImageRef parses an image reference in the format owner/image[:tag]
// Returns owner, image, tag (empty if not specified), and error
func parseImageRef(ref string) (owner, image, tag string, err error) {
	if ref == "" {
		return "", "", "", fmt.Errorf("image reference cannot be empty")
	}

	// Split on slash to get owner and image[:tag]
	slashIdx := strings.Index(ref, "/")
	if slashIdx == -1 {
		return "", "", "", fmt.Errorf("invalid image reference %q: must be in format owner/image[:tag]", ref)
	}

	owner = ref[:slashIdx]
	remainder := ref[slashIdx+1:]

	if owner == "" {
		return "", "", "", fmt.Errorf("invalid image reference %q: owner cannot be empty", ref)
	}

	if remainder == "" {
		return "", "", "", fmt.Errorf("invalid image reference %q: image cannot be empty", ref)
	}

	// Split remainder on colon to get image and optional tag
	colonIdx := strings.Index(remainder, ":")
	if colonIdx == -1 {
		// No tag specified
		image = remainder
		tag = ""
	} else {
		image = remainder[:colonIdx]
		tag = remainder[colonIdx+1:]

		if image == "" {
			return "", "", "", fmt.Errorf("invalid image reference %q: image cannot be empty", ref)
		}
		if tag == "" {
			return "", "", "", fmt.Errorf("invalid image reference %q: tag cannot be empty after colon", ref)
		}
	}

	return owner, image, tag, nil
}
