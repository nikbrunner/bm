package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

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
			line := a.renderPinnedItem(item, isSelected, itemWidth, i)
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

// renderPinnedItem renders an item in the pinned pane with [N] prefix.
func (a App) renderPinnedItem(item Item, selected bool, maxWidth int, index int) string {
	var text, suffix string

	// Show [N] prefix for quick access (1-indexed)
	prefix := fmt.Sprintf("[%d] ", index+1)

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

		// Check for batch operation
		if len(a.modal.DeleteItems) > 0 {
			count := len(a.modal.DeleteItems)
			title.WriteString(action + " " + strconv.Itoa(count) + " items?\n\n")
			content.WriteString(a.styles.Help.Render("This action cannot be undone.") + "\n\n")
			content.WriteString(a.renderHintsInline([]Hint{
				{Key: "Enter", Desc: "confirm"},
				{Key: "Esc", Desc: "cancel"},
			}))
		} else {
			// Single item - try folder first, then bookmark
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
		}

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

	case ModeQuickAddCreateFolder:
		return a.renderQuickAddCreateFolder()

	case ModeCullMenu:
		return a.renderCullMenu()

	case ModeCullLoading:
		return a.renderCullLoading()

	case ModeCullResults:
		return a.renderCullResults()

	case ModeCullInspect:
		return a.renderCullInspect()

	case ModeOrganizeMenu:
		return a.renderOrganizeMenu()
	case ModeOrganizeLoading:
		return a.renderOrganizeLoading()
	case ModeOrganizeResults:
		return a.renderOrganizeResults()

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

func (a App) renderItem(item Item, isCursor bool, maxWidth int) string {
	var prefix, text, suffix string
	var isPinned bool
	isMarked := a.selection.IsSelected(item.ID())

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

	// Add selection marker for marked items (not cursor)
	if isMarked && !isCursor {
		prefix = "▸ " + prefix
	}

	// Truncate if too long using layout function
	line, _ := layout.TruncateWithPrefixSuffix(text, maxWidth, prefix, suffix, a.layoutConfig.Text)

	// Determine which style to use based on cursor and selection state
	needsHighlight := isCursor || isMarked
	if needsHighlight {
		// Pad to fill width for highlight
		for len(line) < maxWidth {
			line += " "
		}
		if isCursor && isMarked {
			return a.styles.ItemMarkedCursor.Render(line)
		} else if isCursor {
			return a.styles.ItemSelected.Render(line)
		}
		return a.styles.ItemMarked.Render(line)
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

	// Determine title based on source
	var title string
	switch a.search.Source {
	case SourceRecent:
		title = "Recent Bookmarks"
	default:
		title = "Find"
	}

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
			// For SourceRecent, show folder path; for SourceAll, no path
			showFolderPath := a.search.Source == SourceRecent
			line := a.renderFuzzyItemWithPath(match, isSelected, listItemWidth, showFolderPath)
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
		a.styles.Title.Render(title)+"  "+a.styles.Empty.Render(countStr),
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

// renderFuzzyItemWithPath renders a fuzzy item with an optional folder path column.
// When showPath is true, the folder path is shown right-aligned next to the title.
func (a App) renderFuzzyItemWithPath(match fuzzyMatch, selected bool, maxWidth int, showPath bool) string {
	var suffix string
	if match.Item.IsFolder() {
		suffix = "/"
	}

	title := match.Item.Title()

	// Get folder path if showing paths (for bookmarks only)
	var folderPath string
	if showPath && !match.Item.IsFolder() {
		folderID := match.Item.Bookmark.FolderID
		if folderID == nil {
			folderPath = "─" // Root-level bookmark
		} else {
			path := a.store.GetFolderPath(folderID)
			// Remove leading slash if present
			if len(path) > 0 && path[0] == '/' {
				path = path[1:]
			}
			folderPath = path
		}
	}

	// Calculate space for path column (roughly 30% of width, min 10 chars)
	pathColWidth := 0
	if showPath && folderPath != "" {
		pathColWidth = maxWidth * 30 / 100
		if pathColWidth < 10 {
			pathColWidth = 10
		}
	}
	titleMaxWidth := maxWidth - pathColWidth

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
			line.WriteString("\033[22;24m")
		} else {
			line.WriteRune(r)
		}
	}
	line.WriteString(suffix)

	result := line.String()

	// Truncate title if needed
	if layout.VisibleLength(result) > titleMaxWidth {
		result = layout.TruncateANSIAware(result, titleMaxWidth, a.layoutConfig.Text)
	}

	// Pad title to fixed width
	visibleLen := layout.VisibleLength(result)
	if visibleLen < titleMaxWidth {
		result += strings.Repeat(" ", titleMaxWidth-visibleLen)
	}

	// Add folder path column if showing paths
	if showPath && folderPath != "" && pathColWidth > 3 {
		pathRunes := []rune(folderPath)
		pathVisualLen := len(pathRunes) // visual length in characters

		// Truncate path from left if needed (keep rightmost part with ellipsis)
		if pathVisualLen > pathColWidth-1 {
			keepLen := pathColWidth - 2 // space for "…" and at least one space padding
			if keepLen > 0 && keepLen < pathVisualLen {
				folderPath = "…" + string(pathRunes[pathVisualLen-keepLen:])
				pathVisualLen = keepLen + 1 // ellipsis + kept chars
			} else {
				folderPath = "…"
				pathVisualLen = 1
			}
		}

		// Pad path to column width (right-align)
		padding := pathColWidth - pathVisualLen
		if padding < 0 {
			padding = 0
		}
		pathPadded := strings.Repeat(" ", padding) + folderPath
		// Apply dimmed style to path
		result += a.styles.Empty.Render(pathPadded)
	}

	if selected {
		return a.styles.ItemSelected.Render(result)
	}
	return a.styles.Item.Render(result)
}

func (a App) renderHelpBar() string {
	var lines []string

	// Line 1: Empty spacer OR message (message replaces the gap)
	if a.messageText != "" {
		lines = append(lines, a.renderMessageLine())
	} else {
		lines = append(lines, "") // Empty line provides gap when no message
	}

	// Line 2: Toggle hints and states (only in normal/filter modes)
	if a.mode == ModeNormal || a.mode == ModeFilter {
		lines = append(lines, a.renderStatusToggles())
	}

	// Line 3: Local (contextual) keyboard hints
	localHints := a.renderHints(a.getContextualHints())
	if localHints != "" {
		lines = append(lines, a.styles.HintLabel.Render("Local  ")+localHints)
	}

	// Line 4: Global keyboard hints (only in normal mode - modals have their own flow)
	if a.mode == ModeNormal {
		globalHints := a.renderHintSlice(a.getGlobalHints())
		if globalHints != "" {
			lines = append(lines, a.styles.HintLabel.Render("Global ")+globalHints)
		}
	}

	return strings.Join(lines, "\n")
}

// renderHintSlice renders a slice of hints in horizontal format.
func (a App) renderHintSlice(hints []Hint) string {
	if len(hints) == 0 {
		return ""
	}

	parts := make([]string, len(hints))
	for i, h := range hints {
		parts[i] = a.renderHint(h)
	}
	return strings.Join(parts, " ")
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

// renderStatusToggles renders the toggle hints and [ord:X] [cfm:X] indicators.
func (a App) renderStatusToggles() string {
	var status strings.Builder

	// Toggle hints
	status.WriteString(a.styles.HintLabel.Render("Toggle "))
	status.WriteString(a.styles.HintKey.Render("to") + ":" + a.styles.HintDesc.Render("order") + " ")
	status.WriteString(a.styles.HintKey.Render("tc") + ":" + a.styles.HintDesc.Render("confirm") + "  ")

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

	// Folder section with filter input
	folderFilterFocused := a.quickAdd.FilterInput.Focused()
	folderLabel := "Folder:"
	if folderFilterFocused {
		folderLabel = a.styles.Tag.Render("Folder:")
	}
	content.WriteString(folderLabel + "\n")

	// Filter input
	content.WriteString(a.quickAdd.FilterInput.View())
	content.WriteString("\n")

	// Show folder options with selection
	visibleFolders := a.layoutConfig.Modal.QuickAddFoldersVisible

	if len(a.quickAdd.FilteredFolders) == 0 {
		// No matches - show create option
		if a.quickAdd.FilterInput.Value() != "" {
			createOption := "[Create: " + a.quickAdd.FilterInput.Value() + "]"
			content.WriteString(a.styles.ItemSelected.Render("> " + createOption))
			content.WriteString("\n")
		} else {
			content.WriteString(a.styles.Empty.Render("No folders"))
			content.WriteString("\n")
		}
	} else {
		start, end := layout.CalculateVisibleListItems(visibleFolders, a.quickAdd.FolderIdx, len(a.quickAdd.FilteredFolders))

		for i := start; i < end; i++ {
			folder := a.quickAdd.FilteredFolders[i]

			// Add indicator for special folders
			displayFolder := folder
			if i == 0 && len(a.quickAdd.Folders) > 0 && folder == a.quickAdd.Folders[0] && a.browser.CurrentFolderID != nil {
				displayFolder += " (current)"
			}

			if i == a.quickAdd.FolderIdx {
				content.WriteString(a.styles.ItemSelected.Render("> " + displayFolder))
			} else {
				content.WriteString("  " + displayFolder)
			}
			content.WriteString("\n")
		}
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

// renderQuickAddCreateFolder renders the parent folder picker for creating a new folder.
func (a App) renderQuickAddCreateFolder() string {
	modalWidth := layout.CalculateModalWidth(a.width, a.layoutConfig.Modal.DefaultWidthPercent, a.layoutConfig.Modal)

	accent := lipgloss.AdaptiveColor{Light: "#4A7070", Dark: "#5F8787"}
	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.ThickBorder()).
		BorderForeground(accent).
		Padding(1, 2).
		Width(modalWidth)

	var content strings.Builder
	content.WriteString(a.styles.Title.Render("Create Folder"))
	content.WriteString("\n\n")
	content.WriteString("Create '" + a.quickAddCreateFolder.NewFolderName + "' in:\n\n")

	// Show parent options with selection
	visibleFolders := a.layoutConfig.Modal.MoveMaxVisible
	start, end := layout.CalculateVisibleListItems(visibleFolders, a.quickAddCreateFolder.ParentIdx, len(a.quickAddCreateFolder.ParentOptions))

	for i := start; i < end; i++ {
		folder := a.quickAddCreateFolder.ParentOptions[i]
		displayFolder := folder
		if folder == "/" {
			displayFolder = "/ (root)"
		}
		if i == a.quickAddCreateFolder.ParentIdx {
			content.WriteString(a.styles.ItemSelected.Render("> " + displayFolder))
		} else {
			content.WriteString("  " + displayFolder)
		}
		content.WriteString("\n")
	}

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
	left.WriteString("0    go to pins\n")
	left.WriteString("\n")
	left.WriteString(a.styles.Title.Render("pins") + "\n")
	left.WriteString("1-9  open pin\n")
	left.WriteString("J/K  reorder\n")
	left.WriteString("\n")
	left.WriteString(a.styles.Title.Render("act") + "\n")
	left.WriteString("l    open url\n")
	left.WriteString("Y    yank url\n")
	left.WriteString("*    pin/unpin\n")
	left.WriteString("s    search\n")
	left.WriteString("R    recent\n")
	left.WriteString("/    filter\n")
	left.WriteString("o    sort mode\n")
	left.WriteString("m    move\n")

	// Right column: Edit + Selection
	var right strings.Builder
	right.WriteString(a.styles.Title.Render("edit") + "\n")
	right.WriteString("a    add bookmark\n")
	right.WriteString("A    add folder\n")
	right.WriteString("i    AI add\n")
	right.WriteString("L    read later\n")
	right.WriteString("O    organize\n")
	right.WriteString("e    edit\n")
	right.WriteString("t    tags\n")
	right.WriteString("y    yank\n")
	right.WriteString("d    delete\n")
	right.WriteString("x    cut\n")
	right.WriteString("p/P  paste\n")
	right.WriteString("c    confirm toggle\n")
	right.WriteString("\n")
	right.WriteString(a.styles.Title.Render("select") + "\n")
	right.WriteString("v    select item\n")
	right.WriteString("V    visual mode\n")
	right.WriteString("Esc  clear select\n")
	right.WriteString("\n")
	right.WriteString(a.styles.Title.Render("tools") + "\n")
	right.WriteString("C    cull dead links\n")
	right.WriteString("\n")
	right.WriteString(a.styles.Help.Render("[?/esc] close  [q] quit"))

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

// renderCullMenu renders the menu to choose between fresh or cached cull.
func (a App) renderCullMenu() string {
	modalWidth := layout.CalculateModalWidth(a.width, a.layoutConfig.Modal.DefaultWidthPercent, a.layoutConfig.Modal)

	accent := lipgloss.AdaptiveColor{Light: "#4A7070", Dark: "#5F8787"}
	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.ThickBorder()).
		BorderForeground(accent).
		Padding(1, 2).
		Width(modalWidth)

	var content strings.Builder
	content.WriteString(a.styles.Title.Render("Cull Dead Links"))
	content.WriteString("\n\n")

	// Menu options
	options := []string{
		"Run fresh check",
		"",
	}

	// Format cached option with age and count
	if a.cull.HasCache {
		age := formatTimeAgo(a.cull.CacheTime)
		count := a.countCachedProblems()
		options[1] = fmt.Sprintf("Use cached results (%s, %d issues)", age, count)
	}

	for i, opt := range options {
		if opt == "" {
			continue // Skip empty (no cache case shouldn't happen since menu only shows when cache exists)
		}
		line := opt
		if i == a.cull.MenuCursor {
			// Pad for selection highlight
			padded := line
			for len(padded) < modalWidth-8 {
				padded += " "
			}
			content.WriteString(a.styles.ItemSelected.Render("▸ " + padded))
		} else {
			content.WriteString("  " + line)
		}
		content.WriteString("\n")
	}

	modal := lipgloss.Place(
		a.width,
		a.height-3,
		lipgloss.Center,
		lipgloss.Center,
		modalStyle.Render(content.String()),
	)

	return lipgloss.JoinVertical(lipgloss.Left, modal, a.renderHelpBar())
}

// renderOrganizeMenu renders the menu to choose between fresh or cached organize.
func (a App) renderOrganizeMenu() string {
	modalWidth := layout.CalculateModalWidth(a.width, a.layoutConfig.Modal.DefaultWidthPercent, a.layoutConfig.Modal)

	accent := lipgloss.AdaptiveColor{Light: "#6B8E23", Dark: "#9ACD32"}
	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.ThickBorder()).
		BorderForeground(accent).
		Padding(1, 2).
		Width(modalWidth)

	var content strings.Builder
	content.WriteString(a.styles.Title.Render("Organize"))
	content.WriteString("\n\n")

	// Menu options
	options := []string{
		"Run fresh analysis",
		"",
	}

	// Format cached option with age and count
	if a.organize.HasCache {
		age := formatTimeAgo(a.organize.CacheTime)
		count := a.countCachedOrganizeSuggestions()
		options[1] = fmt.Sprintf("Use cached results (%s, %d items)", age, count)
	}

	for i, opt := range options {
		if opt == "" {
			continue // Skip empty
		}
		line := opt
		if i == a.organize.MenuCursor {
			// Pad for selection highlight
			padded := line
			for len(padded) < modalWidth-8 {
				padded += " "
			}
			content.WriteString(a.styles.ItemSelected.Render("▸ " + padded))
		} else {
			content.WriteString("  " + line)
		}
		content.WriteString("\n")
	}

	modal := lipgloss.Place(
		a.width,
		a.height-3,
		lipgloss.Center,
		lipgloss.Center,
		modalStyle.Render(content.String()),
	)

	return lipgloss.JoinVertical(lipgloss.Left, modal, a.renderHelpBar())
}

// formatTimeAgo formats a duration since a timestamp in human-readable form.
func formatTimeAgo(t time.Time) string {
	d := time.Since(t)
	if d < time.Minute {
		return "just now"
	} else if d < time.Hour {
		m := int(d.Minutes())
		if m == 1 {
			return "1m ago"
		}
		return fmt.Sprintf("%dm ago", m)
	} else if d < 24*time.Hour {
		h := int(d.Hours())
		if h == 1 {
			return "1h ago"
		}
		return fmt.Sprintf("%dh ago", h)
	}
	days := int(d.Hours() / 24)
	if days == 1 {
		return "1d ago"
	}
	return fmt.Sprintf("%dd ago", days)
}

// renderCullLoading renders the loading screen during URL checking.
func (a App) renderCullLoading() string {
	modalWidth := layout.CalculateModalWidth(a.width, a.layoutConfig.Modal.DefaultWidthPercent, a.layoutConfig.Modal)

	accent := lipgloss.AdaptiveColor{Light: "#4A7070", Dark: "#5F8787"}
	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.ThickBorder()).
		BorderForeground(accent).
		Padding(1, 2).
		Width(modalWidth)

	var content strings.Builder
	content.WriteString(a.styles.Title.Render("Cull Dead Links"))
	content.WriteString("\n\n")
	content.WriteString(fmt.Sprintf("Checking %d bookmarks...\n\n", a.cull.Total))

	// Progress bar
	if a.cull.Total > 0 {
		progress := float64(a.cull.Progress) / float64(a.cull.Total)
		barWidth := modalWidth - 10
		filled := int(progress * float64(barWidth))
		empty := barWidth - filled

		bar := strings.Repeat("█", filled) + strings.Repeat("░", empty)
		content.WriteString(bar + "\n\n")
		content.WriteString(fmt.Sprintf("[%d/%d]", a.cull.Progress, a.cull.Total))
	}

	modal := lipgloss.Place(
		a.width,
		a.height-3,
		lipgloss.Center,
		lipgloss.Center,
		modalStyle.Render(content.String()),
	)

	return lipgloss.JoinVertical(lipgloss.Left, modal, a.renderHelpBar())
}

// renderCullResults renders the group list after URL checking.
func (a App) renderCullResults() string {
	modalWidth := layout.CalculateModalWidth(a.width, a.layoutConfig.Modal.LargeWidthPercent, a.layoutConfig.Modal)

	accent := lipgloss.AdaptiveColor{Light: "#4A7070", Dark: "#5F8787"}
	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.ThickBorder()).
		BorderForeground(accent).
		Padding(1, 2).
		Width(modalWidth)

	var content strings.Builder
	content.WriteString(a.styles.Title.Render("Cull Results"))
	content.WriteString("\n\n")

	if len(a.cull.Groups) == 0 {
		content.WriteString(a.styles.Empty.Render("All bookmarks healthy!"))
	} else {
		// Count totals
		totalDead := 0
		totalUnreachable := 0
		for _, g := range a.cull.Groups {
			if g.Label == "DEAD" {
				totalDead += len(g.Results)
			} else {
				totalUnreachable += len(g.Results)
			}
		}

		// Summary line
		summary := fmt.Sprintf("Found %d dead, %d unreachable", totalDead, totalUnreachable)
		content.WriteString(a.styles.Help.Render(summary))
		content.WriteString("\n\n")

		// Group list
		maxVisible := 10
		start, end := layout.CalculateVisibleListItems(maxVisible, a.cull.GroupCursor, len(a.cull.Groups))

		for i := start; i < end; i++ {
			group := a.cull.Groups[i]
			line := fmt.Sprintf("%s (%d)", group.Label, len(group.Results))

			if i == a.cull.GroupCursor {
				// Pad for selection highlight
				padded := line
				for len(padded) < modalWidth-8 {
					padded += " "
				}
				content.WriteString(a.styles.ItemSelected.Render("▸ " + padded))
			} else {
				content.WriteString("  " + line)
			}
			content.WriteString("\n")
		}
	}

	modal := lipgloss.Place(
		a.width,
		a.height-3,
		lipgloss.Center,
		lipgloss.Center,
		modalStyle.Render(content.String()),
	)

	return lipgloss.JoinVertical(lipgloss.Left, modal, a.renderHelpBar())
}

// renderCullInspect renders the bookmark list within a cull group.
func (a App) renderCullInspect() string {
	modalWidth := layout.CalculateModalWidth(a.width, a.layoutConfig.Modal.LargeWidthPercent, a.layoutConfig.Modal)

	accent := lipgloss.AdaptiveColor{Light: "#4A7070", Dark: "#5F8787"}
	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.ThickBorder()).
		BorderForeground(accent).
		Padding(1, 2).
		Width(modalWidth)

	var content strings.Builder

	group := a.cull.CurrentGroup()
	if group == nil {
		content.WriteString(a.styles.Empty.Render("No group selected"))
	} else {
		// Title with count
		title := fmt.Sprintf("%s (%d)", group.Label, len(group.Results))
		content.WriteString(a.styles.Title.Render(title))
		content.WriteString("\n\n")

		// Bookmark list - compact view
		maxVisible := 12
		itemWidth := modalWidth - 6

		start, end := layout.CalculateVisibleListItems(maxVisible, a.cull.ItemCursor, len(group.Results))

		for i := start; i < end; i++ {
			r := group.Results[i]
			isSelected := i == a.cull.ItemCursor

			// Title (truncated)
			titleLine := r.Bookmark.Title
			if len(titleLine) > itemWidth-2 {
				titleLine = titleLine[:itemWidth-5] + "..."
			}

			// URL (truncated, shorter)
			urlLine := r.Bookmark.URL
			maxURLLen := itemWidth - 4
			if len(urlLine) > maxURLLen {
				urlLine = urlLine[:maxURLLen-3] + "..."
			}

			if isSelected {
				content.WriteString(a.styles.ItemSelected.Render("▸ " + titleLine))
				content.WriteString("\n")
				content.WriteString(a.styles.URL.Render("  " + urlLine))
				content.WriteString("\n")
			} else {
				content.WriteString("  " + titleLine)
				content.WriteString("\n")
				content.WriteString(a.styles.Empty.Render("  " + urlLine))
				content.WriteString("\n")
			}
		}
	}

	modal := lipgloss.Place(
		a.width,
		a.height-3,
		lipgloss.Center,
		lipgloss.Center,
		modalStyle.Render(content.String()),
	)

	return lipgloss.JoinVertical(lipgloss.Left, modal, a.renderHelpBar())
}

// renderOrganizeLoading renders the loading screen during AI analysis.
func (a App) renderOrganizeLoading() string {
	modalWidth := layout.CalculateModalWidth(a.width, a.layoutConfig.Modal.DefaultWidthPercent, a.layoutConfig.Modal)

	accent := lipgloss.AdaptiveColor{Light: "#6B8E23", Dark: "#9ACD32"}
	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.ThickBorder()).
		BorderForeground(accent).
		Padding(1, 2).
		Width(modalWidth)

	var content strings.Builder
	content.WriteString(a.styles.Title.Render("Organize"))
	content.WriteString("\n\n")
	content.WriteString(fmt.Sprintf("Analyzing %d items...\n\n", a.organize.Total))

	// Progress bar
	if a.organize.Total > 0 {
		progress := float64(a.organize.Progress) / float64(a.organize.Total)
		barWidth := modalWidth - 10
		filled := int(progress * float64(barWidth))
		empty := barWidth - filled

		bar := strings.Repeat("█", filled) + strings.Repeat("░", empty)
		content.WriteString(bar + "\n\n")
		content.WriteString(fmt.Sprintf("[%d/%d]", a.organize.Progress, a.organize.Total))
	}

	modal := lipgloss.Place(
		a.width,
		a.height-3,
		lipgloss.Center,
		lipgloss.Center,
		modalStyle.Render(content.String()),
	)

	return lipgloss.JoinVertical(lipgloss.Left, modal, a.renderHelpBar())
}

// renderOrganizeResults renders the list of suggested organization changes.
func (a App) renderOrganizeResults() string {
	modalWidth := layout.CalculateModalWidth(a.width, a.layoutConfig.Modal.LargeWidthPercent, a.layoutConfig.Modal)

	accent := lipgloss.AdaptiveColor{Light: "#6B8E23", Dark: "#9ACD32"}
	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.ThickBorder()).
		BorderForeground(accent).
		Padding(1, 2).
		Width(modalWidth)

	var content strings.Builder

	unprocessed := a.organize.UnprocessedCount()
	title := fmt.Sprintf("Organize Results (%d to review)", unprocessed)
	content.WriteString(a.styles.Title.Render(title))
	content.WriteString("\n\n")

	if len(a.organize.Suggestions) == 0 {
		content.WriteString(a.styles.Empty.Render("All items already well-organized!"))
	} else {
		// Filter to unprocessed suggestions for display
		var visibleSuggestions []int
		for i, sug := range a.organize.Suggestions {
			if !sug.Processed {
				visibleSuggestions = append(visibleSuggestions, i)
			}
		}

		maxVisible := 6 // Reduced to accommodate tag lines
		itemWidth := modalWidth - 6

		// Find cursor position in visible list
		cursorVisibleIdx := 0
		for i, idx := range visibleSuggestions {
			if idx == a.organize.Cursor {
				cursorVisibleIdx = i
				break
			}
		}

		start, end := layout.CalculateVisibleListItems(maxVisible, cursorVisibleIdx, len(visibleSuggestions))

		for vi := start; vi < end; vi++ {
			idx := visibleSuggestions[vi]
			sug := a.organize.Suggestions[idx]
			isSelected := idx == a.organize.Cursor

			// Title line
			titleLine := sug.Item.Title()
			if len(titleLine) > itemWidth-2 {
				titleLine = titleLine[:itemWidth-5] + "..."
			}

			// Path line (only if folder changes)
			var pathLine string
			if sug.HasFolderChanges() {
				pathLine = sug.CurrentPath + " → " + sug.SuggestedPath
				if sug.IsNewFolder {
					pathLine += " (new)"
				}
				if len(pathLine) > itemWidth-4 {
					pathLine = pathLine[:itemWidth-7] + "..."
				}
			}

			// Tags line (only if tags change and not a folder)
			var tagsLine string
			if !sug.Item.IsFolder() && sug.HasTagChanges() {
				currentTagsStr := strings.Join(sug.CurrentTags, ", ")
				if currentTagsStr == "" {
					currentTagsStr = "(none)"
				}
				suggestedTagsStr := strings.Join(sug.SuggestedTags, ", ")
				if suggestedTagsStr == "" {
					suggestedTagsStr = "(none)"
				}
				tagsLine = "Tags: " + currentTagsStr + " → " + suggestedTagsStr
				if len(tagsLine) > itemWidth-4 {
					tagsLine = tagsLine[:itemWidth-7] + "..."
				}
			}

			if isSelected {
				content.WriteString(a.styles.ItemSelected.Render("> " + titleLine))
				content.WriteString("\n")
				if pathLine != "" {
					content.WriteString(a.styles.URL.Render("  " + pathLine))
					content.WriteString("\n")
				}
				if tagsLine != "" {
					content.WriteString(a.styles.Tag.Render("  " + tagsLine))
					content.WriteString("\n")
				}
				content.WriteString("\n")
			} else {
				content.WriteString("  " + titleLine)
				content.WriteString("\n")
				if pathLine != "" {
					content.WriteString(a.styles.Empty.Render("  " + pathLine))
					content.WriteString("\n")
				}
				if tagsLine != "" {
					content.WriteString(a.styles.Empty.Render("  " + tagsLine))
					content.WriteString("\n")
				}
				content.WriteString("\n")
			}
		}
	}

	modal := lipgloss.Place(
		a.width,
		a.height-3,
		lipgloss.Center,
		lipgloss.Center,
		modalStyle.Render(content.String()),
	)

	return lipgloss.JoinVertical(lipgloss.Left, modal, a.renderHelpBar())
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

// saveStore persists the current store to storage (if storage is configured).
// This should be called after any mutation to the store.
func (a *App) saveStore() {
	if a.storage == nil {
		return
	}
	if err := a.storage.Save(a.store); err != nil {
		a.setMessage(MessageError, "Save failed: "+err.Error())
	}
}
