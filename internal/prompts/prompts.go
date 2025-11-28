package prompts

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// Prompter defines the interface for user prompts.
// This allows for dependency injection and testing with mocks.
type Prompter interface {
	Confirm(reader io.Reader, writer io.Writer, message string) (bool, error)
}

// DefaultPrompter implements Prompter using standard input/output.
type DefaultPrompter struct{}

// Confirm prompts the user with a yes/no question and returns true if they answer yes.
func (p *DefaultPrompter) Confirm(reader io.Reader, writer io.Writer, message string) (bool, error) {
	return Confirm(reader, writer, message)
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
