package tui

import (
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/nikbrunner/bm/internal/ai"
	"github.com/nikbrunner/bm/internal/tui/layout"
)

// QuickAddState holds state for the AI-powered quick add feature.
type QuickAddState struct {
	Input     textinput.Model // URL input
	Response  *ai.Response    // AI suggestion
	Error     error           // AI error (if any)
	Folders   []string        // Available folder paths for picker
	FolderIdx int             // Selected folder index in picker
}

// NewQuickAddState creates a new QuickAddState with initialized input.
func NewQuickAddState(cfg layout.LayoutConfig) QuickAddState {
	input := textinput.New()
	input.Placeholder = "https://..."
	input.CharLimit = cfg.Input.URLCharLimit
	input.Width = cfg.Input.QuickAddWidth
	return QuickAddState{
		Input: input,
	}
}

// Reset clears the quick add state for a new session.
func (q *QuickAddState) Reset() {
	q.Input.Reset()
	q.Response = nil
	q.Error = nil
	q.Folders = nil
	q.FolderIdx = 0
}

// MoveState holds state for moving items to different folders.
type MoveState struct {
	FilterInput     textinput.Model // Filter input for folder search
	Folders         []string        // All folder paths
	FilteredFolders []string        // Filtered folder paths based on search
	FolderIdx       int             // Selected folder index in filtered list
}

// NewMoveState creates a new MoveState with initialized input.
func NewMoveState(cfg layout.LayoutConfig) MoveState {
	input := textinput.New()
	input.Placeholder = "Filter folders..."
	input.CharLimit = cfg.Input.TitleCharLimit
	input.Width = cfg.Input.StandardWidth
	return MoveState{
		FilterInput: input,
	}
}

// Reset clears the move state for a new session.
func (m *MoveState) Reset() {
	m.FilterInput.Reset()
	m.Folders = nil
	m.FilteredFolders = nil
	m.FolderIdx = 0
}

// SearchState holds state for global search and local filtering.
type SearchState struct {
	// Global search
	Input        textinput.Model // Search input
	FuzzyMatches []fuzzyMatch    // Current fuzzy match results
	FuzzyCursor  int             // Selected index in fuzzy results
	AllItems     []Item          // All items for global search

	// Local filter
	FilterInput   textinput.Model // Filter input for current folder
	FilterQuery   string          // Active filter query (persists after closing filter)
	FilteredItems []Item          // Items matching filter in current folder
}

// NewSearchState creates a new SearchState with initialized inputs.
func NewSearchState(cfg layout.LayoutConfig) SearchState {
	searchInput := textinput.New()
	searchInput.Placeholder = "Search all..."
	searchInput.CharLimit = cfg.Input.SearchCharLimit
	searchInput.Width = cfg.Input.StandardWidth

	filterInput := textinput.New()
	filterInput.Placeholder = "Filter..."
	filterInput.CharLimit = cfg.Input.FilterCharLimit
	filterInput.Width = cfg.Input.FilterWidth

	return SearchState{
		Input:       searchInput,
		FilterInput: filterInput,
	}
}

// ResetGlobalSearch clears the global search state.
func (s *SearchState) ResetGlobalSearch() {
	s.Input.Reset()
	s.FuzzyMatches = nil
	s.AllItems = nil
	s.FuzzyCursor = 0
}

// ResetFilter clears the local filter state.
func (s *SearchState) ResetFilter() {
	s.FilterInput.Reset()
	s.FilterQuery = ""
	s.FilteredItems = nil
}

// ModalState holds state for edit/add modals (bookmark/folder).
type ModalState struct {
	TitleInput textinput.Model // Title input for folders/bookmarks
	URLInput   textinput.Model // URL input for bookmarks
	TagsInput  textinput.Model // Tags input for bookmarks
	EditItemID string          // ID of item being edited (folder or bookmark)
	CutMode    bool            // true = cut (buffer), false = delete (no buffer)

	// Tag autocompletion
	AllTags          []string // All unique tags in store
	TagSuggestions   []string // Filtered suggestions for current input
	TagSuggestionIdx int      // Selected suggestion index (-1 = none)
}

// NewModalState creates a new ModalState with initialized inputs.
func NewModalState(cfg layout.LayoutConfig) ModalState {
	titleInput := textinput.New()
	titleInput.Placeholder = "Title"
	titleInput.CharLimit = cfg.Input.TitleCharLimit
	titleInput.Width = cfg.Input.StandardWidth

	urlInput := textinput.New()
	urlInput.Placeholder = "URL"
	urlInput.CharLimit = cfg.Input.URLCharLimit
	urlInput.Width = cfg.Input.StandardWidth

	tagsInput := textinput.New()
	tagsInput.Placeholder = "tag1, tag2, tag3"
	tagsInput.CharLimit = cfg.Input.TagsCharLimit
	tagsInput.Width = cfg.Input.StandardWidth

	return ModalState{
		TitleInput:       titleInput,
		URLInput:         urlInput,
		TagsInput:        tagsInput,
		TagSuggestionIdx: -1,
	}
}

// ResetInputs clears all modal inputs for a new modal session.
func (m *ModalState) ResetInputs() {
	m.TitleInput.Reset()
	m.URLInput.Reset()
	m.TagsInput.Reset()
	m.EditItemID = ""
	m.CutMode = false
	m.TagSuggestions = nil
	m.TagSuggestionIdx = -1
}

// BrowserNav holds state for folder navigation in the miller columns.
type BrowserNav struct {
	CurrentFolderID *string  // nil = root
	FolderStack     []string // breadcrumb trail of folder IDs
	Cursor          int      // selected item index
	Items           []Item   // current list items
	SortMode        SortMode // current sort mode
}

// NewBrowserNav creates a new BrowserNav at root.
func NewBrowserNav() BrowserNav {
	return BrowserNav{
		CurrentFolderID: nil,
		FolderStack:     []string{},
		Cursor:          0,
	}
}

// AtRoot returns true if currently at root folder.
func (b *BrowserNav) AtRoot() bool {
	return b.CurrentFolderID == nil
}

// ResetToRoot resets navigation to root folder.
func (b *BrowserNav) ResetToRoot() {
	b.CurrentFolderID = nil
	b.FolderStack = []string{}
	b.Cursor = 0
}
