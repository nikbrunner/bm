package tui

import "strings"

// Hint represents a single keybind hint for display.
type Hint struct {
	Key  string // Display key (e.g., "j/k", "Enter")
	Desc string // Short description (e.g., "move", "open")
}

// renderHint renders a single hint as "key:desc" with styling.
func (a App) renderHint(h Hint) string {
	return a.styles.HintKey.Render(h.Key) + ":" + a.styles.HintDesc.Render(h.Desc)
}

// renderHints renders hints in horizontal format for bottom bar: "j/k:move h:back l:open"
func (a App) renderHints(hints HintSet) string {
	allHints := hints.All()
	if len(allHints) == 0 {
		return ""
	}

	parts := make([]string, len(allHints))
	for i, h := range allHints {
		parts[i] = a.renderHint(h)
	}
	return strings.Join(parts, " ")
}

// renderHintsInline renders hints in inline format for modals: "Enter confirm  Esc cancel"
func (a App) renderHintsInline(hints []Hint) string {
	if len(hints) == 0 {
		return ""
	}

	parts := make([]string, len(hints))
	for i, h := range hints {
		parts[i] = a.styles.HintKey.Render(h.Key) + " " + a.styles.HintDesc.Render(h.Desc)
	}
	return strings.Join(parts, "  ")
}

// HintSet is an ordered collection of hints by group.
type HintSet struct {
	Nav    []Hint // Navigation hints (j/k, h/l, etc.)
	Edit   []Hint // Edit hints (a, e, d, etc.)
	Action []Hint // Action hints (Enter, Tab, etc.)
	System []Hint // System hints (?, q, Esc)
}

// All returns all hints flattened in display order: Nav + Action + Edit + System.
func (h HintSet) All() []Hint {
	result := make([]Hint, 0, len(h.Nav)+len(h.Action)+len(h.Edit)+len(h.System))
	result = append(result, h.Nav...)
	result = append(result, h.Action...)
	result = append(result, h.Edit...)
	result = append(result, h.System...)
	return result
}

// getContextualHints returns the appropriate hints for the current mode.
func (a App) getContextualHints() HintSet {
	switch a.mode {
	case ModeNormal:
		return a.getNormalModeHints()
	case ModeFilter:
		return a.getFilterModeHints()
	case ModeSearch:
		return a.getSearchModeHints()
	case ModeAddBookmark, ModeEditBookmark:
		return a.getBookmarkFormHints()
	case ModeAddFolder, ModeEditFolder:
		return a.getFolderFormHints()
	case ModeEditTags:
		return a.getTagsFormHints()
	case ModeConfirmDelete:
		return a.getConfirmDeleteHints()
	case ModeMove:
		return a.getMoveHints()
	case ModeQuickAdd:
		return a.getQuickAddHints()
	case ModeQuickAddLoading:
		return a.getQuickAddLoadingHints()
	case ModeQuickAddConfirm:
		return a.getQuickAddConfirmHints()
	case ModeQuickAddCreateFolder:
		return a.getQuickAddCreateFolderHints()
	case ModeReadLaterLoading:
		return a.getReadLaterLoadingHints()
	case ModeHelp:
		// Help overlay covers screen, minimal hints
		return HintSet{
			System: []Hint{{Key: "?/q/Esc", Desc: "close"}},
		}
	default:
		return HintSet{}
	}
}

// getNormalModeHints returns hints for ModeNormal (main browse).
func (a App) getNormalModeHints() HintSet {
	return HintSet{
		Nav: []Hint{
			{Key: "j/k", Desc: "move"},
			{Key: "h", Desc: "back"},
			{Key: "l", Desc: "open"},
		},
		Action: []Hint{
			{Key: "s", Desc: "search"},
			{Key: "/", Desc: "filter"},
		},
		Edit: []Hint{
			{Key: "a", Desc: "add"},
			{Key: "e", Desc: "edit"},
			{Key: "d", Desc: "del"},
		},
		System: []Hint{
			{Key: "?", Desc: "help"},
			{Key: "q", Desc: "quit"},
		},
	}
}

// getFilterModeHints returns hints for ModeFilter (local filter active).
func (a App) getFilterModeHints() HintSet {
	return HintSet{
		Nav: []Hint{
			{Key: "type", Desc: "filter"},
		},
		Action: []Hint{
			{Key: "Enter", Desc: "apply"},
		},
		System: []Hint{
			{Key: "Esc", Desc: "cancel"},
		},
	}
}

// getSearchModeHints returns hints for ModeSearch (fuzzy finder).
func (a App) getSearchModeHints() HintSet {
	return HintSet{
		Nav: []Hint{
			{Key: "j/k", Desc: "move"},
		},
		Action: []Hint{
			{Key: "Enter", Desc: "open"},
		},
		System: []Hint{
			{Key: "Esc", Desc: "cancel"},
		},
	}
}

// getBookmarkFormHints returns hints for ModeAddBookmark/ModeEditBookmark.
func (a App) getBookmarkFormHints() HintSet {
	return HintSet{
		Nav: []Hint{
			{Key: "Tab", Desc: "next"},
		},
		Action: []Hint{
			{Key: "Enter", Desc: "save"},
		},
		System: []Hint{
			{Key: "Esc", Desc: "cancel"},
		},
	}
}

// getFolderFormHints returns hints for ModeAddFolder/ModeEditFolder.
func (a App) getFolderFormHints() HintSet {
	return HintSet{
		Action: []Hint{
			{Key: "Enter", Desc: "save"},
		},
		System: []Hint{
			{Key: "Esc", Desc: "cancel"},
		},
	}
}

// getTagsFormHints returns hints for ModeEditTags.
func (a App) getTagsFormHints() HintSet {
	hints := HintSet{
		Action: []Hint{
			{Key: "Enter", Desc: "save"},
		},
		System: []Hint{
			{Key: "Esc", Desc: "cancel"},
		},
	}
	// Add suggestion hints if suggestions are available
	if len(a.modal.TagSuggestions) > 0 {
		hints.Nav = []Hint{
			{Key: "↑/↓", Desc: "suggest"},
			{Key: "Tab", Desc: "accept"},
		}
	}
	return hints
}

// getConfirmDeleteHints returns hints for ModeConfirmDelete.
// Returns empty - hints are shown inside the modal itself.
func (a App) getConfirmDeleteHints() HintSet {
	return HintSet{}
}

// getMoveHints returns hints for ModeMove.
func (a App) getMoveHints() HintSet {
	return HintSet{
		Nav: []Hint{
			{Key: "↑/↓", Desc: "nav"},
			{Key: "type", Desc: "filter"},
		},
		Action: []Hint{
			{Key: "Enter", Desc: "move"},
		},
		System: []Hint{
			{Key: "Esc", Desc: "cancel"},
		},
	}
}

// getQuickAddHints returns hints for ModeQuickAdd (URL input).
func (a App) getQuickAddHints() HintSet {
	return HintSet{
		Action: []Hint{
			{Key: "Enter", Desc: "analyze"},
		},
		System: []Hint{
			{Key: "Esc", Desc: "cancel"},
		},
	}
}

// getQuickAddLoadingHints returns hints for ModeQuickAddLoading.
func (a App) getQuickAddLoadingHints() HintSet {
	return HintSet{
		System: []Hint{
			{Key: "Esc", Desc: "cancel"},
		},
	}
}

// getQuickAddConfirmHints returns hints for ModeQuickAddConfirm.
func (a App) getQuickAddConfirmHints() HintSet {
	return HintSet{
		Nav: []Hint{
			{Key: "Tab", Desc: "next"},
			{Key: "↑/↓", Desc: "folder"},
			{Key: "type", Desc: "search"},
		},
		Action: []Hint{
			{Key: "Enter", Desc: "save"},
		},
		System: []Hint{
			{Key: "Esc", Desc: "cancel"},
		},
	}
}

// getQuickAddCreateFolderHints returns hints for ModeQuickAddCreateFolder.
func (a App) getQuickAddCreateFolderHints() HintSet {
	return HintSet{
		Nav: []Hint{
			{Key: "j/k", Desc: "select"},
		},
		Action: []Hint{
			{Key: "Enter", Desc: "create"},
		},
		System: []Hint{
			{Key: "Esc", Desc: "cancel"},
		},
	}
}

// getReadLaterLoadingHints returns hints for ModeReadLaterLoading.
func (a App) getReadLaterLoadingHints() HintSet {
	return HintSet{
		System: []Hint{
			{Key: "", Desc: "adding..."},
		},
	}
}
