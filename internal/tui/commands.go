package tui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jeramiahgcoffey/portview/internal/config"
	"github.com/jeramiahgcoffey/portview/internal/scanner"
)

// scanResultMsg carries the outcome of a scan back to the model.
type scanResultMsg struct {
	servers []scanner.Server
	err     error
	at      time.Time
}

// tickMsg fires on each poll-loop interval.
type tickMsg time.Time

// openResultMsg reports the outcome of opening a port in the browser.
type openResultMsg struct {
	port int
	err  error
}

// killResultMsg reports the outcome of signaling a process.
type killResultMsg struct {
	pid int
	err error
}

// saveResultMsg reports the outcome of persisting config.
type saveResultMsg struct {
	err error
}

// scanCmd runs a single scan and merges config (hidden filter + labels) into
// the result. It is fired both by the poll ticker and by manual refresh.
func scanCmd(s scanner.Scanner, cfg config.Config) tea.Cmd {
	return func() tea.Msg {
		servers, err := s.Scan(context.Background())
		if err == nil {
			servers = applyConfig(servers, cfg)
		}
		return scanResultMsg{servers: servers, err: err, at: time.Now()}
	}
}

// tickCmd schedules the next poll-loop tick.
func tickCmd(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// openCmd opens http://localhost:<port> in the default browser.
func openCmd(port int) tea.Cmd {
	return func() tea.Msg {
		err := browserOpen(fmt.Sprintf("http://localhost:%d", port))
		return openResultMsg{port: port, err: err}
	}
}

// killCmd sends SIGTERM to a process.
func killCmd(pid int) tea.Cmd {
	return func() tea.Msg {
		return killResultMsg{pid: pid, err: killProcess(pid)}
	}
}

// saveCmd persists the config to disk (lazily creating the file).
func saveCmd(cfg config.Config) tea.Cmd {
	return func() tea.Msg {
		return saveResultMsg{err: cfg.Save()}
	}
}

// applyConfig is the app-logic merge step: drop hidden ports and attach the
// user's saved label to each remaining server.
func applyConfig(in []scanner.Server, cfg config.Config) []scanner.Server {
	out := make([]scanner.Server, 0, len(in))
	for _, s := range in {
		if cfg.IsHidden(s.Port) {
			continue
		}
		s.Label = cfg.LabelFor(s.Port)
		out = append(out, s)
	}
	return out
}

// browserOpen launches the platform's default URL opener. Opening a browser is
// an app action, not port discovery, so a runtime OS switch is appropriate
// here (the platform abstraction lives in the scanner layer).
func browserOpen(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}

// killProcess sends a graceful termination signal to pid.
func killProcess(pid int) error {
	p, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return p.Signal(syscall.SIGTERM)
}
