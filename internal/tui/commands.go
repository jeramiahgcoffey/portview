package tui

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jeramiahcoffey/portview/internal/scanner"
)

// scanResultMsg carries the result of a background scan.
type scanResultMsg struct {
	servers []scanner.Server
	err     error
}

// tickMsg signals that the refresh interval has elapsed.
type tickMsg time.Time

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
