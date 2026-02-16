package tui

import (
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/jeramiahcoffey/portview/internal/config"
	"github.com/jeramiahcoffey/portview/internal/scanner"
)

type mode int

const (
	modeNormal mode = iota
	modeFilter
	modeLabel
	modeConfirmKill
	modeHelp
)

// Model is the Bubble Tea model for the portview TUI.
type Model struct {
	servers     []scanner.Server
	filtered    []scanner.Server
	cursor      int
	mode        mode
	scanner     scanner.Scanner
	config      config.Config
	configPath  string
	width       int
	height      int
	lastRefresh time.Time
	filterText  string
	labelInput  textinput.Model
	err         error
}

// New creates a new TUI model.
func New(s scanner.Scanner, cfg config.Config, cfgPath string) Model {
	ti := textinput.New()
	ti.Placeholder = "label"
	ti.CharLimit = 30

	return Model{
		scanner:    s,
		config:     cfg,
		configPath: cfgPath,
		labelInput: ti,
	}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return tea.Batch(m.doScan(), doTick(m.config.RefreshInterval))
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case scanResultMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.err = nil
		m.servers = msg.servers
		m.lastRefresh = time.Now()
		m.applyFilter()
		return m, nil

	case tickMsg:
		return m, tea.Batch(m.doScan(), doTick(m.config.RefreshInterval))

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

// handleKey processes key events based on the current mode.
func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.mode {
	case modeNormal:
		return m.handleNormalKey(msg)
	default:
		return m, nil
	}
}

// handleNormalKey processes key events in normal mode.
func (m Model) handleNormalKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Quit):
		return m, tea.Quit
	case key.Matches(msg, keys.Down):
		if len(m.filtered) > 0 && m.cursor < len(m.filtered)-1 {
			m.cursor++
		}
		return m, nil
	case key.Matches(msg, keys.Up):
		if m.cursor > 0 {
			m.cursor--
		}
		return m, nil
	}
	return m, nil
}

// applyFilter copies servers to filtered, applying the current filter text.
func (m *Model) applyFilter() {
	m.filtered = make([]scanner.Server, len(m.servers))
	copy(m.filtered, m.servers)

	// Clamp cursor
	if m.cursor >= len(m.filtered) {
		m.cursor = max(0, len(m.filtered)-1)
	}
}

// View implements tea.Model.
func (m Model) View() string {
	return "" // Will be implemented in Task 4.1
}
