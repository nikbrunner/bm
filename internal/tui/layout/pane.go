package layout

// PaneLayout holds calculated pane dimensions.
type PaneLayout struct {
	Width int
	Count int // 3 or 4 panes
}

// CalculatePaneHeight computes the content height for panes.
// Returns at least MinHeight.
func CalculatePaneHeight(terminalHeight int, cfg PaneConfig) int {
	height := terminalHeight - cfg.HeightReduction
	if height < cfg.MinHeight {
		return cfg.MinHeight
	}
	return height
}

// CalculatePaneWidth computes width for each pane based on layout.
// hasPinnedItems: whether pinned pane is shown
// atRoot: whether currently at root folder
func CalculatePaneWidth(terminalWidth int, hasPinnedItems, atRoot bool, cfg PaneConfig) PaneLayout {
	var paneCount int
	var offset int
	var minWidth int

	if hasPinnedItems && !atRoot {
		// 4-pane layout: pinned | parent | current | preview
		paneCount = 4
		offset = cfg.FourPaneWidthOffset
		minWidth = cfg.MinFourPaneWidth
	} else {
		// 3-pane layout: (pinned or parent) | current | preview
		paneCount = 3
		offset = cfg.ThreePaneWidthOffset
		minWidth = cfg.MinThreePaneWidth
	}

	width := (terminalWidth - offset) / paneCount
	if width < minWidth {
		width = minWidth
	}

	return PaneLayout{
		Width: width,
		Count: paneCount,
	}
}

// CalculateItemWidth computes the width available for item content.
func CalculateItemWidth(paneWidth int, cfg PaneConfig) int {
	return paneWidth - cfg.ContentPadding
}

// CalculateVisibleHeight computes the visible item count in a pane.
func CalculateVisibleHeight(paneHeight, headerLines int) int {
	height := paneHeight - headerLines
	if height < 1 {
		return 1
	}
	return height
}

// CalculateViewportOffset calculates the scroll offset needed to keep the
// selected item visible within the viewport.
func CalculateViewportOffset(selected, total, viewportHeight int) int {
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
