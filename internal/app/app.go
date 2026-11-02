// Package app wires the CLI and its services.
package app

import (
	"fmt"
	"os"

	"diffo/internal/git"
	"diffo/internal/highlight"
	"diffo/internal/tui"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/term"
)

// Run starts the terminal-only, read-only change browser.
func Run() error {
	if !term.IsTerminal(os.Stdin.Fd()) || !term.IsTerminal(os.Stdout.Fd()) {
		return fmt.Errorf("diffo requires an interactive terminal on stdin and stdout")
	}
	directory, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}
	model := tui.New(func() (*git.Repository, error) {
		return git.Discover(directory)
	}, highlight.New())
	program := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := program.Run(); err != nil {
		return fmt.Errorf("run terminal UI: %w", err)
	}
	return nil
}
