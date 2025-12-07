package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/nikbrunner/bm/internal/model"
	"github.com/nikbrunner/bm/internal/tui/layout"
)

// renderView creates the complete Miller columns view.
func (a App) renderView() string {
	// Check if we're in a modal mode (ModeFilter stays inline, not a modal)
	if a.mode != ModeNormal && a.mode != ModeFilter {
		return a.renderModal()
	}

	// Calculate pane dimensions using layout config
	paneHeight := layout.CalculatePaneHeight(a.height, a.layoutConfig.Pane)
	hasPinnedItems := len(a.pinnedItems) > 0
	atRoot := a.currentFolderID == nil
	paneLayout := layout.CalculatePaneWidth(a.width, hasPinnedItems, atRoot, a.layoutConfig.Pane)
	paneWidth := paneLayout.Width

	var columns string

	if hasPinnedItems && atRoot {
		// At root with pinned items: 3 panes (pinned replaces parent since both would show "bm/bookmarks")
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

	content := a.styles.App.Render(
		lipgloss.JoinVertical(lipgloss.Left, columns, helpBar),
	)

	// Use Place to ensure exact terminal dimensions and prevent overflow
	return lipgloss.Place(a.width, a.height, lipgloss.Left, lipgloss.Top, content)
}

// renderPinnedPane renders the leftmost pane with pinned items.
func (a App) renderPinnedPane(width, height int) string {
	var content strings.Builder

	// Header
	content.WriteString(a.styles.Title.Render("bm") + "\n")
	content.WriteString(a.styles.Empty.Render("bookmarks") + "\n\n")

	// Pinned section header
	content.WriteString("── Pinned ──\n")

	visibleHeight := layout.CalculateVisibleHeight(height, a.layoutConfig.Pane.PinnedHeaderReduction)
	itemWidth := layout.CalculateItemWidth(width, a.layoutConfig.Pane)

	if len(a.pinnedItems) == 0 {
		content.WriteString(a.styles.Empty.Render("(no pinned items)"))
	} else {
		// Calculate viewport offset to keep cursor visible
		offset := layout.CalculateViewportOffset(a.pinnedCursor, len(a.pinnedItems), visibleHeight)

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
			line := a.renderPinnedItem(item, isSelected, itemWidth)
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
	var text, suffix string

	// All pinned items get * prefix
	prefix := "* "

	if item.IsFolder() {
		text = item.Title()
		suffix = "/"
	} else {
		text = item.Title()
	}

	// Truncate if too long using layout function
	line, _ := layout.TruncateWithPrefixSuffix(text, maxWidth, prefix, suffix, a.layoutConfig.Text)

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

	// Industrial style: thick borders, teal accent
	accent := lipgloss.AdaptiveColor{Light: "#4A7070", Dark: "#5F8787"}
	modalWidth := layout.CalculateModalWidth(a.width, a.layoutConfig.Modal.DefaultWidth, a.layoutConfig.Modal)
	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.ThickBorder()).
		BorderForeground(accent).
		Padding(1, 2).
		Width(modalWidth)

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
		content.WriteString("\n")

		// Render tag suggestions if any
		if len(a.tagSuggestions) > 0 {
			content.WriteString("\n")
			for i, tag := range a.tagSuggestions {
				if i == a.tagSuggestionIdx {
					content.WriteString(a.styles.ItemSelected.Render("▸ " + tag))
				} else {
					content.WriteString(a.styles.Help.Render("  " + tag))
				}
				content.WriteString("\n")
			}
		}

		content.WriteString("\n")
		helpText := "Enter: save • Esc: cancel"
		if len(a.tagSuggestions) > 0 {
			helpText = "Tab: accept • ↑↓: navigate • Enter: save • Esc: cancel"
		}
		content.WriteString(a.styles.Help.Render(helpText))

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

	case ModeMove:
		// Get item being moved
		displayItems := a.getDisplayItems()
		var itemName string
		if a.cursor < len(displayItems) {
			item := displayItems[a.cursor]
			if item.IsFolder() {
				itemName = item.Folder.Name
			} else {
				itemName = item.Bookmark.Title
			}
		}

		title.WriteString("Move Item\n\n")
		content.WriteString("Moving: " + itemName + "\n\n")

		// Filter input
		content.WriteString(a.moveFilterInput.View())
		content.WriteString("\n\n")

		// Render filtered folder list
		if len(a.moveFilteredFolders) == 0 {
			content.WriteString(a.styles.Empty.Render("No matching folders"))
			content.WriteString("\n")
		} else {
			maxVisible := a.layoutConfig.Modal.MoveMaxVisible
			start, end := layout.CalculateVisibleListItems(maxVisible, a.moveFolderIdx, len(a.moveFilteredFolders))

			for i := start; i < end; i++ {
				folder := a.moveFilteredFolders[i]
				if i == a.moveFolderIdx {
					content.WriteString(a.styles.ItemSelected.Render("▸ " + folder))
				} else {
					content.WriteString("  " + folder)
				}
				content.WriteString("\n")
			}
		}

		content.WriteString("\n")
		content.WriteString(a.styles.Help.Render("↑/↓: navigate • Enter: move • Esc: cancel"))

	case ModeFilter:
		// ModeFilter is handled inline in renderCurrentPane, not as a modal
		// This case should not be reached
		return a.renderView()
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

	visibleHeight := layout.CalculateVisibleHeight(height, 0)
	itemWidth := layout.CalculateItemWidth(width, a.layoutConfig.Pane)

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
			offset := layout.CalculateViewportOffset(currentIdx, len(items), visibleHeight)

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
				line := a.renderItem(item, isCurrentFolder, itemWidth)
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

	// Calculate header lines for filter
	headerLines := 0
	if a.mode == ModeFilter || a.filterQuery != "" {
		headerLines = 1
	}
	visibleHeight := layout.CalculateVisibleHeight(height, headerLines)
	itemWidth := layout.CalculateItemWidth(width, a.layoutConfig.Pane)

	// Show filter input or indicator at top
	if a.mode == ModeFilter {
		content.WriteString("/" + a.filterInput.View() + "\n")
	} else if a.filterQuery != "" {
		filterIndicator := a.styles.Tag.Render("/" + a.filterQuery)
		content.WriteString(filterIndicator + "\n")
	}

	// Get display items (filtered or all)
	displayItems := a.getDisplayItems()

	if len(displayItems) == 0 {
		if a.filterQuery != "" {
			content.WriteString(a.styles.Empty.Render("(no matches)"))
		} else {
			content.WriteString(a.styles.Empty.Render("(empty)"))
		}
	} else {
		// Calculate viewport offset to keep cursor visible
		offset := layout.CalculateViewportOffset(a.cursor, len(displayItems), visibleHeight)

		for i, item := range displayItems {
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
			line := a.renderItem(item, isSelected, itemWidth)
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

	visibleHeight := layout.CalculateVisibleHeight(height, 0)
	itemWidth := layout.CalculateItemWidth(width, a.layoutConfig.Pane)

	displayItems := a.getDisplayItems()
	if len(displayItems) > 0 && a.cursor < len(displayItems) {
		item := displayItems[a.cursor]

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
					content.WriteString(a.renderItem(child, false, itemWidth) + "\n")
				}
			}
		} else {
			// Show bookmark details
			b := item.Bookmark
			content.WriteString(a.styles.Title.Render(b.Title) + "\n\n")

			// URL (potentially truncated)
			url, _ := layout.TruncateText(b.URL, itemWidth, a.layoutConfig.Text)
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
	var prefix, text, suffix string
	var isPinned bool

	if item.IsFolder() {
		isPinned = item.Folder.Pinned
		if isPinned {
			prefix = "* "
		}
		text = item.Title()
		suffix = "/"
	} else {
		isPinned = item.Bookmark.Pinned
		if isPinned {
			prefix = "* "
		}
		text = item.Title()
	}

	// Truncate if too long using layout function
	line, _ := layout.TruncateWithPrefixSuffix(text, maxWidth, prefix, suffix, a.layoutConfig.Text)

	if selected {
		// Pad to fill width for selection highlight
		for len(line) < maxWidth {
			line += " "
		}
		return a.styles.ItemSelected.Render(line)
	}
	return a.styles.Item.Render(line)
}

// renderFuzzyFinder renders the fuzzy finder as a full-screen brutalist view.
func (a App) renderFuzzyFinder() string {
	// Brutalist style: no borders, full screen, top-left aligned (like help overlay)
	contentStyle := lipgloss.NewStyle().Padding(1, 2)

	// Calculate layout using config
	fuzzyLayout := layout.CalculateFuzzyLayout(a.width, a.height, a.layoutConfig.Fuzzy)
	listWidth := fuzzyLayout.ListWidth
	previewWidth := fuzzyLayout.PreviewWidth
	listHeight := fuzzyLayout.ListHeight
	listItemWidth := layout.CalculateItemWidth(listWidth, a.layoutConfig.Pane)
	previewItemWidth := layout.CalculateItemWidth(previewWidth, a.layoutConfig.Pane)

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
			line := a.renderFuzzyItem(match, isSelected, listItemWidth)
			results.WriteString(line + "\n")
		}
	}

	// Build preview content
	var preview strings.Builder
	if len(a.fuzzyMatches) > 0 && a.fuzzyCursor < len(a.fuzzyMatches) {
		selectedItem := a.fuzzyMatches[a.fuzzyCursor].Item
		if selectedItem.IsFolder() {
			preview.WriteString(selectedItem.Folder.Name + "/")
			preview.WriteString("\n\n")
			folderID := selectedItem.Folder.ID
			children := a.getItemsForFolder(&folderID)
			preview.WriteString(a.styles.Empty.Render(fmt.Sprintf("%d items", len(children))))
		} else {
			b := selectedItem.Bookmark
			preview.WriteString(a.styles.Title.Render(b.Title))
			preview.WriteString("\n\n")
			url, _ := layout.TruncateText(b.URL, previewItemWidth, a.layoutConfig.Text)
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

	// Style for results list (no border)
	resultsStyle := lipgloss.NewStyle().
		Width(listWidth).
		Height(listHeight)

	// Style for preview (no border)
	previewStyle := lipgloss.NewStyle().
		Width(previewWidth).
		Height(listHeight).
		PaddingLeft(2)

	resultsPane := resultsStyle.Render(strings.TrimRight(results.String(), "\n"))
	previewPane := previewStyle.Render(strings.TrimRight(preview.String(), "\n"))

	// Join panes horizontally
	panes := lipgloss.JoinHorizontal(lipgloss.Top, resultsPane, previewPane)

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
		a.styles.Help.Render("j/k: move  Enter: open  Esc: cancel"),
	)

	// Top-left aligned, brutalist style
	return lipgloss.Place(
		a.width,
		a.height,
		lipgloss.Left,
		lipgloss.Top,
		contentStyle.Render(content),
	)
}

// renderFuzzyItem renders a single item in the fuzzy results with highlighting.
func (a App) renderFuzzyItem(match fuzzyMatch, selected bool, maxWidth int) string {
	var suffix string
	if match.Item.IsFolder() {
		suffix = "/"
	}

	title := match.Item.Title()

	// Build highlighted string with ANSI codes for matched characters
	var line strings.Builder

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
	line.WriteString(suffix)

	result := line.String()

	// Use ANSI-aware truncation to preserve highlighting
	if layout.VisibleLength(result) > maxWidth {
		result = layout.TruncateANSIAware(result, maxWidth, a.layoutConfig.Text)
	}

	if selected {
		return a.styles.ItemSelected.Render(result)
	}
	return a.styles.Item.Render(result)
}

func (a App) renderHelpBar() string {
	// Industrial style: abbreviated brackets, minimal
	var status strings.Builder

	// Sort mode indicator (abbreviated)
	sortLabels := map[SortMode]string{
		SortManual:  "man",
		SortAlpha:   "a-z",
		SortCreated: "new",
		SortVisited: "vis",
	}
	status.WriteString("[ord:" + sortLabels[a.sortMode] + "]")

	// Confirm mode indicator (abbreviated)
	if a.confirmDelete {
		status.WriteString(" [cfm:on]")
	} else {
		status.WriteString(" [cfm:off]")
	}

	// Help hint
	status.WriteString(" [?:keys]")

	// Message (if any) - style based on type
	if a.messageText != "" {
		var msgStyle lipgloss.Style
		switch a.messageType {
		case MessageError:
			msgStyle = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "#CC3333", Dark: "#FF6666"}).
				Bold(true)
		case MessageWarning:
			msgStyle = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "#CC8800", Dark: "#FFAA00"}).
				Bold(true)
		case MessageSuccess:
			msgStyle = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "#338833", Dark: "#66CC66"}).
				Bold(true)
		default: // MessageInfo or MessageNone
			msgStyle = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "#4A7070", Dark: "#5F8787"}).
				Bold(true)
		}
		status.WriteString("  " + msgStyle.Render(a.messageText))
	}

	// Filter mode hint (only when filtering)
	var hints string
	if a.mode == ModeFilter {
		hints = "type to filter · enter: apply · esc: cancel"
	}

	// Style for status line
	lineStyle := lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#888888", Dark: "#606060"})

	content := lineStyle.Render(status.String())
	if hints != "" {
		content += "\n" + lineStyle.Render(hints)
	}

	// Only pad top, not bottom
	wrapperStyle := lipgloss.NewStyle().PaddingTop(1)
	return wrapperStyle.Render(content)
}

// renderQuickAddConfirm renders the AI quick add confirmation modal.
func (a App) renderQuickAddConfirm() string {
	modalWidth := layout.CalculateModalWidth(a.width, a.layoutConfig.Modal.LargeWidth, a.layoutConfig.Modal)

	accent := lipgloss.AdaptiveColor{Light: "#4A7070", Dark: "#5F8787"}
	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.ThickBorder()).
		BorderForeground(accent).
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
	visibleFolders := a.layoutConfig.Modal.QuickAddFoldersVisible
	start, end := layout.CalculateVisibleListItems(visibleFolders, a.quickAddFolderIdx, len(a.quickAddFolders))

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
	// Brutalist style: no border, just raw columns
	modalStyle := lipgloss.NewStyle().
		Padding(1, 2)

	// Left column: Navigation + Actions
	var left strings.Builder
	left.WriteString(a.styles.Title.Render("nav") + "\n")
	left.WriteString("j/k  move\n")
	left.WriteString("h/l  back/fwd\n")
	left.WriteString("gg   top\n")
	left.WriteString("G    bottom\n")
	left.WriteString("\n")
	left.WriteString(a.styles.Title.Render("act") + "\n")
	left.WriteString("l    open url\n")
	left.WriteString("Y    yank url\n")
	left.WriteString("*    pin/unpin\n")
	left.WriteString("s    search\n")
	left.WriteString("/    filter\n")
	left.WriteString("o    sort\n")
	left.WriteString("m    move\n")

	// Right column: Edit
	var right strings.Builder
	right.WriteString(a.styles.Title.Render("edit") + "\n")
	right.WriteString("a    add bookmark\n")
	right.WriteString("A    add folder\n")
	right.WriteString("i    AI add\n")
	right.WriteString("e    edit\n")
	right.WriteString("t    tags\n")
	right.WriteString("y    yank\n")
	right.WriteString("d    delete\n")
	right.WriteString("x    cut\n")
	right.WriteString("p/P  paste\n")
	right.WriteString("c    confirm toggle\n")
	right.WriteString("\n")
	right.WriteString(a.styles.Help.Render("[?/q/esc] close"))

	// Join columns
	leftCol := lipgloss.NewStyle().Width(a.layoutConfig.Modal.HelpLeftColumnWidth).Render(left.String())
	rightCol := lipgloss.NewStyle().Width(a.layoutConfig.Modal.HelpRightColumnWidth).Render(right.String())
	cols := lipgloss.JoinHorizontal(lipgloss.Top, leftCol, "  ", rightCol)

	// Top-left aligned, brutalist style
	return lipgloss.Place(
		a.width,
		a.height,
		lipgloss.Left,
		lipgloss.Top,
		modalStyle.Render(cols),
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

// Store returns the underlying store (for access from view).
func (a App) Store() *model.Store {
	return a.store
}
