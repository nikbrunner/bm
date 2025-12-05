package tui

import "github.com/charmbracelet/lipgloss"

// Styles holds all lipgloss styles for the TUI.
type Styles struct {
	App          lipgloss.Style
	Pane         lipgloss.Style
	PaneActive   lipgloss.Style
	Title        lipgloss.Style
	Item         lipgloss.Style
	ItemSelected lipgloss.Style
	Folder       lipgloss.Style
	Bookmark     lipgloss.Style
	URL          lipgloss.Style
	Tag          lipgloss.Style
	Date         lipgloss.Style
	Help         lipgloss.Style
	Empty        lipgloss.Style
}

// DefaultStyles returns the default style configuration.
func DefaultStyles() Styles {
	subtle := lipgloss.AdaptiveColor{Light: "#666666", Dark: "#888888"}
	highlight := lipgloss.AdaptiveColor{Light: "#874BFD", Dark: "#7D56F4"}
	folder := lipgloss.AdaptiveColor{Light: "#1E90FF", Dark: "#87CEEB"}
	tag := lipgloss.AdaptiveColor{Light: "#228B22", Dark: "#90EE90"}

	return Styles{
		App: lipgloss.NewStyle().
			Padding(1, 2),

		Pane: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(subtle).
			Padding(0, 1),

		PaneActive: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(highlight).
			Padding(0, 1),

		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(highlight),

		Item: lipgloss.NewStyle().
			PaddingLeft(1),

		ItemSelected: lipgloss.NewStyle().
			PaddingLeft(1).
			Background(highlight).
			Foreground(lipgloss.Color("#FFFFFF")),

		Folder: lipgloss.NewStyle().
			Foreground(folder),

		Bookmark: lipgloss.NewStyle(),

		URL: lipgloss.NewStyle().
			Foreground(subtle).
			Italic(true),

		Tag: lipgloss.NewStyle().
			Foreground(tag),

		Date: lipgloss.NewStyle().
			Foreground(subtle),

		Help: lipgloss.NewStyle().
			Foreground(subtle).
			Padding(1, 0),

		Empty: lipgloss.NewStyle().
			Foreground(subtle).
			Italic(true),
	}
}
