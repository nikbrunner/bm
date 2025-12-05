package picker

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/nikbrunner/bm/internal/model"
	"github.com/nikbrunner/bm/internal/search"
)

var (
	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("212")).
			Bold(true)

	normalStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	urlStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("244")).
			Italic(true)

	headerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("99")).
			Bold(true).
			MarginBottom(1)
)

// Picker is a simple TUI for selecting from search results.
type Picker struct {
	results   []search.SearchResult
	query     string
	cursor    int
	selected  bool
	cancelled bool
	width     int
	height    int
}

// New creates a new Picker with the given search results.
func New(results []search.SearchResult, query string) Picker {
	return Picker{
		results: results,
		query:   query,
		cursor:  0,
		width:   80,
		height:  24,
	}
}

// Init implements tea.Model.
func (p Picker) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (p Picker) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		p.width = msg.Width
		p.height = msg.Height
		return p, nil

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEsc, tea.KeyCtrlC:
			p.cancelled = true
			return p, tea.Quit

		case tea.KeyEnter:
			p.selected = true
			return p, tea.Quit

		case tea.KeyDown:
			if p.cursor < len(p.results)-1 {
				p.cursor++
			}
			return p, nil

		case tea.KeyUp:
			if p.cursor > 0 {
				p.cursor--
			}
			return p, nil
		}

		// Handle j/k vim keys
		if msg.Type == tea.KeyRunes {
			switch string(msg.Runes) {
			case "j":
				if p.cursor < len(p.results)-1 {
					p.cursor++
				}
				return p, nil
			case "k":
				if p.cursor > 0 {
					p.cursor--
				}
				return p, nil
			case "q":
				p.cancelled = true
				return p, tea.Quit
			}
		}
	}

	return p, nil
}

// View implements tea.Model.
func (p Picker) View() string {
	var b strings.Builder

	// Header
	b.WriteString(headerStyle.Render(fmt.Sprintf("Search: %s (%d results)", p.query, len(p.results))))
	b.WriteString("\n\n")

	// List items
	for i, result := range p.results {
		cursor := "  "
		style := normalStyle
		if i == p.cursor {
			cursor = "> "
			style = selectedStyle
		}

		title := style.Render(result.Bookmark.Title)
		url := urlStyle.Render(result.Bookmark.URL)

		b.WriteString(fmt.Sprintf("%s%s\n", cursor, title))
		b.WriteString(fmt.Sprintf("   %s\n", url))
	}

	// Footer
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render("j/k: move  Enter: open  q/Esc: cancel"))

	return b.String()
}

// SelectedBookmark returns the selected bookmark, or nil if cancelled.
func (p Picker) SelectedBookmark() *model.Bookmark {
	if p.cancelled || !p.selected {
		return nil
	}
	if p.cursor < len(p.results) {
		return p.results[p.cursor].Bookmark
	}
	return nil
}

// Cancelled returns true if the user cancelled the selection.
func (p Picker) Cancelled() bool {
	return p.cancelled
}
