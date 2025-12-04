package testutil

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateEphemeralName(t *testing.T) {
	t.Parallel()

	t.Run("has correct prefix", func(t *testing.T) {
		name := GenerateEphemeralName("ghcrctl-ephemeral")
		assert.GreaterOrEqual(t, len(name), len("ghcrctl-ephemeral-"), "name too short: %s", name)
		assert.Equal(t, "ghcrctl-ephemeral-", name[:18])
	})

	t.Run("has 8 character random suffix", func(t *testing.T) {
		name := GenerateEphemeralName("ghcrctl-ephemeral")
		suffix := name[18:]
		assert.Len(t, suffix, 8, "expected 8 char suffix")
		// Should be lowercase hex
		matched, _ := regexp.MatchString("^[0-9a-f]{8}$", suffix)
		assert.True(t, matched, "suffix should be 8 lowercase hex chars, got: %s", suffix)
	})

	t.Run("generates unique names", func(t *testing.T) {
		seen := make(map[string]bool)
		for i := 0; i < 100; i++ {
			name := GenerateEphemeralName("test")
			assert.False(t, seen[name], "duplicate name generated: %s", name)
			seen[name] = true
		}
	})

	t.Run("works with different prefixes", func(t *testing.T) {
		name1 := GenerateEphemeralName("foo")
		name2 := GenerateEphemeralName("bar-baz")

		assert.Equal(t, "foo-", name1[:4])
		assert.Equal(t, "bar-baz-", name2[:8])
	})
}
