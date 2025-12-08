package layout

// FuzzyLayout holds calculated fuzzy finder dimensions.
type FuzzyLayout struct {
	ListWidth    int
	PreviewWidth int
	ListHeight   int
}

// CalculateModalWidth computes responsive modal width based on percentage of terminal width.
// Uses widthPercent of terminal width, clamped between MinWidth and MaxWidth.
func CalculateModalWidth(terminalWidth, widthPercent int, cfg ModalConfig) int {
	width := terminalWidth * widthPercent / 100

	// Apply min/max constraints
	if width < cfg.MinWidth {
		width = cfg.MinWidth
	}
	if width > cfg.MaxWidth {
		width = cfg.MaxWidth
	}

	// Don't exceed terminal width
	if width > terminalWidth-4 {
		width = terminalWidth - 4
	}
	if width < 1 {
		return 1
	}

	return width
}

// CalculateFuzzyLayout computes the fuzzy finder pane dimensions.
func CalculateFuzzyLayout(terminalWidth, terminalHeight int, cfg FuzzyConfig) FuzzyLayout {
	listWidth := terminalWidth * cfg.ListWidthPercent / 100
	previewWidth := terminalWidth * cfg.PreviewWidthPercent / 100
	listHeight := terminalHeight - cfg.HeaderReduction

	if listHeight < 1 {
		listHeight = 1
	}

	return FuzzyLayout{
		ListWidth:    listWidth,
		PreviewWidth: previewWidth,
		ListHeight:   listHeight,
	}
}

// CalculateVisibleListItems computes the start and end indices for a scrollable list.
// Returns (start, end) where items[start:end] should be displayed.
func CalculateVisibleListItems(maxVisible, selectedIdx, totalItems int) (start, end int) {
	if totalItems <= maxVisible {
		return 0, totalItems
	}

	if selectedIdx >= maxVisible {
		start = selectedIdx - maxVisible + 1
	}

	end = start + maxVisible
	if end > totalItems {
		end = totalItems
	}

	return start, end
}
