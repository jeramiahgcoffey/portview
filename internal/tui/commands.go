package tui

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jeramiahcoffey/portview/internal/config"
	"github.com/jeramiahcoffey/portview/internal/scanner"
)

// scanResultMsg carries the result of a background scan.
type scanResultMsg struct {
	servers []scanner.Server
	err     error
}

// tickMsg signals that the refresh interval has elapsed.
type tickMsg time.Time

// killResultMsg carries the result of a kill operation.
type killResultMsg struct {
	pid int
	err error
}

// labelSavedMsg signals that config was saved (or failed).
type labelSavedMsg struct {
	err error
}

// doScan returns a command that performs a port scan using the model's scanner.
func (m Model) doScan() tea.Cmd {
	s := m.scanner
	return func() tea.Msg {
		servers, err := s.Scan(context.Background())
		return scanResultMsg{servers: servers, err: err}
	}
}

// doTick returns a command that waits for the given interval then sends a tickMsg.
func doTick(interval time.Duration) tea.Cmd {
	return tea.Tick(interval, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// doKill sends SIGTERM to the given PID.
func doKill(pid int) tea.Cmd {
	return func() tea.Msg {
		err := syscall.Kill(pid, syscall.SIGTERM)
		return killResultMsg{pid: pid, err: err}
	}
}

// doOpen opens a URL in the default browser.
func doOpen(port int) tea.Cmd {
	return func() tea.Msg {
		url := fmt.Sprintf("http://localhost:%d", port)
		var cmd *exec.Cmd
		switch runtime.GOOS {
		case "darwin":
			cmd = exec.Command("open", url)
		default:
			cmd = exec.Command("xdg-open", url)
		}
		_ = cmd.Start()
		return nil
	}
}

// doSaveConfig persists the config to disk.
func doSaveConfig(path string, cfg config.Config) tea.Cmd {
	return func() tea.Msg {
		err := config.Save(path, cfg)
		return labelSavedMsg{err: err}
	}
}
