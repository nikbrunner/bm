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

	paneHeight := a.height - 5 // account for help bar (3 lines), app top padding (1), and borders
	if paneHeight < 5 {
		paneHeight = 5
	}

	var columns string

	// Determine layout based on pinned items and current location
	hasPinnedItems := len(a.pinnedItems) > 0
	atRoot := a.currentFolderID == nil

	if hasPinnedItems && atRoot {
		// At root with pinned items: 3 panes (pinned replaces parent since both would show "bm/bookmarks")
		paneWidth := (a.width - 8) / 3
		if paneWidth < 20 {
			paneWidth = 20
		}

		pinnedPane := a.renderPinnedPane(paneWidth, paneHeight)
		middlePane := a.renderCurrentPane(paneWidth, paneHeight)
		rightPane := a.renderPreviewPane(paneWidth, paneHeight)

		columns = lipgloss.JoinHorizontal(
			lipgloss.Top,
			pinnedPane,
			middlePane,
			rightPane,
		)
	} else if hasPinnedItems && !atRoot {
		// In subfolder with pinned items: 4 panes (pinned | parent | current | preview)
		paneWidth := (a.width - 10) / 4
		if paneWidth < 15 {
			paneWidth = 15
		}

		pinnedPane := a.renderPinnedPane(paneWidth, paneHeight)
		leftPane := a.renderParentPane(paneWidth, paneHeight)
		middlePane := a.renderCurrentPane(paneWidth, paneHeight)
		rightPane := a.renderPreviewPane(paneWidth, paneHeight)

		columns = lipgloss.JoinHorizontal(
			lipgloss.Top,
			pinnedPane,
			leftPane,
			middlePane,
			rightPane,
		)
	} else {
		// No pinned items: 3 panes (parent | current | preview)
		paneWidth := (a.width - 8) / 3
		if paneWidth < 20 {
			paneWidth = 20
		}

		leftPane := a.renderParentPane(paneWidth, paneHeight)
		middlePane := a.renderCurrentPane(paneWidth, paneHeight)
		rightPane := a.renderPreviewPane(paneWidth, paneHeight)

		columns = lipgloss.JoinHorizontal(
			lipgloss.Top,
			leftPane,
			middlePane,
			rightPane,
		)
	}

	// Add help bar at bottom
	helpBar := a.renderHelpBar()

	return a.styles.App.Render(
		lipgloss.JoinVertical(lipgloss.Left, columns, helpBar),
	)
}

// renderPinnedPane renders the leftmost pane with pinned items.
func (a App) renderPinnedPane(width, height int) string {
	var content strings.Builder

	// Header
	content.WriteString(a.styles.Title.Render("bm") + "\n")
	content.WriteString(a.styles.Empty.Render("bookmarks") + "\n\n")

	// Pinned section header
	content.WriteString("── Pinned ──\n")

	visibleHeight := height - 4 // account for header and divider
	if visibleHeight < 1 {
		visibleHeight = 1
	}

	if len(a.pinnedItems) == 0 {
		content.WriteString(a.styles.Empty.Render("(no pinned items)"))
	} else {
		// Calculate viewport offset to keep cursor visible
		offset := calculateViewportOffset(a.pinnedCursor, len(a.pinnedItems), visibleHeight)

		for i, item := range a.pinnedItems {
			// Skip items before viewport
			if i < offset {
				continue
			}
			// Stop after viewport is filled
			if i >= offset+visibleHeight {
				break
			}
			// Highlight selected item only when pinned pane is focused
			isSelected := a.focusedPane == PanePinned && i == a.pinnedCursor
			line := a.renderPinnedItem(item, isSelected, width-4)
			content.WriteString(line + "\n")
		}
	}

	// Use active style when focused, regular style otherwise
	if a.focusedPane == PanePinned {
		return a.styles.PaneActive.
			Width(width).
			Height(height).
			Render(strings.TrimRight(content.String(), "\n"))
	}
	return a.styles.Pane.
		Width(width).
		Height(height).
		Render(strings.TrimRight(content.String(), "\n"))
}

// renderPinnedItem renders an item in the pinned pane.
func (a App) renderPinnedItem(item Item, selected bool, maxWidth int) string {
	var prefix, text string

	if item.IsFolder() {
		prefix = "★ 📁 "
		text = item.Title()
	} else {
		prefix = "★ "
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
		// Determine action and item type
		action := "Delete"
		if a.cutMode {
			action = "Cut"
		}

		// Try folder first, then bookmark
		var itemType, itemName string
		if folder := a.store.GetFolderByID(a.editItemID); folder != nil {
			itemType = "Folder"
			itemName = folder.Name
		} else if bookmark := a.store.GetBookmarkByID(a.editItemID); bookmark != nil {
			itemType = "Bookmark"
			itemName = bookmark.Title
		} else {
			itemType = "Item"
			itemName = "this item"
		}

		title.WriteString(action + " " + itemType + "?\n\n")
		content.WriteString("Are you sure you want to " + strings.ToLower(action) + " \"" + itemName + "\"?\n\n")
		content.WriteString(a.styles.Help.Render("Enter: confirm • Esc: cancel"))

	case ModeSearch:
		// Render full-screen fuzzy finder
		return a.renderFuzzyFinder()

	case ModeHelp:
		// Render help overlay
		return a.renderHelpOverlay()

	case ModeQuickAdd:
		title.WriteString("AI Quick Add\n\n")
		content.WriteString("URL:\n")
		content.WriteString(a.quickAddInput.View())
		content.WriteString("\n\n")
		content.WriteString(a.styles.Help.Render("Enter: analyze • Esc: cancel"))

	case ModeQuickAddLoading:
		title.WriteString("AI Quick Add\n\n")
		content.WriteString("Analyzing link...\n\n")
		content.WriteString(a.styles.Empty.Render("Please wait while AI suggests title, folder, and tags"))
		content.WriteString("\n\n")
		content.WriteString(a.styles.Help.Render("Esc: cancel"))

	case ModeQuickAddConfirm:
		return a.renderQuickAddConfirm()
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

	visibleHeight := height
	if visibleHeight < 1 {
		visibleHeight = 1
	}

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

			// Find index of current folder for viewport calculation
			currentIdx := 0
			for i, item := range items {
				if item.IsFolder() && item.Folder.ID == *a.currentFolderID {
					currentIdx = i
					break
				}
			}

			// Calculate viewport offset to keep current folder visible
			offset := calculateViewportOffset(currentIdx, len(items), visibleHeight)

			for i, item := range items {
				// Skip items before viewport
				if i < offset {
					continue
				}
				// Stop after viewport is filled
				if i >= offset+visibleHeight {
					break
				}
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

	visibleHeight := height
	if visibleHeight < 1 {
		visibleHeight = 1
	}

	if len(a.items) == 0 {
		content.WriteString(a.styles.Empty.Render("(empty)"))
	} else {
		// Calculate viewport offset to keep cursor visible
		offset := calculateViewportOffset(a.cursor, len(a.items), visibleHeight)

		for i, item := range a.items {
			// Skip items before viewport
			if i < offset {
				continue
			}
			// Stop after viewport is filled
			if i >= offset+visibleHeight {
				break
			}
			// Only show selection when browser pane is focused
			isSelected := a.focusedPane == PaneBrowser && i == a.cursor
			line := a.renderItem(item, isSelected, width-4)
			content.WriteString(line + "\n")
		}
	}

	// Use active style when browser pane is focused
	if a.focusedPane == PaneBrowser {
		return a.styles.PaneActive.
			Width(width).
			Height(height).
			Render(strings.TrimRight(content.String(), "\n"))
	}
	return a.styles.Pane.
		Width(width).
		Height(height).
		Render(strings.TrimRight(content.String(), "\n"))
}

func (a App) renderPreviewPane(width, height int) string {
	var content strings.Builder

	visibleHeight := height
	if visibleHeight < 1 {
		visibleHeight = 1
	}

	if len(a.items) > 0 && a.cursor < len(a.items) {
		item := a.items[a.cursor]

		if item.IsFolder() {
			// Show folder contents preview
			folderID := item.Folder.ID
			children := a.getItemsForFolder(&folderID)

			if len(children) == 0 {
				content.WriteString(a.styles.Empty.Render("(empty folder)"))
			} else {
				// Limit to visible height
				for i, child := range children {
					if i >= visibleHeight {
						break
					}
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
	var isPinned bool

	if item.IsFolder() {
		isPinned = item.Folder.Pinned
		if isPinned {
			prefix = "★ 📁 "
		} else {
			prefix = "📁 "
		}
		text = item.Title()
	} else {
		isPinned = item.Bookmark.Pinned
		if isPinned {
			prefix = "★ "
		} else {
			prefix = "   "
		}
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
	// Overlay size: 65% width, 35% height
	modalWidth := a.width * 65 / 100
	modalHeight := a.height * 35 / 100

	// Minimum sizes
	if modalWidth < 60 {
		modalWidth = 60
	}
	if modalHeight < 10 {
		modalHeight = 10
	}

	// Split width: 25% list, 75% preview
	contentWidth := modalWidth - 8
	listWidth := contentWidth * 25 / 100
	previewWidth := contentWidth * 75 / 100
	listHeight := modalHeight - 6 // Account for header, input, help

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
			line := a.renderFuzzyItem(match, isSelected, listWidth-2)
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
			preview.WriteString(a.styles.Empty.Render(fmt.Sprintf("%d items", len(children))))
		} else {
			b := selectedItem.Bookmark
			preview.WriteString(a.styles.Title.Render(b.Title))
			preview.WriteString("\n\n")
			// URL (truncate if needed)
			url := b.URL
			if len(url) > previewWidth-2 {
				url = url[:previewWidth-5] + "..."
			}
			preview.WriteString(a.styles.URL.Render(url))
			if len(b.Tags) > 0 {
				preview.WriteString("\n\n")
				tags := make([]string, len(b.Tags))
				for i, tag := range b.Tags {
					tags[i] = "#" + tag
				}
				preview.WriteString(a.styles.Tag.Render(strings.Join(tags, " ")))
			}
		}
	}

	// Style for results list
	resultsStyle := lipgloss.NewStyle().
		Width(listWidth).
		Height(listHeight)

	// Style for preview
	previewStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("#444")).
		Width(previewWidth).
		Height(listHeight).
		Padding(0, 1)

	resultsPane := resultsStyle.Render(strings.TrimRight(results.String(), "\n"))
	previewPane := previewStyle.Render(strings.TrimRight(preview.String(), "\n"))

	// Join panes horizontally
	panes := lipgloss.JoinHorizontal(lipgloss.Top, resultsPane, " ", previewPane)

	// Build the modal
	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#7D56F4")).
		Padding(1, 2)

	// Result count
	countStr := fmt.Sprintf("%d results", len(a.fuzzyMatches))
	if len(a.fuzzyMatches) == 1 {
		countStr = "1 result"
	}

	content := lipgloss.JoinVertical(lipgloss.Left,
		a.styles.Title.Render("Find")+"  "+a.styles.Empty.Render(countStr),
		"",
		"> "+a.searchInput.Value()+"█",
		"",
		panes,
		"",
		a.styles.Help.Render("j/k: move • Enter: select • Esc: cancel"),
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
	// Build status line
	var status strings.Builder

	// Sort mode indicator
	sortLabels := map[SortMode]string{
		SortManual:  "manual",
		SortAlpha:   "A-Z",
		SortCreated: "created",
		SortVisited: "visited",
	}
	status.WriteString("[sort: " + sortLabels[a.sortMode] + "]")

	// Confirm mode indicator
	if a.confirmDelete {
		status.WriteString(" [confirm: on]")
	} else {
		warnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("208")).Bold(true)
		status.WriteString(" " + warnStyle.Render("[confirm: off]"))
	}

	// Status message (if any)
	if a.statusMessage != "" {
		statusStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true)
		status.WriteString("  " + statusStyle.Render(a.statusMessage))
	}

	// Hints line
	hints := "j/k: move  h/l: nav  o: open  m: pin  /: find  a: add  i: AI add  e: edit  d: del  ?: help  q: quit"

	// Style without padding for individual lines
	lineStyle := lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#666666", Dark: "#888888"})

	// Two lines: status on top, hints below
	content := lineStyle.Render(status.String()) + "\n" + lineStyle.Render(hints)

	// Only pad top, not bottom
	wrapperStyle := lipgloss.NewStyle().PaddingTop(1)
	return wrapperStyle.Render(content)
}

// renderQuickAddConfirm renders the AI quick add confirmation modal.
func (a App) renderQuickAddConfirm() string {
	modalWidth := 60
	if a.width < 70 {
		modalWidth = a.width - 10
	}

	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#7D56F4")).
		Padding(1, 2).
		Width(modalWidth)

	var content strings.Builder
	content.WriteString(a.styles.Title.Render("AI Quick Add - Confirm"))
	content.WriteString("\n\n")

	// Title input
	titleLabel := "Title:"
	if a.titleInput.Focused() {
		titleLabel = a.styles.Tag.Render("Title:")
	}
	content.WriteString(titleLabel + "\n")
	content.WriteString(a.titleInput.View())
	content.WriteString("\n\n")

	// Folder picker
	folderLabel := "Folder:"
	if !a.titleInput.Focused() && !a.tagsInput.Focused() {
		folderLabel = a.styles.Tag.Render("Folder:")
	}
	content.WriteString(folderLabel + "\n")

	// Show folder options with selection
	visibleFolders := 5
	start := 0
	if a.quickAddFolderIdx >= visibleFolders {
		start = a.quickAddFolderIdx - visibleFolders + 1
	}
	end := start + visibleFolders
	if end > len(a.quickAddFolders) {
		end = len(a.quickAddFolders)
	}

	for i := start; i < end; i++ {
		folder := a.quickAddFolders[i]
		if i == a.quickAddFolderIdx {
			content.WriteString(a.styles.ItemSelected.Render("> " + folder))
		} else {
			content.WriteString("  " + folder)
		}
		content.WriteString("\n")
	}
	content.WriteString("\n")

	// Tags input
	tagsLabel := "Tags:"
	if a.tagsInput.Focused() {
		tagsLabel = a.styles.Tag.Render("Tags:")
	}
	content.WriteString(tagsLabel + "\n")
	content.WriteString(a.tagsInput.View())
	content.WriteString("\n\n")

	content.WriteString(a.styles.Help.Render("Tab: next field • j/k: folder • Enter: save • Esc: cancel"))

	return lipgloss.Place(
		a.width,
		a.height,
		lipgloss.Center,
		lipgloss.Center,
		modalStyle.Render(content.String()),
	)
}

// renderHelpOverlay renders the help overlay.
func (a App) renderHelpOverlay() string {
	modalWidth := 60
	if a.width < 70 {
		modalWidth = a.width - 10
	}

	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#7D56F4")).
		Padding(1, 2).
		Width(modalWidth)

	// Build help content
	var content strings.Builder
	content.WriteString(a.styles.Title.Render("Keyboard Shortcuts"))
	content.WriteString("\n\n")

	// Navigation
	content.WriteString(a.styles.Tag.Render("Navigation"))
	content.WriteString("\n")
	content.WriteString("  j/k       Move down/up\n")
	content.WriteString("  h/l       Navigate back/forward (l opens bookmarks)\n")
	content.WriteString("  gg        Jump to top\n")
	content.WriteString("  G         Jump to bottom\n")
	content.WriteString("\n")

	// Actions
	content.WriteString(a.styles.Tag.Render("Actions"))
	content.WriteString("\n")
	content.WriteString("  o/Enter   Open URL in browser\n")
	content.WriteString("  Y         Yank URL to clipboard\n")
	content.WriteString("  m         Pin/unpin item\n")
	content.WriteString("  /         Global search\n")
	content.WriteString("  s         Cycle sort mode\n")
	content.WriteString("  c         Toggle delete confirmations\n")
	content.WriteString("\n")

	// CRUD
	content.WriteString(a.styles.Tag.Render("Edit"))
	content.WriteString("\n")
	content.WriteString("  a         Add bookmark\n")
	content.WriteString("  A         Add folder\n")
	content.WriteString("  i         AI quick add\n")
	content.WriteString("  e         Edit selected\n")
	content.WriteString("  t         Edit tags\n")
	content.WriteString("  y         Yank (copy)\n")
	content.WriteString("  d         Delete\n")
	content.WriteString("  x         Cut (delete + buffer)\n")
	content.WriteString("  p/P       Paste after/before\n")
	content.WriteString("\n")

	// Close
	content.WriteString(a.styles.Help.Render("Press ? or q or Esc to close"))

	return lipgloss.Place(
		a.width,
		a.height,
		lipgloss.Center,
		lipgloss.Center,
		modalStyle.Render(content.String()),
	)
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

// calculateViewportOffset calculates the scroll offset needed to keep the
// selected item visible within the viewport.
func calculateViewportOffset(selected, total, viewportHeight int) int {
	if total <= viewportHeight {
		return 0
	}

	// Keep selection roughly centered, but clamp to valid range
	offset := selected - viewportHeight/2
	if offset < 0 {
		offset = 0
	}

	maxOffset := total - viewportHeight
	if offset > maxOffset {
		offset = maxOffset
	}

	return offset
}
