package tui

import "github.com/charmbracelet/bubbles/key"

// KeyMap defines all key bindings for the application.
type KeyMap struct {
	Up            key.Binding
	Down          key.Binding
	Left          key.Binding
	Right         key.Binding
	Top           key.Binding
	Bottom        key.Binding
	Yank          key.Binding
	Delete        key.Binding
	Cut           key.Binding
	PasteAfter    key.Binding
	PasteBefore   key.Binding
	AddBookmark   key.Binding
	AddFolder     key.Binding
	QuickAdd      key.Binding
	Edit          key.Binding
	EditTags      key.Binding
	Sort          key.Binding
	ToggleConfirm key.Binding
	Search        key.Binding
	Filter        key.Binding
	YankURL       key.Binding
	Pin           key.Binding
	Move          key.Binding
	Help          key.Binding
	Quit          key.Binding
}

// DefaultKeyMap returns the default vim-style key bindings.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up: key.NewBinding(
			key.WithKeys("k", "up"),
			key.WithHelp("k/up", "move up"),
		),
		Down: key.NewBinding(
			key.WithKeys("j", "down"),
			key.WithHelp("j/down", "move down"),
		),
		Left: key.NewBinding(
			key.WithKeys("h", "left"),
			key.WithHelp("h/left", "go back"),
		),
		Right: key.NewBinding(
			key.WithKeys("l", "right", "enter"),
			key.WithHelp("l/right", "enter folder"),
		),
		Top: key.NewBinding(
			key.WithKeys("g"),
			key.WithHelp("gg", "go to top"),
		),
		Bottom: key.NewBinding(
			key.WithKeys("G"),
			key.WithHelp("G", "go to bottom"),
		),
		Yank: key.NewBinding(
			key.WithKeys("y"),
			key.WithHelp("y", "yank"),
		),
		Delete: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "delete"),
		),
		Cut: key.NewBinding(
			key.WithKeys("x"),
			key.WithHelp("x", "cut"),
		),
		PasteAfter: key.NewBinding(
			key.WithKeys("p"),
			key.WithHelp("p", "paste after"),
		),
		PasteBefore: key.NewBinding(
			key.WithKeys("P"),
			key.WithHelp("P", "paste before"),
		),
		AddBookmark: key.NewBinding(
			key.WithKeys("a"),
			key.WithHelp("a", "add bookmark"),
		),
		AddFolder: key.NewBinding(
			key.WithKeys("A"),
			key.WithHelp("A", "add folder"),
		),
		QuickAdd: key.NewBinding(
			key.WithKeys("i"),
			key.WithHelp("i", "AI quick add"),
		),
		Edit: key.NewBinding(
			key.WithKeys("e"),
			key.WithHelp("e", "edit"),
		),
		EditTags: key.NewBinding(
			key.WithKeys("t"),
			key.WithHelp("t", "edit tags"),
		),
		Sort: key.NewBinding(
			key.WithKeys("o"),
			key.WithHelp("o", "cycle sort"),
		),
		ToggleConfirm: key.NewBinding(
			key.WithKeys("c"),
			key.WithHelp("c", "toggle confirm"),
		),
		Search: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "global search"),
		),
		Filter: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "filter"),
		),
		YankURL: key.NewBinding(
			key.WithKeys("Y"),
			key.WithHelp("Y", "yank URL"),
		),
		Pin: key.NewBinding(
			key.WithKeys("*"),
			key.WithHelp("*", "pin/unpin"),
		),
		Move: key.NewBinding(
			key.WithKeys("m"),
			key.WithHelp("m", "move to folder"),
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
