package tui

import (
	"fmt"
	"strings"
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
		m.mergeLabels()
		m.filterHidden()
		m.lastRefresh = time.Now()
		m.applyFilter()
		return m, nil

	case tickMsg:
		return m, tea.Batch(m.doScan(), doTick(m.config.RefreshInterval))

	case killResultMsg:
		// After a kill, trigger a rescan
		return m, m.doScan()

	case labelSavedMsg:
		if msg.err != nil {
			m.err = msg.err
		}
		return m, nil

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
	case modeFilter:
		return m.handleFilterKey(msg)
	case modeConfirmKill:
		return m.handleConfirmKillKey(msg)
	case modeLabel:
		return m.handleLabelKey(msg)
	case modeHelp:
		return m.handleHelpKey(msg)
	}
	return m, nil
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
	case key.Matches(msg, keys.Filter):
		m.mode = modeFilter
		return m, nil
	case key.Matches(msg, keys.Kill):
		if len(m.filtered) == 0 {
			return m, nil
		}
		m.mode = modeConfirmKill
		return m, nil
	case key.Matches(msg, keys.Label):
		if len(m.filtered) == 0 {
			return m, nil
		}
		m.mode = modeLabel
		// Pre-populate with existing label
		s := m.filtered[m.cursor]
		existingLabel := m.config.Labels[s.Port]
		m.labelInput.SetValue(existingLabel)
		m.labelInput.Focus()
		return m, nil
	case key.Matches(msg, keys.Help):
		m.mode = modeHelp
		return m, nil
	case key.Matches(msg, keys.Open):
		if len(m.filtered) == 0 {
			return m, nil
		}
		s := m.filtered[m.cursor]
		return m, doOpen(s.Port)
	case key.Matches(msg, keys.Refresh):
		return m, m.doScan()
	}
	return m, nil
}

// handleFilterKey processes key events in filter mode.
func (m Model) handleFilterKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc, tea.KeyEnter:
		m.mode = modeNormal
		return m, nil
	case tea.KeyBackspace:
		if len(m.filterText) > 0 {
			m.filterText = m.filterText[:len(m.filterText)-1]
			m.applyFilter()
		}
		return m, nil
	case tea.KeyRunes:
		m.filterText += string(msg.Runes)
		m.applyFilter()
		return m, nil
	}
	return m, nil
}

// handleConfirmKillKey processes key events in kill confirmation mode.
func (m Model) handleConfirmKillKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Confirm):
		if len(m.filtered) > 0 {
			s := m.filtered[m.cursor]
			m.mode = modeNormal
			return m, doKill(s.PID)
		}
		m.mode = modeNormal
		return m, nil
	case key.Matches(msg, keys.Cancel):
		m.mode = modeNormal
		return m, nil
	}
	return m, nil
}

// handleLabelKey processes key events in label editing mode.
func (m Model) handleLabelKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.mode = modeNormal
		m.labelInput.Blur()
		return m, nil
	case tea.KeyEnter:
		if len(m.filtered) > 0 {
			s := m.filtered[m.cursor]
			label := m.labelInput.Value()
			if label == "" {
				m.config.RemoveLabel(s.Port)
			} else {
				m.config.SetLabel(s.Port, label)
			}
			// Update the server's label in memory
			m.filtered[m.cursor].Label = label
			for i, srv := range m.servers {
				if srv.Port == s.Port {
					m.servers[i].Label = label
				}
			}
			m.mode = modeNormal
			m.labelInput.Blur()
			return m, doSaveConfig(m.configPath, m.config)
		}
		m.mode = modeNormal
		m.labelInput.Blur()
		return m, nil
	}
	// Delegate to the textinput component
	var cmd tea.Cmd
	m.labelInput, cmd = m.labelInput.Update(msg)
	return m, cmd
}

// handleHelpKey processes key events in help mode — any key exits.
func (m Model) handleHelpKey(_ tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.mode = modeNormal
	return m, nil
}

// applyFilter copies servers to filtered, applying the current filter text.
func (m *Model) applyFilter() {
	if m.filterText == "" {
		m.filtered = make([]scanner.Server, len(m.servers))
		copy(m.filtered, m.servers)
	} else {
		lower := strings.ToLower(m.filterText)
		m.filtered = nil
		for _, s := range m.servers {
			portStr := fmt.Sprintf("%d", s.Port)
			if strings.Contains(strings.ToLower(s.Process), lower) ||
				strings.Contains(strings.ToLower(s.Label), lower) ||
				strings.Contains(portStr, m.filterText) {
				m.filtered = append(m.filtered, s)
			}
		}
	}

	// Clamp cursor
	if len(m.filtered) == 0 {
		m.cursor = 0
	} else if m.cursor >= len(m.filtered) {
		m.cursor = len(m.filtered) - 1
	}
}

// mergeLabels applies config labels to servers.
func (m *Model) mergeLabels() {
	for i, s := range m.servers {
		if label, ok := m.config.Labels[s.Port]; ok {
			m.servers[i].Label = label
		}
	}
}

// filterHidden removes servers on hidden ports.
func (m *Model) filterHidden() {
	filtered := m.servers[:0:0]
	for _, s := range m.servers {
		if !m.config.IsHidden(s.Port) {
			filtered = append(filtered, s)
		}
	}
	m.servers = filtered
}
