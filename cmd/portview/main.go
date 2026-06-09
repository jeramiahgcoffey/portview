// Command portview is a TUI for discovering and managing localhost dev servers.
package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jeramiahgcoffey/portview/internal/config"
	"github.com/jeramiahgcoffey/portview/internal/scanner"
	"github.com/jeramiahgcoffey/portview/internal/tui"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "portview: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	s := scanner.New(scanner.Options{
		MinPort: cfg.PortRange.Min,
		MaxPort: cfg.PortRange.Max,
	})

	p := tea.NewProgram(tui.New(s, cfg), tea.WithAltScreen())
	_, err = p.Run()
	return err
}
