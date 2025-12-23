package tui

import (
	"strconv"
	"strings"
)

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

// getGlobalHints returns hints for keys that work in any pane/mode.
func (a App) getGlobalHints() []Hint {
	return []Hint{
		{Key: "s", Desc: "search"},
		{Key: "R", Desc: "recent"},
		{Key: "/", Desc: "filter"},
		{Key: "i", Desc: "AI add"},
		{Key: "L", Desc: "read later"},
		{Key: "a/A", Desc: "add"},
		{Key: "C", Desc: "cull"},
		{Key: "?", Desc: "help"},
		{Key: "q", Desc: "quit"},
	}
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
	case ModeCullMenu:
		return a.getCullMenuHints()
	case ModeCullLoading:
		return a.getCullLoadingHints()
	case ModeCullResults:
		return a.getCullResultsHints()
	case ModeCullInspect:
		return a.getCullInspectHints()
	case ModeOrganizeMenu:
		return a.getOrganizeMenuHints()
	case ModeOrganizeLoading:
		return a.getOrganizeLoadingHints()
	case ModeOrganizeResults:
		return a.getOrganizeResultsHints()
	default:
		return HintSet{}
	}
}

// getNormalModeHints returns hints for ModeNormal (main browse).
// Only shows local/pane-specific hints - global hints are shown separately.
func (a App) getNormalModeHints() HintSet {
	hints := HintSet{
		Nav: []Hint{
			{Key: "j/k", Desc: "move"},
			{Key: "h", Desc: "back"},
			{Key: "l", Desc: "enter"},
		},
		Edit: []Hint{
			{Key: "y", Desc: "yank"},
			{Key: "d", Desc: "del"},
			{Key: "x", Desc: "cut"},
			{Key: "p", Desc: "paste"},
			{Key: "e", Desc: "edit"},
		},
		Action: []Hint{
			{Key: "o", Desc: "open"},
			{Key: "*", Desc: "pin"},
			{Key: "m", Desc: "move"},
			{Key: "O", Desc: "organize"},
			{Key: "v/V", Desc: "select"},
		},
	}

	// Show selection hints when items are selected
	if a.selection.HasSelection() {
		count := a.selection.Count()
		hints.Action = []Hint{
			{Key: "v", Desc: "±sel"},
			{Key: "Esc", Desc: "clear"},
			{Key: "d", Desc: "del " + strconv.Itoa(count)},
		}
	}

	return hints
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
			{Key: "Enter", Desc: "go to"},
			{Key: "^o", Desc: "open"},
			{Key: "^e", Desc: "edit"},
			{Key: "^y", Desc: "yank"},
			{Key: "^d", Desc: "del"},
		},
		System: []Hint{
			{Key: "Esc", Desc: "back"},
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

// getCullMenuHints returns hints for ModeCullMenu.
func (a App) getCullMenuHints() HintSet {
	return HintSet{
		Nav: []Hint{
			{Key: "j/k", Desc: "navigate"},
		},
		Action: []Hint{
			{Key: "Enter", Desc: "select"},
		},
		System: []Hint{
			{Key: "Esc", Desc: "cancel"},
		},
	}
}

// getCullLoadingHints returns hints for ModeCullLoading.
func (a App) getCullLoadingHints() HintSet {
	return HintSet{
		System: []Hint{
			{Key: "Esc", Desc: "cancel"},
		},
	}
}

// getCullResultsHints returns hints for ModeCullResults.
func (a App) getCullResultsHints() HintSet {
	return HintSet{
		Nav: []Hint{
			{Key: "j/k", Desc: "move"},
		},
		Action: []Hint{
			{Key: "Enter", Desc: "inspect"},
			{Key: "d", Desc: "del all"},
		},
		System: []Hint{
			{Key: "Esc", Desc: "back"},
		},
	}
}

// getCullInspectHints returns hints for ModeCullInspect.
func (a App) getCullInspectHints() HintSet {
	return HintSet{
		Nav: []Hint{
			{Key: "j/k", Desc: "move"},
		},
		Action: []Hint{
			{Key: "d", Desc: "del"},
			{Key: "o", Desc: "open"},
			{Key: "e", Desc: "edit"},
			{Key: "m", Desc: "move"},
		},
		System: []Hint{
			{Key: "Esc", Desc: "back"},
		},
	}
}

// getOrganizeMenuHints returns hints for ModeOrganizeMenu.
func (a App) getOrganizeMenuHints() HintSet {
	return HintSet{
		Nav: []Hint{
			{Key: "j/k", Desc: "move"},
		},
		Action: []Hint{
			{Key: "Enter", Desc: "select"},
		},
		System: []Hint{
			{Key: "Esc", Desc: "back"},
		},
	}
}

// getOrganizeLoadingHints returns hints for ModeOrganizeLoading.
func (a App) getOrganizeLoadingHints() HintSet {
	return HintSet{
		System: []Hint{
			{Key: "Esc", Desc: "cancel"},
		},
	}
}

// getOrganizeResultsHints returns hints for ModeOrganizeResults.
func (a App) getOrganizeResultsHints() HintSet {
	return HintSet{
		Nav: []Hint{
			{Key: "j/k", Desc: "move"},
		},
		Action: []Hint{
			{Key: "Enter", Desc: "accept"},
			{Key: "s", Desc: "skip"},
			{Key: "o", Desc: "open"},
			{Key: "m", Desc: "move"},
			{Key: "d", Desc: "del"},
		},
		System: []Hint{
			{Key: "Esc", Desc: "back"},
		},
	}
}
