package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/nikbrunner/bm/internal/model"
)

// renderView creates the complete Miller columns view.
func (a App) renderView() string {
	// Check if we're in a modal mode
	if a.mode != ModeNormal {
		return a.renderModal()
	}

	// Calculate pane widths (3 columns)
	paneWidth := (a.width - 8) / 3 // account for borders and padding
	if paneWidth < 20 {
		paneWidth = 20
	}
	paneHeight := a.height - 6 // account for help bar and app padding
	if paneHeight < 5 {
		paneHeight = 5
	}

	// Build three panes
	leftPane := a.renderParentPane(paneWidth, paneHeight)
	middlePane := a.renderCurrentPane(paneWidth, paneHeight)
	rightPane := a.renderPreviewPane(paneWidth, paneHeight)

	// Join panes horizontally
	columns := lipgloss.JoinHorizontal(
		lipgloss.Top,
		leftPane,
		middlePane,
		rightPane,
	)

	// Add help bar at bottom
	helpBar := a.renderHelpBar()

	return a.styles.App.Render(
		lipgloss.JoinVertical(lipgloss.Left, columns, helpBar),
	)
}

// renderModal renders the current modal dialog.
func (a App) renderModal() string {
	var title, content strings.Builder

	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#7D56F4")).
		Padding(1, 2).
		Width(50)

	switch a.mode {
	case ModeAddFolder:
		title.WriteString("Add Folder\n\n")
		content.WriteString("Name:\n")
		content.WriteString(a.titleInput.View())
		content.WriteString("\n\n")
		content.WriteString(a.styles.Help.Render("Enter: save • Esc: cancel"))

	case ModeAddBookmark:
		title.WriteString("Add Bookmark\n\n")
		content.WriteString("Title:\n")
		content.WriteString(a.titleInput.View())
		content.WriteString("\n\n")
		content.WriteString("URL:\n")
		content.WriteString(a.urlInput.View())
		content.WriteString("\n\n")
		content.WriteString(a.styles.Help.Render("Tab: switch field • Enter: save • Esc: cancel"))

	case ModeEditFolder:
		title.WriteString("Edit Folder\n\n")
		content.WriteString("Name:\n")
		content.WriteString(a.titleInput.View())
		content.WriteString("\n\n")
		content.WriteString(a.styles.Help.Render("Enter: save • Esc: cancel"))

	case ModeEditBookmark:
		title.WriteString("Edit Bookmark\n\n")
		content.WriteString("Title:\n")
		content.WriteString(a.titleInput.View())
		content.WriteString("\n\n")
		content.WriteString("URL:\n")
		content.WriteString(a.urlInput.View())
		content.WriteString("\n\n")
		content.WriteString(a.styles.Help.Render("Tab: switch field • Enter: save • Esc: cancel"))

	case ModeEditTags:
		title.WriteString("Edit Tags\n\n")
		content.WriteString("Tags (comma-separated):\n")
		content.WriteString(a.tagsInput.View())
		content.WriteString("\n\n")
		content.WriteString(a.styles.Help.Render("Enter: save • Esc: cancel"))

	case ModeConfirmDelete:
		folder := a.store.GetFolderByID(a.editItemID)
		folderName := "this folder"
		if folder != nil {
			folderName = "\"" + folder.Name + "\""
		}
		title.WriteString("Delete Folder?\n\n")
		content.WriteString("Are you sure you want to delete " + folderName + "?\n\n")
		content.WriteString(a.styles.Help.Render("Enter: confirm • Esc: cancel"))

	case ModeSearch:
		// Render full-screen fuzzy finder
		return a.renderFuzzyFinder()
	}

	modalContent := a.styles.Title.Render(title.String()) + content.String()

	return lipgloss.Place(
		a.width,
		a.height,
		lipgloss.Center,
		lipgloss.Center,
		modalStyle.Render(modalContent),
	)
}

func (a App) renderParentPane(width, height int) string {
	var content strings.Builder

	if a.currentFolderID == nil {
		// At root - show app title
		content.WriteString(a.styles.Title.Render("bm"))
		content.WriteString("\n")
		content.WriteString(a.styles.Empty.Render("bookmarks"))
	} else {
		// Show parent folder contents
		currentFolder := a.store.GetFolderByID(*a.currentFolderID)
		if currentFolder != nil {
			// Show the parent folder's contents (siblings of current)
			parentFolderID := currentFolder.ParentID
			items := a.getItemsForFolder(parentFolderID)

			for _, item := range items {
				// Highlight current folder in parent view
				isCurrentFolder := item.IsFolder() && item.Folder.ID == *a.currentFolderID
				line := a.renderItem(item, isCurrentFolder, width-4)
				content.WriteString(line + "\n")
			}
		}
	}

	return a.styles.Pane.
		Width(width).
		Height(height).
		Render(strings.TrimRight(content.String(), "\n"))
}

func (a App) renderCurrentPane(width, height int) string {
	var content strings.Builder

	if len(a.items) == 0 {
		content.WriteString(a.styles.Empty.Render("(empty)"))
	} else {
		for i, item := range a.items {
			isSelected := i == a.cursor
			line := a.renderItem(item, isSelected, width-4)
			content.WriteString(line + "\n")
		}
	}

	return a.styles.PaneActive.
		Width(width).
		Height(height).
		Render(strings.TrimRight(content.String(), "\n"))
}

func (a App) renderPreviewPane(width, height int) string {
	var content strings.Builder

	if len(a.items) > 0 && a.cursor < len(a.items) {
		item := a.items[a.cursor]

		if item.IsFolder() {
			// Show folder contents preview
			folderID := item.Folder.ID
			children := a.getItemsForFolder(&folderID)

			if len(children) == 0 {
				content.WriteString(a.styles.Empty.Render("(empty folder)"))
			} else {
				for _, child := range children {
					content.WriteString(a.renderItem(child, false, width-4) + "\n")
				}
			}
		} else {
			// Show bookmark details
			b := item.Bookmark
			content.WriteString(a.styles.Title.Render(b.Title) + "\n\n")

			// URL (potentially wrapped)
			url := b.URL
			if len(url) > width-4 {
				url = url[:width-7] + "..."
			}
			content.WriteString(a.styles.URL.Render(url) + "\n\n")

			// Tags
			if len(b.Tags) > 0 {
				tags := make([]string, len(b.Tags))
				for i, tag := range b.Tags {
					tags[i] = "#" + tag
				}
				content.WriteString(a.styles.Tag.Render(strings.Join(tags, " ")) + "\n\n")
			}

			// Dates
			content.WriteString(a.styles.Date.Render(
				fmt.Sprintf("Created: %s", b.CreatedAt.Format("2006-01-02")),
			) + "\n")

			if b.VisitedAt != nil {
				content.WriteString(a.styles.Date.Render(
					fmt.Sprintf("Visited: %s", b.VisitedAt.Format("2006-01-02")),
				))
			}
		}
	}

	return a.styles.Pane.
		Width(width).
		Height(height).
		Render(strings.TrimRight(content.String(), "\n"))
}

func (a App) renderItem(item Item, selected bool, maxWidth int) string {
	var prefix, text string

	if item.IsFolder() {
		prefix = "📁 "
		text = item.Title()
	} else {
		prefix = "   "
		text = item.Title()
	}

	// Truncate if too long
	line := prefix + text
	if len(line) > maxWidth {
		line = line[:maxWidth-3] + "..."
	}

	if selected {
		// Pad to fill width for selection highlight
		for len(line) < maxWidth {
			line += " "
		}
		return a.styles.ItemSelected.Render(line)
	}
	return a.styles.Item.Render(line)
}

// renderFuzzyFinder renders the fuzzy finder overlay.
func (a App) renderFuzzyFinder() string {
	// Calculate dimensions
	width := a.width - 4
	if width < 60 {
		width = 60
	}
	height := a.height - 4
	if height < 10 {
		height = 10
	}

	// Input line at top
	inputLine := "> " + a.searchInput.View()

	// Split into results (left) and preview (right)
	resultsWidth := (width - 4) / 2
	previewWidth := width - resultsWidth - 4

	// Build results list
	var results strings.Builder
	maxResults := height - 6 // Leave room for input and help
	if maxResults < 3 {
		maxResults = 3
	}

	if len(a.fuzzyMatches) == 0 {
		results.WriteString(a.styles.Empty.Render("No matches"))
	} else {
		for i, match := range a.fuzzyMatches {
			if i >= maxResults {
				break
			}
			isSelected := i == a.fuzzyCursor
			line := a.renderFuzzyItem(match, isSelected, resultsWidth-2)
			results.WriteString(line + "\n")
		}
	}

	// Build preview pane
	var preview strings.Builder
	if len(a.fuzzyMatches) > 0 && a.fuzzyCursor < len(a.fuzzyMatches) {
		selectedItem := a.fuzzyMatches[a.fuzzyCursor].Item
		if selectedItem.IsFolder() {
			preview.WriteString(a.styles.Title.Render("📁 " + selectedItem.Folder.Name))
			preview.WriteString("\n\n")
			// Show folder contents count
			folderID := selectedItem.Folder.ID
			children := a.getItemsForFolder(&folderID)
			preview.WriteString(a.styles.Date.Render(fmt.Sprintf("%d items", len(children))))
		} else {
			b := selectedItem.Bookmark
			preview.WriteString(a.styles.Title.Render(b.Title))
			preview.WriteString("\n\n")
			// URL
			url := b.URL
			if len(url) > previewWidth-4 {
				url = url[:previewWidth-7] + "..."
			}
			preview.WriteString(a.styles.URL.Render(url))
			preview.WriteString("\n\n")
			// Tags
			if len(b.Tags) > 0 {
				tags := make([]string, len(b.Tags))
				for i, tag := range b.Tags {
					tags[i] = "#" + tag
				}
				preview.WriteString(a.styles.Tag.Render(strings.Join(tags, " ")))
				preview.WriteString("\n")
			}
		}
	}

	// Style the panes
	resultsPane := a.styles.PaneActive.
		Width(resultsWidth).
		Height(height - 5).
		Render(strings.TrimRight(results.String(), "\n"))

	previewPane := a.styles.Pane.
		Width(previewWidth).
		Height(height - 5).
		Render(strings.TrimRight(preview.String(), "\n"))

	// Join panes
	panes := lipgloss.JoinHorizontal(lipgloss.Top, resultsPane, previewPane)

	// Help text
	help := a.styles.Help.Render("↑/k ↓/j: navigate • Enter: select • Esc: cancel")

	// Build modal content
	modalContent := lipgloss.JoinVertical(lipgloss.Left,
		a.styles.Title.Render("Find"),
		inputLine,
		"",
		panes,
		help,
	)

	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#7D56F4")).
		Padding(1, 2).
		Width(width)

	return lipgloss.Place(
		a.width,
		a.height,
		lipgloss.Center,
		lipgloss.Center,
		modalStyle.Render(modalContent),
	)
}

// renderFuzzyItem renders a single item in the fuzzy results with highlighting.
func (a App) renderFuzzyItem(match fuzzyMatch, selected bool, maxWidth int) string {
	var prefix string
	if match.Item.IsFolder() {
		prefix = "📁 "
	} else {
		prefix = "   "
	}

	title := match.Item.Title()

	// Build highlighted string
	var line strings.Builder
	line.WriteString(prefix)

	// Apply highlighting to matched characters
	matchSet := make(map[int]bool)
	for _, idx := range match.MatchedIndexes {
		matchSet[idx] = true
	}

	for i, r := range title {
		if matchSet[i] {
			// Highlight matched character (bold/underline)
			line.WriteString("\033[1;4m")
			line.WriteRune(r)
			line.WriteString("\033[0m")
		} else {
			line.WriteRune(r)
		}
	}

	result := line.String()

	// Truncate if needed (rough estimate due to ANSI codes)
	if len(title)+len(prefix) > maxWidth {
		result = prefix + title[:maxWidth-len(prefix)-3] + "..."
	}

	if selected {
		return a.styles.ItemSelected.Render(result)
	}
	return a.styles.Item.Render(result)
}

func (a App) renderHelpBar() string {
	// Build status indicators
	var status strings.Builder

	// Sort mode indicator
	sortLabels := map[SortMode]string{
		SortManual:  "manual",
		SortAlpha:   "A-Z",
		SortCreated: "created",
		SortVisited: "visited",
	}
	status.WriteString("[sort: " + sortLabels[a.sortMode] + "]")

	help := "j/k: move  h/l: navigate  /: find  s: sort  a: add  e: edit  dd: cut  p: paste  q: quit"

	return a.styles.Help.Render(status.String() + "  " + help)
}

func (a App) getItemsForFolder(folderID *string) []Item {
	items := []Item{}

	folders := a.store.GetFoldersInFolder(folderID)
	for i := range folders {
		items = append(items, Item{Kind: ItemFolder, Folder: &folders[i]})
	}

	bookmarks := a.store.GetBookmarksInFolder(folderID)
	for i := range bookmarks {
		items = append(items, Item{Kind: ItemBookmark, Bookmark: &bookmarks[i]})
	}

	return items
}

// getBreadcrumb returns the folder path as a string.
func (a App) getBreadcrumb() string {
	if a.currentFolderID == nil {
		return "/"
	}

	parts := []string{}

	// Build from stack
	for _, id := range a.folderStack {
		folder := a.store.GetFolderByID(id)
		if folder != nil {
			parts = append(parts, folder.Name)
		}
	}

	// Add current folder
	currentFolder := a.store.GetFolderByID(*a.currentFolderID)
	if currentFolder != nil {
		parts = append(parts, currentFolder.Name)
	}

	return "/" + strings.Join(parts, "/")
}

// Store returns the underlying store (for access from view).
func (a App) Store() *model.Store {
	return a.store
}
