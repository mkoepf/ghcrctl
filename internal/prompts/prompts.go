// Package prompts provides interactive user confirmation utilities for
// destructive operations. It supports both simple yes/no prompts and
// type-to-confirm prompts for high-risk operations like package deletion.
package prompts

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// ConfirmWithInput prompts the user to type a specific value to confirm a dangerous operation.
// Returns true only if the user types the exact expected value.
func ConfirmWithInput(reader io.Reader, writer io.Writer, message string, expected string) (bool, error) {
	fmt.Fprintf(writer, "%s '%s': ", message, expected)

	scanner := bufio.NewScanner(reader)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return false, fmt.Errorf("failed to read input: %w", err)
		}
		// EOF or no input - default to no
		return false, nil
	}

	answer := strings.TrimSpace(scanner.Text())
	return answer == expected, nil
}

// Confirm prompts the user with a yes/no question and returns true if they answer yes
// Defaults to "no" if user just presses enter
func Confirm(reader io.Reader, writer io.Writer, message string) (bool, error) {
	fmt.Fprintf(writer, "%s [y/N]: ", message)

	scanner := bufio.NewScanner(reader)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return false, fmt.Errorf("failed to read input: %w", err)
		}
		// EOF or no input - default to no
		return false, nil
	}

	answer := strings.ToLower(strings.TrimSpace(scanner.Text()))
	return answer == "y" || answer == "yes", nil
}
