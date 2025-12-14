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
	HintKey      lipgloss.Style // Key portion of hints (e.g., "Enter", "j/k")
	HintDesc     lipgloss.Style // Description portion of hints (e.g., "confirm", "move")
	Breadcrumb   lipgloss.Style // Folder path breadcrumb above Miller columns
}

// DefaultStyles returns the default style configuration.
// Industrial design: grayscale with single desaturated teal accent.
func DefaultStyles() Styles {
	// Industrial color palette
	primary := lipgloss.AdaptiveColor{Light: "#505050", Dark: "#A0A0A0"} // main text
	subtle := lipgloss.AdaptiveColor{Light: "#888888", Dark: "#606060"}  // secondary text
	accent := lipgloss.AdaptiveColor{Light: "#4A7070", Dark: "#5F8787"}  // desaturated teal
	border := lipgloss.AdaptiveColor{Light: "#888888", Dark: "#505050"}  // inactive borders

	return Styles{
		App: lipgloss.NewStyle().
			PaddingTop(1).
			PaddingLeft(2).
			PaddingRight(2),

		Pane: lipgloss.NewStyle().
			Border(lipgloss.ThickBorder()).
			BorderForeground(border).
			Padding(0, 1),

		PaneActive: lipgloss.NewStyle().
			Border(lipgloss.ThickBorder()).
			BorderForeground(accent).
			Padding(0, 1),

		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(accent),

		Item: lipgloss.NewStyle().
			Foreground(primary).
			PaddingLeft(1),

		ItemSelected: lipgloss.NewStyle().
			PaddingLeft(1).
			Background(accent).
			Foreground(lipgloss.Color("#1A1A1A")),

		Folder: lipgloss.NewStyle().
			Foreground(primary),

		Bookmark: lipgloss.NewStyle().
			Foreground(primary),

		URL: lipgloss.NewStyle().
			Foreground(subtle),

		Tag: lipgloss.NewStyle().
			Foreground(subtle),

		Date: lipgloss.NewStyle().
			Foreground(subtle),

		Help: lipgloss.NewStyle().
			Foreground(subtle).
			Padding(1, 0),

		Empty: lipgloss.NewStyle().
			Foreground(subtle),

		HintKey: lipgloss.NewStyle().
			Foreground(subtle),

		HintDesc: lipgloss.NewStyle().
			Foreground(subtle),

		Breadcrumb: lipgloss.NewStyle().
			Foreground(subtle).
			PaddingLeft(1),
	}
}
