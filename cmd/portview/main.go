// Command portview is a TUI for discovering and managing localhost dev
// servers, with non-TUI subcommands (list, kill, open) for scripting.
package main

import (
	"flag"
	"fmt"
	"os"
	"slices"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jeramiahgcoffey/portview/internal/cli"
	"github.com/jeramiahgcoffey/portview/internal/config"
	"github.com/jeramiahgcoffey/portview/internal/scanner"
	"github.com/jeramiahgcoffey/portview/internal/tui"
)

// version is set at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	args := os.Args[1:]
	if len(args) > 0 && slices.Contains(cli.Commands, args[0]) {
		os.Exit(cli.Run(args, version, os.Stdout, os.Stderr))
	}

	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("portview %s\n", version)
		return
	}

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
