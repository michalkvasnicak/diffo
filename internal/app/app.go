// Package app wires the CLI and its services.
package app

import (
	"fmt"
	"io"
)

// Run starts the application. Git diff and TUI functionality are not implemented yet.
func Run(out io.Writer) error {
	_, err := fmt.Fprintln(out, "diffo: Git diff TUI — coming soon")
	return err
}
