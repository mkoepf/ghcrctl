package cmd

import "github.com/spf13/cobra"

// newProvenanceCmd creates the provenance command using the generic artifact factory.
func newProvenanceCmd() *cobra.Command {
	return newArtifactCmd(artifactConfig{
		Name:       "provenance",
		Short:      "Display provenance attestation",
		NoFoundMsg: "no provenance found",
		Role:       "provenance",
		Long: `Display the provenance attestation for a container image. If multiple provenance documents exist, use --digest to select one or --all to show all.

Examples:
  # Show provenance for latest tag
  ghcrctl provenance mkoepf/myimage

  # Show provenance for specific tag
  ghcrctl provenance mkoepf/myimage:v1.0.0

  # Show specific provenance by digest
  ghcrctl provenance mkoepf/myimage --digest abc123def456

  # Show all provenance documents
  ghcrctl provenance mkoepf/myimage --all

  # Output in JSON format
  ghcrctl provenance mkoepf/myimage --json`,
	})
}
