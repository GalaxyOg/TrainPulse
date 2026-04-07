package tui

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/term"
)

func Run(opts Options) error {
	if opts.Store == nil {
		return fmt.Errorf("tui: store is required")
	}
	m := newModel(opts)
	programOpts := []tea.ProgramOption{
		tea.WithInput(os.Stdin),
		tea.WithOutput(os.Stdout),
	}
	if term.IsTerminal(int(os.Stdout.Fd())) {
		programOpts = append(programOpts, tea.WithAltScreen())
	}
	p := tea.NewProgram(m, programOpts...)
	_, err := p.Run()
	return err
}
