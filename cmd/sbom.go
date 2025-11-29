package cmd

import "github.com/spf13/cobra"

// newSBOMCmd creates the sbom command using the generic artifact factory.
func newSBOMCmd() *cobra.Command {
	return newArtifactCmd(artifactConfig{
		Name:       "sbom",
		Short:      "Display SBOM (Software Bill of Materials)",
		NoFoundMsg: "no SBOM found",
		Role:       "sbom",
		Long: `Display the SBOM for a container image. If multiple SBOMs exist, use --digest to select one or --all to show all.

Examples:
  # Show SBOM for latest tag
  ghcrctl sbom mkoepf/myimage

  # Show SBOM for specific tag
  ghcrctl sbom mkoepf/myimage:v1.0.0

  # Show specific SBOM by digest
  ghcrctl sbom mkoepf/myimage --digest abc123def456

  # Show all SBOMs
  ghcrctl sbom mkoepf/myimage --all

  # Output in JSON format
  ghcrctl sbom mkoepf/myimage --json`,
	})
}
