package tui

import "github.com/charmbracelet/bubbles/key"

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
	Confirm key.Binding
	Cancel  key.Binding
}

var keys = keyMap{
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("\u2191/k", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("\u2193/j", "down"),
	),
	Open: key.NewBinding(
		key.WithKeys("o", "enter"),
		key.WithHelp("o/enter", "open in browser"),
	),
	Kill: key.NewBinding(
		key.WithKeys("x"),
		key.WithHelp("x", "kill process"),
	),
	Label: key.NewBinding(
		key.WithKeys("l"),
		key.WithHelp("l", "edit label"),
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
	Confirm: key.NewBinding(
		key.WithKeys("y"),
		key.WithHelp("y", "confirm"),
	),
	Cancel: key.NewBinding(
		key.WithKeys("n", "esc"),
		key.WithHelp("n/esc", "cancel"),
	),
}
