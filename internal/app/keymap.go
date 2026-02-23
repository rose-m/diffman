package app

import "github.com/charmbracelet/bubbles/key"

// KeyMap defines global and pane-specific bindings.
type KeyMap struct {
	Quit         key.Binding
	ToggleFocus  key.Binding
	Up           key.Binding
	Down         key.Binding
	Open         key.Binding
	ToggleFiles  key.Binding
	Refresh      key.Binding
	Top          key.Binding
	Bottom       key.Binding
	PageDown     key.Binding
	PageUp       key.Binding
	ScrollDown   key.Binding
	ScrollUp     key.Binding
	Help         key.Binding
	ToggleMode   key.Binding
	Create       key.Binding
	Edit         key.Binding
	Delete       key.Binding
	NextComment  key.Binding
	PrevComment  key.Binding
	Export       key.Binding
	ClearAll     key.Binding
	CommentsView key.Binding
}

func defaultKeyMap() KeyMap {
	return KeyMap{
		Quit:         key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
		ToggleFocus:  key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "switch focus")),
		Up:           key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("k/up", "move up")),
		Down:         key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("j/down", "move down")),
		Open:         key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open diff")),
		ToggleFiles:  key.NewBinding(key.WithKeys("z"), key.WithHelp("z", "toggle file pane width")),
		Refresh:      key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh files")),
		Top:          key.NewBinding(key.WithKeys("g"), key.WithHelp("g", "top")),
		Bottom:       key.NewBinding(key.WithKeys("G"), key.WithHelp("G", "bottom")),
		PageDown:     key.NewBinding(key.WithKeys("ctrl+f"), key.WithHelp("ctrl+f", "page down")),
		PageUp:       key.NewBinding(key.WithKeys("ctrl+b"), key.WithHelp("ctrl+b", "page up")),
		ScrollDown:   key.NewBinding(key.WithKeys("ctrl+e"), key.WithHelp("ctrl+e", "scroll down")),
		ScrollUp:     key.NewBinding(key.WithKeys("ctrl+y"), key.WithHelp("ctrl+y", "scroll up")),
		Help:         key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		ToggleMode:   key.NewBinding(key.WithKeys("t"), key.WithHelp("t", "toggle diff mode")),
		Create:       key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "new comment")),
		Edit:         key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "edit comment")),
		Delete:       key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "delete comment")),
		NextComment:  key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "next comment")),
		PrevComment:  key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "prev comment")),
		Export:       key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "copy export")),
		ClearAll:     key.NewBinding(key.WithKeys("C"), key.WithHelp("C", "clear all comments")),
		CommentsView: key.NewBinding(key.WithKeys("m"), key.WithHelp("m", "comments view")),
	}
}
