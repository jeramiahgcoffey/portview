package tui

import "github.com/charmbracelet/bubbles/key"

// keyMap defines every keybinding in the TUI. It implements help.KeyMap so the
// help overlay (added in a later phase) can render automatically.
type keyMap struct {
	Up      key.Binding
	Down    key.Binding
	Open    key.Binding
	Kill    key.Binding
	Label   key.Binding
	Refresh key.Binding
	Filter  key.Binding
	Help    key.Binding
	Quit    key.Binding
}

// defaultKeys returns the keybindings from the design doc.
func defaultKeys() keyMap {
	return keyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		Open: key.NewBinding(
			key.WithKeys("o", "enter"),
			key.WithHelp("o", "open"),
		),
		Kill: key.NewBinding(
			key.WithKeys("x"),
			key.WithHelp("x", "kill"),
		),
		Label: key.NewBinding(
			key.WithKeys("l"),
			key.WithHelp("l", "label"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "refresh"),
		),
		Filter: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "filter"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
	}
}

// ShortHelp implements help.KeyMap.
func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Open, k.Kill, k.Label, k.Refresh, k.Filter, k.Help, k.Quit}
}

// FullHelp implements help.KeyMap.
func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down},
		{k.Open, k.Kill, k.Label},
		{k.Refresh, k.Filter},
		{k.Help, k.Quit},
	}
}
