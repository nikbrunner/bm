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
	atRoot := a.browser.CurrentFolderID == nil
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

	// Add breadcrumb above columns
	breadcrumb := a.renderBreadcrumb()

	// Add help bar at bottom
	helpBar := a.renderHelpBar()

	content := a.styles.App.Render(
		lipgloss.JoinVertical(lipgloss.Left, breadcrumb, columns, helpBar),
	)

	// Use Place to ensure exact terminal dimensions and prevent overflow
	return lipgloss.Place(a.width, a.height, lipgloss.Left, lipgloss.Top, content)
}

// renderBreadcrumb renders the folder path breadcrumb above the Miller columns.
func (a App) renderBreadcrumb() string {
	var path string

	if a.browser.CurrentFolderID == nil {
		// At root - show app name
		path = "bm"
	} else {
		// In a subfolder - show full path
		path = a.store.GetFolderPath(a.browser.CurrentFolderID)
	}

	// Calculate available width (terminal width minus app padding: left=2, right=2)
	availableWidth := a.width - 4

	// Truncate from left if path is too long
	path = layout.TruncatePathFromLeft(path, availableWidth, a.layoutConfig.Text)

	return a.styles.Breadcrumb.Render(path)
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
	modalWidth := layout.CalculateModalWidth(a.width, a.layoutConfig.Modal.DefaultWidthPercent, a.layoutConfig.Modal)
	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.ThickBorder()).
		BorderForeground(accent).
		Padding(1, 2).
		Width(modalWidth)

	switch a.mode {
	case ModeAddFolder:
		title.WriteString("Add Folder\n\n")
		content.WriteString("Name:\n")
		content.WriteString(a.modal.TitleInput.View())

	case ModeAddBookmark:
		title.WriteString("Add Bookmark\n\n")
		content.WriteString("Title:\n")
		content.WriteString(a.modal.TitleInput.View())
		content.WriteString("\n\n")
		content.WriteString("URL:\n")
		content.WriteString(a.modal.URLInput.View())

	case ModeEditFolder:
		title.WriteString("Edit Folder\n\n")
		content.WriteString("Name:\n")
		content.WriteString(a.modal.TitleInput.View())

	case ModeEditBookmark:
		title.WriteString("Edit Bookmark\n\n")
		content.WriteString("Title:\n")
		content.WriteString(a.modal.TitleInput.View())
		content.WriteString("\n\n")
		content.WriteString("URL:\n")
		content.WriteString(a.modal.URLInput.View())

	case ModeEditTags:
		title.WriteString("Edit Tags\n\n")
		content.WriteString("Tags (comma-separated):\n")
		content.WriteString(a.modal.TagsInput.View())
		content.WriteString("\n")

		// Render tag suggestions if any
		if len(a.modal.TagSuggestions) > 0 {
			content.WriteString("\n")
			for i, tag := range a.modal.TagSuggestions {
				if i == a.modal.TagSuggestionIdx {
					content.WriteString(a.styles.ItemSelected.Render("▸ " + tag))
				} else {
					content.WriteString(a.styles.Help.Render("  " + tag))
				}
				content.WriteString("\n")
			}
		}

	case ModeConfirmDelete:
		// Determine action and item type
		action := "Delete"
		if a.modal.CutMode {
			action = "Cut"
		}

		// Try folder first, then bookmark
		var itemType, itemName string
		if folder := a.store.GetFolderByID(a.modal.EditItemID); folder != nil {
			itemType = "Folder"
			itemName = folder.Name
		} else if bookmark := a.store.GetBookmarkByID(a.modal.EditItemID); bookmark != nil {
			itemType = "Bookmark"
			itemName = bookmark.Title
		} else {
			itemType = "Item"
			itemName = "this item"
		}

		title.WriteString(action + " " + itemType + "?\n\n")
		content.WriteString("\"" + itemName + "\"\n\n")
		content.WriteString(a.styles.Help.Render("This action cannot be undone.") + "\n\n")
		content.WriteString(a.renderHintsInline([]Hint{
			{Key: "Enter", Desc: "confirm"},
			{Key: "Esc", Desc: "cancel"},
		}))

	case ModeSearch:
		// Render full-screen fuzzy finder
		return a.renderFuzzyFinder()

	case ModeHelp:
		// Render help overlay
		return a.renderHelpOverlay()

	case ModeQuickAdd:
		title.WriteString("AI Quick Add\n\n")
		content.WriteString("URL:\n")
		content.WriteString(a.quickAdd.Input.View())

	case ModeQuickAddLoading:
		title.WriteString("AI Quick Add\n\n")
		content.WriteString("Analyzing link...\n\n")
		content.WriteString(a.styles.Empty.Render("Please wait while AI suggests title, folder, and tags"))

	case ModeReadLaterLoading:
		title.WriteString("Adding to " + a.config.QuickAddFolder + "\n\n")
		content.WriteString("URL:\n")
		content.WriteString(a.styles.URL.Render(a.readLaterURL) + "\n\n")
		content.WriteString(a.styles.Empty.Render("Fetching title and tags..."))

	case ModeQuickAddConfirm:
		return a.renderQuickAddConfirm()

	case ModeMove:
		// Get item being moved
		displayItems := a.getDisplayItems()
		var itemName string
		if a.browser.Cursor < len(displayItems) {
			item := displayItems[a.browser.Cursor]
			if item.IsFolder() {
				itemName = item.Folder.Name
			} else {
				itemName = item.Bookmark.Title
			}
		}

		title.WriteString("Move Item\n\n")
		content.WriteString("Moving: " + itemName + "\n\n")

		// Filter input
		content.WriteString(a.move.FilterInput.View())
		content.WriteString("\n\n")

		// Render filtered folder list
		if len(a.move.FilteredFolders) == 0 {
			content.WriteString(a.styles.Empty.Render("No matching folders"))
			content.WriteString("\n")
		} else {
			maxVisible := a.layoutConfig.Modal.MoveMaxVisible
			start, end := layout.CalculateVisibleListItems(maxVisible, a.move.FolderIdx, len(a.move.FilteredFolders))

			for i := start; i < end; i++ {
				folder := a.move.FilteredFolders[i]
				if i == a.move.FolderIdx {
					content.WriteString(a.styles.ItemSelected.Render("▸ " + folder))
				} else {
					content.WriteString("  " + folder)
				}
				content.WriteString("\n")
			}
		}

	case ModeFilter:
		// ModeFilter is handled inline in renderCurrentPane, not as a modal
		// This case should not be reached
		return a.renderView()
	}

	modalContent := a.styles.Title.Render(title.String()) + content.String()

	// Place modal in center, then add help bar at bottom
	modal := lipgloss.Place(
		a.width,
		a.height-3, // Leave room for help bar
		lipgloss.Center,
		lipgloss.Center,
		modalStyle.Render(modalContent),
	)

	return lipgloss.JoinVertical(lipgloss.Left, modal, a.renderHelpBar())
}

func (a App) renderParentPane(width, height int) string {
	var content strings.Builder

	visibleHeight := layout.CalculateVisibleHeight(height, 0)
	itemWidth := layout.CalculateItemWidth(width, a.layoutConfig.Pane)

	if a.browser.CurrentFolderID == nil {
		// At root - show app title
		content.WriteString(a.styles.Title.Render("bm"))
		content.WriteString("\n")
		content.WriteString(a.styles.Empty.Render("bookmarks"))
	} else {
		// Show parent folder contents
		currentFolder := a.store.GetFolderByID(*a.browser.CurrentFolderID)
		if currentFolder != nil {
			// Show the parent folder's contents (siblings of current)
			parentFolderID := currentFolder.ParentID
			items := a.getItemsForFolder(parentFolderID)

			// Find index of current folder for viewport calculation
			currentIdx := 0
			for i, item := range items {
				if item.IsFolder() && item.Folder.ID == *a.browser.CurrentFolderID {
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
				isCurrentFolder := item.IsFolder() && item.Folder.ID == *a.browser.CurrentFolderID
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
	if a.mode == ModeFilter || a.search.FilterQuery != "" {
		headerLines = 1
	}
	visibleHeight := layout.CalculateVisibleHeight(height, headerLines)
	itemWidth := layout.CalculateItemWidth(width, a.layoutConfig.Pane)

	// Show filter input or indicator at top
	if a.mode == ModeFilter {
		content.WriteString("/" + a.search.FilterInput.View() + "\n")
	} else if a.search.FilterQuery != "" {
		filterIndicator := a.styles.Tag.Render("/" + a.search.FilterQuery)
		content.WriteString(filterIndicator + "\n")
	}

	// Get display items (filtered or all)
	displayItems := a.getDisplayItems()

	if len(displayItems) == 0 {
		if a.search.FilterQuery != "" {
			content.WriteString(a.styles.Empty.Render("(no matches)"))
		} else {
			content.WriteString(a.styles.Empty.Render("(empty)"))
		}
	} else {
		// Calculate viewport offset to keep cursor visible
		offset := layout.CalculateViewportOffset(a.browser.Cursor, len(displayItems), visibleHeight)

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
			isSelected := a.focusedPane == PaneBrowser && i == a.browser.Cursor
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
	if len(displayItems) > 0 && a.browser.Cursor < len(displayItems) {
		item := displayItems[a.browser.Cursor]

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
	if len(a.search.FuzzyMatches) == 0 {
		results.WriteString(a.styles.Empty.Render("No matches"))
	} else {
		for i, match := range a.search.FuzzyMatches {
			if i >= listHeight {
				break
			}
			isSelected := i == a.search.FuzzyCursor
			line := a.renderFuzzyItem(match, isSelected, listItemWidth)
			results.WriteString(line + "\n")
		}
	}

	// Build preview content
	var preview strings.Builder
	if len(a.search.FuzzyMatches) > 0 && a.search.FuzzyCursor < len(a.search.FuzzyMatches) {
		selectedItem := a.search.FuzzyMatches[a.search.FuzzyCursor].Item
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
	countStr := fmt.Sprintf("%d results", len(a.search.FuzzyMatches))
	if len(a.search.FuzzyMatches) == 1 {
		countStr = "1 result"
	}

	content := lipgloss.JoinVertical(lipgloss.Left,
		a.styles.Title.Render("Find")+"  "+a.styles.Empty.Render(countStr),
		"",
		"> "+a.search.Input.Value()+"█",
		"",
		panes,
	)

	// Top-left aligned, leave room for help bar at bottom
	main := lipgloss.Place(
		a.width,
		a.height-3,
		lipgloss.Left,
		lipgloss.Top,
		contentStyle.Render(content),
	)

	return lipgloss.JoinVertical(lipgloss.Left, main, a.renderHelpBar())
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
			// Use \033[22;24m to reset only bold/underline, preserving background color
			line.WriteString("\033[1;4m")
			line.WriteRune(r)
			line.WriteString("\033[22;24m")
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

	// Pad to full width for consistent selection highlighting
	visibleLen := layout.VisibleLength(result)
	if visibleLen < maxWidth {
		result += strings.Repeat(" ", maxWidth-visibleLen)
	}

	if selected {
		return a.styles.ItemSelected.Render(result)
	}
	return a.styles.Item.Render(result)
}

func (a App) renderHelpBar() string {
	lineStyle := lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#888888", Dark: "#606060"})

	var lines []string

	// Line 1: Message (if any)
	if a.messageText != "" {
		lines = append(lines, a.renderMessageLine())
	}

	// Line 2: Toggle states (only in normal/filter modes)
	if a.mode == ModeNormal || a.mode == ModeFilter {
		lines = append(lines, lineStyle.Render(a.renderStatusToggles()))
	}

	// Line 3: Contextual keyboard hints
	hintsStr := a.renderHints(a.getContextualHints())
	if hintsStr != "" {
		lines = append(lines, lineStyle.Render(hintsStr))
	}

	// Only pad top, not bottom
	wrapperStyle := lipgloss.NewStyle().PaddingTop(1)
	return wrapperStyle.Render(strings.Join(lines, "\n"))
}

// renderMessageLine renders the styled message with prefix icon based on type.
func (a App) renderMessageLine() string {
	var msgStyle lipgloss.Style
	var prefix string

	switch a.messageType {
	case MessageError:
		msgStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#CC3333", Dark: "#FF6666"}).
			Bold(true)
		prefix = "✗ "
	case MessageWarning:
		msgStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#CC8800", Dark: "#FFAA00"}).
			Bold(true)
		prefix = "⚠ "
	case MessageSuccess:
		msgStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#338833", Dark: "#66CC66"}).
			Bold(true)
		prefix = "✓ "
	default: // MessageInfo
		msgStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#4A7070", Dark: "#5F8787"}).
			Bold(true)
		prefix = ""
	}

	return msgStyle.Render(prefix + a.messageText)
}

// renderStatusToggles renders the [ord:X] [cfm:X] indicators.
func (a App) renderStatusToggles() string {
	var status strings.Builder

	// Sort mode indicator (abbreviated)
	sortLabels := map[SortMode]string{
		SortManual:  "man",
		SortAlpha:   "a-z",
		SortCreated: "new",
		SortVisited: "vis",
	}
	status.WriteString("[ord:" + sortLabels[a.browser.SortMode] + "]")

	// Confirm mode indicator (abbreviated)
	if a.confirmDelete {
		status.WriteString(" [cfm:on]")
	} else {
		status.WriteString(" [cfm:off]")
	}

	return status.String()
}

// renderQuickAddConfirm renders the AI quick add confirmation modal.
func (a App) renderQuickAddConfirm() string {
	modalWidth := layout.CalculateModalWidth(a.width, a.layoutConfig.Modal.LargeWidthPercent, a.layoutConfig.Modal)

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
	if a.modal.TitleInput.Focused() {
		titleLabel = a.styles.Tag.Render("Title:")
	}
	content.WriteString(titleLabel + "\n")
	content.WriteString(a.modal.TitleInput.View())
	content.WriteString("\n\n")

	// Folder picker
	folderLabel := "Folder:"
	if !a.modal.TitleInput.Focused() && !a.modal.TagsInput.Focused() {
		folderLabel = a.styles.Tag.Render("Folder:")
	}
	content.WriteString(folderLabel + "\n")

	// Show folder options with selection
	visibleFolders := a.layoutConfig.Modal.QuickAddFoldersVisible
	start, end := layout.CalculateVisibleListItems(visibleFolders, a.quickAdd.FolderIdx, len(a.quickAdd.Folders))

	for i := start; i < end; i++ {
		folder := a.quickAdd.Folders[i]
		if i == a.quickAdd.FolderIdx {
			content.WriteString(a.styles.ItemSelected.Render("> " + folder))
		} else {
			content.WriteString("  " + folder)
		}
		content.WriteString("\n")
	}
	content.WriteString("\n")

	// Tags input
	tagsLabel := "Tags:"
	if a.modal.TagsInput.Focused() {
		tagsLabel = a.styles.Tag.Render("Tags:")
	}
	content.WriteString(tagsLabel + "\n")
	content.WriteString(a.modal.TagsInput.View())

	// Place modal in center, then add help bar at bottom
	modal := lipgloss.Place(
		a.width,
		a.height-3,
		lipgloss.Center,
		lipgloss.Center,
		modalStyle.Render(content.String()),
	)

	return lipgloss.JoinVertical(lipgloss.Left, modal, a.renderHelpBar())
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
