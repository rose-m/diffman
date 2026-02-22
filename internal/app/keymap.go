package app

import "github.com/charmbracelet/bubbles/key"

// KeyMap defines global and pane-specific bindings.
type KeyMap struct {
	Quit        key.Binding
	ToggleFocus key.Binding
	Up          key.Binding
	Down        key.Binding
	Open        key.Binding
	Refresh     key.Binding
	Top         key.Binding
	Bottom      key.Binding
	Help        key.Binding
}

func defaultKeyMap() KeyMap {
	return KeyMap{
		Quit:        key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
		ToggleFocus: key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "switch focus")),
		Up:          key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("k/up", "move up")),
		Down:        key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("j/down", "move down")),
		Open:        key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open diff")),
		Refresh:     key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh files")),
		Top:         key.NewBinding(key.WithKeys("g"), key.WithHelp("g", "top")),
		Bottom:      key.NewBinding(key.WithKeys("G"), key.WithHelp("G", "bottom")),
		Help:        key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
	}
}
