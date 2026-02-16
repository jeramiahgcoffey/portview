package tui

import (
	"time"

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
	return m, nil // Will be implemented in Task 3.4
}

// View implements tea.Model.
func (m Model) View() string {
	return "" // Will be implemented in Task 4.1
}
