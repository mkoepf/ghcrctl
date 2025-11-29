package cmd

import (
	"bytes"
	"testing"
)

func TestDiscoverCmd_RequiresPackageName(t *testing.T) {
	cmd := newDiscoverCmd()
	cmd.SetArgs([]string{})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for missing package name")
	}
}

func TestDiscoverCmd_HasFlags(t *testing.T) {
	cmd := newDiscoverCmd()

	// Check --json flag exists
	jsonFlag := cmd.Flags().Lookup("json")
	if jsonFlag == nil {
		t.Error("expected --json flag")
	}

	// Check --tree flag exists
	treeFlag := cmd.Flags().Lookup("tree")
	if treeFlag == nil {
		t.Error("expected --tree flag")
	}

	// Check -o flag exists
	outputFlag := cmd.Flags().Lookup("output")
	if outputFlag == nil {
		t.Error("expected -o/--output flag")
	}
}

func TestDiscoverCmd_InvalidOutputFormat(t *testing.T) {
	cmd := newDiscoverCmd()
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"owner/test-package", "-o", "invalid"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for invalid output format")
	}
}
