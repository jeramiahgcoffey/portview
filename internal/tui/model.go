// Package tui implements portview's terminal UI with Bubble Tea. It depends on
// the scanner and config packages but never on a specific platform: the
// scanner.Scanner interface hides that detail entirely.
package tui

import (
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/jeramiahgcoffey/portview/internal/config"
	"github.com/jeramiahgcoffey/portview/internal/scanner"
)

// mode is the current interaction mode of the TUI.
type mode int

const (
	modeNormal      mode = iota // browsing the list
	modeConfirmKill             // awaiting y/n to kill a process
	modeLabel                   // editing a label inline
	modeFilter                  // typing a live filter
)

// Model is the Bubble Tea model for portview.
type Model struct {
	scanner scanner.Scanner
	cfg     config.Config
	keys    keyMap
	help    help.Model

	servers  []scanner.Server // full decorated list (post hidden-filter + labels)
	cursor   int              // index into the visible (filtered) list
	lastScan time.Time
	scanErr  error

	mode        mode
	killTarget  scanner.Server  // process awaiting kill confirmation
	editPort    int             // port whose label is being edited
	labelInput  textinput.Model // inline label editor
	filterInput textinput.Model // live filter input
	filter      string          // active filter query (persists after Enter)
	showHelp    bool            // full help overlay visible
	status      string          // transient feedback line

	width  int
	height int
}

// New constructs a Model wired to a scanner and config.
func New(s scanner.Scanner, cfg config.Config) Model {
	label := textinput.New()
	label.Prompt = ""
	label.CharLimit = 24

	filter := textinput.New()
	filter.Prompt = "/"
	filter.Placeholder = "port, process, or label"

	return Model{
		scanner:     s,
		cfg:         cfg,
		keys:        defaultKeys(),
		help:        help.New(),
		labelInput:  label,
		filterInput: filter,
	}
}

// Init kicks off the first scan and starts the poll-loop ticker.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		scanCmd(m.scanner, m.cfg),
		tickCmd(m.cfg.RefreshInterval.Std()),
	)
}

// Update handles messages: window resize, poll ticks, scan/action results, and
// keys (dispatched by mode).
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.help.Width = msg.Width
		return m, nil

	case tickMsg:
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

	case openResultMsg:
		if msg.err != nil {
			m.status = "open failed: " + msg.err.Error()
		} else {
			m.status = "opened localhost:" + strconv.Itoa(msg.port)
		}
		return m, nil

	case killResultMsg:
		if msg.err != nil {
			m.status = "kill failed: " + msg.err.Error()
			return m, nil
		}
		m.status = "sent SIGTERM to PID " + strconv.Itoa(msg.pid)
		// Refresh immediately so the killed server drops out of the list.
		return m, scanCmd(m.scanner, m.cfg)

	case saveResultMsg:
		if msg.err != nil {
			m.status = "save failed: " + msg.err.Error()
		}
		return m, nil

	case tea.KeyMsg:
		// Ctrl+C always quits, regardless of mode.
		if msg.Type == tea.KeyCtrlC {
			return m, tea.Quit
		}
		switch m.mode {
		case modeConfirmKill:
			return m.handleConfirmKill(msg)
		case modeLabel:
			return m.handleLabel(msg)
		case modeFilter:
			return m.handleFilter(msg)
		default:
			return m.handleNormal(msg)
		}
	}

	return m, nil
}

// handleNormal processes keys while browsing the list.
func (m Model) handleNormal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// While the help overlay is open, only help/quit/esc are meaningful.
	if m.showHelp {
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, m.keys.Help), msg.Type == tea.KeyEsc:
			m.showHelp = false
		}
		return m, nil
	}

	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit

	case key.Matches(msg, m.keys.Help):
		m.showHelp = true
		return m, nil

	case key.Matches(msg, m.keys.Up):
		m.status = ""
		m.moveCursor(-1)
		return m, nil

	case key.Matches(msg, m.keys.Down):
		m.status = ""
		m.moveCursor(1)
		return m, nil

	case key.Matches(msg, m.keys.Refresh):
		m.status = ""
		return m, scanCmd(m.scanner, m.cfg)

	case key.Matches(msg, m.keys.Open):
		if s, ok := m.selected(); ok {
			return m, openCmd(s.Port)
		}
		return m, nil

	case key.Matches(msg, m.keys.Kill):
		if s, ok := m.selected(); ok {
			m.mode = modeConfirmKill
			m.killTarget = s
		}
		return m, nil

	case key.Matches(msg, m.keys.Label):
		if s, ok := m.selected(); ok {
			m.mode = modeLabel
			m.editPort = s.Port
			m.labelInput.SetValue(s.Label)
			m.labelInput.CursorEnd()
			m.labelInput.Focus()
		}
		return m, nil

	case key.Matches(msg, m.keys.Filter):
		m.mode = modeFilter
		m.filterInput.SetValue(m.filter)
		m.filterInput.CursorEnd()
		m.filterInput.Focus()
		return m, nil

	case msg.Type == tea.KeyEsc:
		// Esc clears an active filter.
		if m.filter != "" {
			m.filter = ""
			m.clampCursor()
		}
		return m, nil
	}

	return m, nil
}

// handleConfirmKill processes the y/n kill confirmation.
func (m Model) handleConfirmKill(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		pid := m.killTarget.PID
		m.mode = modeNormal
		return m, killCmd(pid)
	case "n", "N", "esc", "q":
		m.mode = modeNormal
		m.status = "kill cancelled"
		return m, nil
	}
	return m, nil
}

// handleLabel processes inline label editing.
func (m Model) handleLabel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		value := strings.TrimSpace(m.labelInput.Value())
		m.cfg.SetLabel(m.editPort, value)
		m.servers = applyConfig(m.servers, m.cfg)
		m.mode = modeNormal
		m.labelInput.Blur()
		if value == "" {
			m.status = "label cleared"
		} else {
			m.status = "labeled :" + strconv.Itoa(m.editPort) + " " + value
		}
		return m, saveCmd(m.cfg)

	case tea.KeyEsc:
		m.mode = modeNormal
		m.labelInput.Blur()
		return m, nil
	}

	var cmd tea.Cmd
	m.labelInput, cmd = m.labelInput.Update(msg)
	return m, cmd
}

// handleFilter processes the live filter input.
func (m Model) handleFilter(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		// Keep the filter applied and return to browsing.
		m.mode = modeNormal
		m.filterInput.Blur()
		return m, nil

	case tea.KeyEsc:
		// Cancel: clear the filter entirely.
		m.mode = modeNormal
		m.filter = ""
		m.filterInput.SetValue("")
		m.filterInput.Blur()
		m.clampCursor()
		return m, nil
	}

	var cmd tea.Cmd
	m.filterInput, cmd = m.filterInput.Update(msg)
	m.filter = m.filterInput.Value()
	m.clampCursor()
	return m, cmd
}

// visibleServers returns the servers matching the active filter.
func (m Model) visibleServers() []scanner.Server {
	if m.filter == "" {
		return m.servers
	}
	q := strings.ToLower(m.filter)
	out := make([]scanner.Server, 0, len(m.servers))
	for _, s := range m.servers {
		if matchesFilter(s, q) {
			out = append(out, s)
		}
	}
	return out
}

// matchesFilter reports whether a server matches a lowercased query against its
// port, process name, or label.
func matchesFilter(s scanner.Server, q string) bool {
	return strings.Contains(strconv.Itoa(s.Port), q) ||
		strings.Contains(strings.ToLower(s.Process), q) ||
		strings.Contains(strings.ToLower(s.Label), q)
}

// moveCursor moves the selection by delta, clamped to the visible list.
func (m *Model) moveCursor(delta int) {
	m.cursor += delta
	m.clampCursor()
}

// clampCursor keeps the cursor within [0, len-1] of the visible list.
func (m *Model) clampCursor() {
	n := len(m.visibleServers())
	if n == 0 {
		m.cursor = 0
		return
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= n {
		m.cursor = n - 1
	}
}

// selected returns the server under the cursor and whether one exists.
func (m Model) selected() (scanner.Server, bool) {
	vis := m.visibleServers()
	if m.cursor < 0 || m.cursor >= len(vis) {
		return scanner.Server{}, false
	}
	return vis[m.cursor], true
}
