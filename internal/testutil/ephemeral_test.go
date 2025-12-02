package testutil

import (
	"regexp"
	"testing"
)

func TestGenerateEphemeralName(t *testing.T) {
	t.Parallel()

	t.Run("has correct prefix", func(t *testing.T) {
		name := GenerateEphemeralName("ghcrctl-ephemeral")
		if len(name) < len("ghcrctl-ephemeral-") {
			t.Errorf("name too short: %s", name)
		}
		if name[:18] != "ghcrctl-ephemeral-" {
			t.Errorf("expected prefix 'ghcrctl-ephemeral-', got %s", name[:18])
		}
	})

	t.Run("has 8 character random suffix", func(t *testing.T) {
		name := GenerateEphemeralName("ghcrctl-ephemeral")
		suffix := name[18:]
		if len(suffix) != 8 {
			t.Errorf("expected 8 char suffix, got %d: %s", len(suffix), suffix)
		}
		// Should be lowercase hex
		matched, _ := regexp.MatchString("^[0-9a-f]{8}$", suffix)
		if !matched {
			t.Errorf("suffix should be 8 lowercase hex chars, got: %s", suffix)
		}
	})

	t.Run("generates unique names", func(t *testing.T) {
		seen := make(map[string]bool)
		for i := 0; i < 100; i++ {
			name := GenerateEphemeralName("test")
			if seen[name] {
				t.Errorf("duplicate name generated: %s", name)
			}
			seen[name] = true
		}
	})

	t.Run("works with different prefixes", func(t *testing.T) {
		name1 := GenerateEphemeralName("foo")
		name2 := GenerateEphemeralName("bar-baz")

		if name1[:4] != "foo-" {
			t.Errorf("expected prefix 'foo-', got %s", name1[:4])
		}
		if name2[:8] != "bar-baz-" {
			t.Errorf("expected prefix 'bar-baz-', got %s", name2[:8])
		}
	})
}
