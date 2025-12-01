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

// parsePackageRef parses a package reference in the format owner/package
// It rejects inline tags - use selector flags (--tag, --digest, --version) instead.
// Returns owner, package name, and error
func parsePackageRef(ref string) (owner, packageName string, err error) {
	if ref == "" {
		return "", "", fmt.Errorf("package reference cannot be empty")
	}

	// Check for inline tag (not allowed in new CLI design)
	if strings.Contains(ref, ":") {
		return "", "", fmt.Errorf("inline tags not supported in package reference %q\nUse selector flags instead: --tag, --digest, or --version", ref)
	}

	// Split on slash to get owner and package
	slashIdx := strings.Index(ref, "/")
	if slashIdx == -1 {
		return "", "", fmt.Errorf("invalid package reference %q: must be in format owner/package", ref)
	}

	owner = ref[:slashIdx]
	packageName = ref[slashIdx+1:]

	if owner == "" {
		return "", "", fmt.Errorf("invalid package reference %q: owner cannot be empty", ref)
	}

	if packageName == "" {
		return "", "", fmt.Errorf("invalid package reference %q: package cannot be empty", ref)
	}

	return owner, packageName, nil
}
