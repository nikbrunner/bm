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
	// Use most of the screen
	modalWidth := a.width - 10
	if modalWidth < 60 {
		modalWidth = 60
	}
	if modalWidth > 120 {
		modalWidth = 120
	}

	modalHeight := a.height - 6
	if modalHeight < 15 {
		modalHeight = 15
	}

	// Content width (accounting for modal padding and border)
	contentWidth := modalWidth - 6

	// Split content between results and preview
	resultsWidth := contentWidth * 2 / 3
	previewWidth := contentWidth - resultsWidth - 2

	// Calculate how many results we can show
	listHeight := modalHeight - 8 // Account for title, input, help, padding
	if listHeight < 5 {
		listHeight = 5
	}

	// Build results list
	var results strings.Builder
	if len(a.fuzzyMatches) == 0 {
		results.WriteString(a.styles.Empty.Render("No matches"))
	} else {
		for i, match := range a.fuzzyMatches {
			if i >= listHeight {
				break
			}
			isSelected := i == a.fuzzyCursor
			line := a.renderFuzzyItem(match, isSelected, resultsWidth-4)
			results.WriteString(line + "\n")
		}
	}

	// Build preview content
	var preview strings.Builder
	if len(a.fuzzyMatches) > 0 && a.fuzzyCursor < len(a.fuzzyMatches) {
		selectedItem := a.fuzzyMatches[a.fuzzyCursor].Item
		if selectedItem.IsFolder() {
			preview.WriteString("📁 " + selectedItem.Folder.Name)
			preview.WriteString("\n\n")
			folderID := selectedItem.Folder.ID
			children := a.getItemsForFolder(&folderID)
			preview.WriteString(fmt.Sprintf("%d items", len(children)))
		} else {
			b := selectedItem.Bookmark
			preview.WriteString(b.Title)
			preview.WriteString("\n\n")
			// URL (truncate if needed)
			url := b.URL
			if len(url) > previewWidth-2 {
				url = url[:previewWidth-5] + "..."
			}
			preview.WriteString(url)
			if len(b.Tags) > 0 {
				preview.WriteString("\n\n")
				tags := make([]string, len(b.Tags))
				for i, tag := range b.Tags {
					tags[i] = "#" + tag
				}
				preview.WriteString(strings.Join(tags, " "))
			}
		}
	}

	// Simple box styles for the panes
	resultsStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#7D56F4")).
		Width(resultsWidth).
		Height(listHeight)

	previewStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("#666")).
		Width(previewWidth).
		Height(listHeight).
		Padding(0, 1)

	resultsPane := resultsStyle.Render(strings.TrimRight(results.String(), "\n"))
	previewPane := previewStyle.Render(strings.TrimRight(preview.String(), "\n"))

	// Join panes horizontally
	panes := lipgloss.JoinHorizontal(lipgloss.Top, resultsPane, " ", previewPane)

	// Build the modal
	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(lipgloss.Color("#7D56F4")).
		Padding(1, 2)

	content := lipgloss.JoinVertical(lipgloss.Left,
		a.styles.Title.Render("Find"),
		"",
		"> "+a.searchInput.View(),
		"",
		panes,
		"",
		a.styles.Help.Render("↑/k ↓/j: navigate • Enter: select • Esc: cancel"),
	)

	return lipgloss.Place(
		a.width,
		a.height,
		lipgloss.Center,
		lipgloss.Center,
		modalStyle.Render(content),
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
