// Package tui implements portview's terminal UI with Bubble Tea. It depends on
// the scanner and config packages but never on a specific platform: the
// scanner.Scanner interface hides that detail entirely.
package tui

import (
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/jeramiahgcoffey/portview/internal/config"
	"github.com/jeramiahgcoffey/portview/internal/scanner"
)

// Model is the Bubble Tea model for portview.
type Model struct {
	scanner scanner.Scanner
	cfg     config.Config
	keys    keyMap

	servers  []scanner.Server
	cursor   int
	lastScan time.Time
	scanErr  error

	width  int
	height int
}

// New constructs a Model wired to a scanner and config.
func New(s scanner.Scanner, cfg config.Config) Model {
	return Model{
		scanner: s,
		cfg:     cfg,
		keys:    defaultKeys(),
	}
}

// Init kicks off the first scan and starts the poll-loop ticker.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		scanCmd(m.scanner, m.cfg),
		tickCmd(m.cfg.RefreshInterval.Std()),
	)
}

// Update handles messages: window resize, poll ticks, scan results, and keys.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tickMsg:
		// Fire a scan and schedule the next tick.
		return m, tea.Batch(
			scanCmd(m.scanner, m.cfg),
			tickCmd(m.cfg.RefreshInterval.Std()),
		)

	case scanResultMsg:
		m.scanErr = msg.err
		if msg.err == nil {
			m.servers = msg.servers
			m.lastScan = msg.at
			m.clampCursor()
		}
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

// handleKey processes a key press in normal mode. Action modes (kill confirm,
// label edit, filter) are added in a later phase.
func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit

	case key.Matches(msg, m.keys.Up):
		m.moveCursor(-1)
		return m, nil

	case key.Matches(msg, m.keys.Down):
		m.moveCursor(1)
		return m, nil

	case key.Matches(msg, m.keys.Refresh):
		// Immediate scan outside the ticker.
		return m, scanCmd(m.scanner, m.cfg)
	}

	return m, nil
}

// moveCursor moves the selection by delta, clamped to the list bounds.
func (m *Model) moveCursor(delta int) {
	m.cursor += delta
	m.clampCursor()
}

// clampCursor keeps the cursor within [0, len-1], or 0 when the list is empty.
func (m *Model) clampCursor() {
	if len(m.servers) == 0 {
		m.cursor = 0
		return
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= len(m.servers) {
		m.cursor = len(m.servers) - 1
	}
}
