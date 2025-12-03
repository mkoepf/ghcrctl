package cmd

import (
	"fmt"
	"strings"
)

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
