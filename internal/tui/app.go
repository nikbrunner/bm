package tui

import (
	"encoding/json"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/nikbrunner/bm/internal/ai"
	"github.com/nikbrunner/bm/internal/culler"
	"github.com/nikbrunner/bm/internal/model"
	"github.com/nikbrunner/bm/internal/storage"
	"github.com/nikbrunner/bm/internal/tui/layout"
	"github.com/sahilm/fuzzy"
)

// Package-level atomic counter for cull progress (bubbletea models are immutable)
var cullProgressCounter int64
var organizeProgressCounter int64

// CullCache represents the cached cull results for disk persistence.
type CullCache struct {
	Timestamp time.Time         `json:"timestamp"`
	Results   []CullCacheResult `json:"results"`
}

// CullCacheResult is a serializable version of culler.Result.
type CullCacheResult struct {
	BookmarkID string `json:"bookmarkId"`
	Status     int    `json:"status"` // culler.Status as int
	StatusCode int    `json:"statusCode"`
	Error      string `json:"error"`
}

// OrganizeCache represents the cached organize results for disk persistence.
type OrganizeCache struct {
	Timestamp time.Time              `json:"timestamp"`
	Results   []OrganizeCacheResult  `json:"results"`
}

// OrganizeCacheResult is a serializable version of OrganizeSuggestion.
type OrganizeCacheResult struct {
	ItemID        string   `json:"itemId"`
	IsFolder      bool     `json:"isFolder"`
	CurrentPath   string   `json:"currentPath"`
	SuggestedPath string   `json:"suggestedPath"`
	IsNewFolder   bool     `json:"isNewFolder"`
	CurrentTags   []string `json:"currentTags"`
	SuggestedTags []string `json:"suggestedTags"`
}

// aiResponseMsg is sent when the AI API call completes.
type aiResponseMsg struct {
	response *ai.Response
	err      error
}

// organizeResponseMsg is sent when an AI organize suggestion completes.
type organizeResponseMsg struct {
	item     Item
	response *ai.OrganizeResponse
	err      error
}

// organizeCompleteMsg is sent when all items have been analyzed.
type organizeCompleteMsg struct{}

// organizeTickMsg is sent periodically to update progress display.
type organizeTickMsg struct{}

// organizeResultsMsg carries the final suggestions.
type organizeResultsMsg struct {
	suggestions []OrganizeSuggestion
}

// Mode represents the current UI mode.
type Mode int

const (
	ModeNormal Mode = iota
	ModeAddBookmark
	ModeAddFolder
	ModeEditFolder
	ModeEditBookmark
	ModeEditTags
	ModeConfirmDelete
	ModeSearch
	ModeFilter // Local filter for current folder
	ModeHelp
	ModeQuickAdd             // URL input for AI quick add
	ModeQuickAddLoading      // Waiting for AI response
	ModeQuickAddConfirm      // Review/edit AI suggestion
	ModeQuickAddCreateFolder // Create new folder during quick add
	ModeReadLaterLoading     // Waiting for AI response for read later
	ModeMove                 // Move item to different folder
	ModeCullMenu             // Menu to choose fresh vs cached cull
	ModeCullLoading          // Checking URLs, show progress
	ModeCullResults          // Group list view for cull results
	ModeCullInspect          // Bookmark list within a cull group
	ModeOrganizeMenu         // Menu to choose fresh vs cached organize
	ModeOrganizeLoading      // Analyzing items for organize suggestions
	ModeOrganizeResults      // List of suggested organization changes
)

// hasTextInput returns true if the mode has an active text input where 'q' shouldn't quit.
func (m Mode) hasTextInput() bool {
	switch m {
	case ModeAddBookmark, ModeAddFolder, ModeEditFolder, ModeEditBookmark, ModeEditTags,
		ModeSearch, ModeFilter, ModeQuickAdd, ModeQuickAddConfirm, ModeMove:
		return true
	}
	return false
}

// SortMode represents the current sort mode.
type SortMode int

const (
	SortManual  SortMode = iota // preserve insertion order
	SortAlpha                   // alphabetical
	SortCreated                 // by creation date (newest first)
	SortVisited                 // by visit date (most recent first)
)

// FocusedPane represents which pane has focus.
type FocusedPane int

const (
	PanePinned  FocusedPane = iota // leftmost pane with pinned items
	PaneBrowser                    // Miller column browser pane
)

// MessageType represents the type of status message.
type MessageType int

const (
	MessageNone    MessageType = iota // no message
	MessageInfo                       // informational (dim)
	MessageSuccess                    // success (green)
	MessageWarning                    // warning (yellow)
	MessageError                      // error (red)
)

// messageClearMsg is sent when the message timeout expires.
type messageClearMsg struct{}

// openURLErrorMsg is sent when opening a URL fails.
type openURLErrorMsg struct {
	err error
}

// clipboardErrorMsg is sent when clipboard operation fails.
type clipboardErrorMsg struct {
	err error
}

// cullProgressMsg is sent periodically during URL checking.
type cullProgressMsg struct {
	completed int
	total     int
}

// cullCompleteMsg is sent when URL checking is complete.
type cullCompleteMsg struct {
	results []culler.Result
}

// cullTickMsg is sent periodically to update the progress display.
type cullTickMsg struct{}

// messageDuration is how long messages are displayed before auto-clearing.
const messageDuration = 3 * time.Second

// fuzzyMatch represents a fuzzy search match with highlighting info.
type fuzzyMatch struct {
	Item           Item
	MatchedIndexes []int
	Score          int
}

// itemStrings implements fuzzy.Source for Item slice.
type itemStrings []Item

func (is itemStrings) String(i int) string {
	return is[i].Title()
}

func (is itemStrings) Len() int {
	return len(is)
}

// App is the main bubbletea model for the bookmark manager.
type App struct {
	store        *model.Store
	storage      storage.Storage // for auto-saving after mutations
	config       *storage.Config // app settings (quick add folder, etc.)
	keys         KeyMap
	styles       Styles
	layoutConfig layout.LayoutConfig

	// Focus state
	focusedPane  FocusedPane // which pane has focus
	pinnedCursor int         // cursor in pinned pane
	pinnedItems  []Item      // cached pinned items (folders first, then bookmarks)

	// Browser navigation state
	browser BrowserNav

	// Global search (s key) and local filter (/ key)
	search SearchState

	// For gg command
	lastKeyWasG bool

	// Yank buffer (supports batch yank)
	yankedItems []Item

	// UI mode and modal state
	mode  Mode
	modal ModalState

	// Quick add (AI-powered) state
	quickAdd             QuickAddState
	quickAddCreateFolder QuickAddCreateFolderState

	// Read later state (URL being processed)
	readLaterURL string

	// Move state
	move MoveState

	// Selection state (visual mode)
	selection SelectionState

	// Cull state
	cull CullState

	// Organize state
	organize OrganizeState

	// Settings
	confirmDelete bool // true = ask confirmation before delete (default true)

	// Message display (for user feedback)
	messageType MessageType // type determines styling
	messageText string      // the message content

	// Window dimensions
	width  int
	height int
}

// AppParams holds parameters for creating a new App.
type AppParams struct {
	Store        *model.Store
	Storage      storage.Storage      // optional, for auto-saving after mutations
	Config       *storage.Config      // optional, uses default if nil
	Keys         *KeyMap              // optional, uses default if nil
	Styles       *Styles              // optional, uses default if nil
	LayoutConfig *layout.LayoutConfig // optional, uses default if nil
}

// NewApp creates a new App with the given parameters.
func NewApp(params AppParams) App {
	cfg := storage.DefaultConfig()
	if params.Config != nil {
		cfg = *params.Config
	}

	keys := DefaultKeyMap()
	if params.Keys != nil {
		keys = *params.Keys
	}

	styles := DefaultStyles()
	if params.Styles != nil {
		styles = *params.Styles
	}

	layoutCfg := layout.DefaultConfig()
	if params.LayoutConfig != nil {
		layoutCfg = *params.LayoutConfig
	}

	app := App{
		store:         params.Store,
		storage:       params.Storage,
		config:        &cfg,
		keys:          keys,
		styles:        styles,
		layoutConfig:  layoutCfg,
		focusedPane:   PaneBrowser, // will be updated after refreshPinnedItems
		pinnedCursor:  0,
		browser:       NewBrowserNav(),
		mode:          ModeNormal,
		modal:         NewModalState(layoutCfg),
		search:        NewSearchState(layoutCfg),
		quickAdd:      NewQuickAddState(layoutCfg),
		move:          NewMoveState(layoutCfg),
		selection:     NewSelectionState(),
		cull:          NewCullState(),
		organize:      NewOrganizeState(),
		confirmDelete: true,
		width:         80,
		height:        24,
	}

	app.refreshItems()
	app.refreshPinnedItems()

	// Start focused on pinned pane if there are pinned items
	if len(app.pinnedItems) > 0 {
		app.focusedPane = PanePinned
	}

	return app
}

// refreshItems rebuilds the items slice based on current folder and sort mode.
func (a *App) refreshItems() {
	a.browser.Items = []Item{}
	// Clear filter when refreshing (folder changed)
	a.search.FilterQuery = ""
	a.search.FilteredItems = nil
	// Clear selection when folder changes
	a.selection.Reset()

	// Get folders and bookmarks
	folders := a.store.GetFoldersInFolder(a.browser.CurrentFolderID)
	bookmarks := a.store.GetBookmarksInFolder(a.browser.CurrentFolderID)

	// Apply sorting based on current mode
	switch a.browser.SortMode {
	case SortAlpha:
		// Sort folders alphabetically
		sort.Slice(folders, func(i, j int) bool {
			return strings.ToLower(folders[i].Name) < strings.ToLower(folders[j].Name)
		})
		// Sort bookmarks alphabetically
		sort.Slice(bookmarks, func(i, j int) bool {
			return strings.ToLower(bookmarks[i].Title) < strings.ToLower(bookmarks[j].Title)
		})

	case SortCreated:
		// Sort bookmarks by created date (newest first)
		sort.Slice(bookmarks, func(i, j int) bool {
			return bookmarks[i].CreatedAt.After(bookmarks[j].CreatedAt)
		})

	case SortVisited:
		// Sort bookmarks by visit date (most recent first, never visited at end)
		sort.Slice(bookmarks, func(i, j int) bool {
			if bookmarks[i].VisitedAt == nil && bookmarks[j].VisitedAt == nil {
				return false // maintain order
			}
			if bookmarks[i].VisitedAt == nil {
				return false // nil goes to end
			}
			if bookmarks[j].VisitedAt == nil {
				return true // nil goes to end
			}
			return bookmarks[i].VisitedAt.After(*bookmarks[j].VisitedAt)
		})
	}
	// SortManual: keep insertion order (no sorting)

	// Add folders first (folders always before bookmarks)
	for i := range folders {
		a.browser.Items = append(a.browser.Items, Item{
			Kind:   ItemFolder,
			Folder: &folders[i],
		})
	}

	// Add bookmarks
	for i := range bookmarks {
		a.browser.Items = append(a.browser.Items, Item{
			Kind:     ItemBookmark,
			Bookmark: &bookmarks[i],
		})
	}
}

// refreshPinnedItems rebuilds the pinnedItems slice from the store, sorted by PinOrder.
func (a *App) refreshPinnedItems() {
	a.pinnedItems = []Item{}

	// Get pinned folders and bookmarks (already sorted by PinOrder)
	folders := a.store.GetPinnedFolders()
	bookmarks := a.store.GetPinnedBookmarks()

	// Combine into items with their PinOrder
	type orderedItem struct {
		item  Item
		order int
	}
	var ordered []orderedItem

	for i := range folders {
		ordered = append(ordered, orderedItem{
			item:  Item{Kind: ItemFolder, Folder: &folders[i]},
			order: folders[i].PinOrder,
		})
	}
	for i := range bookmarks {
		ordered = append(ordered, orderedItem{
			item:  Item{Kind: ItemBookmark, Bookmark: &bookmarks[i]},
			order: bookmarks[i].PinOrder,
		})
	}

	// Sort by PinOrder
	sort.Slice(ordered, func(i, j int) bool {
		return ordered[i].order < ordered[j].order
	})

	// Extract items in order
	for _, o := range ordered {
		a.pinnedItems = append(a.pinnedItems, o.item)
	}
}

// selectedPinnedItem returns the currently selected pinned item, or nil if none.
func (a *App) selectedPinnedItem() *Item {
	if len(a.pinnedItems) == 0 || a.pinnedCursor >= len(a.pinnedItems) {
		return nil
	}
	return &a.pinnedItems[a.pinnedCursor]
}

// buildFolderStack builds the folder stack from root to the given parent folder.
func (a *App) buildFolderStack(parentID *string) {
	if parentID == nil {
		return
	}
	// Build path from root to parent
	var path []string
	currentID := parentID
	for currentID != nil {
		folder := a.store.GetFolderByID(*currentID)
		if folder == nil {
			break
		}
		path = append([]string{folder.ID}, path...) // prepend
		currentID = folder.ParentID
	}
	a.browser.FolderStack = path
}

// updateFuzzyMatches performs fuzzy matching on allItems with the current query.
func (a *App) updateFuzzyMatches() {
	query := a.search.Input.Value()

	if query == "" {
		// No query - show all items
		a.search.FuzzyMatches = make([]fuzzyMatch, len(a.search.AllItems))
		for i, item := range a.search.AllItems {
			a.search.FuzzyMatches[i] = fuzzyMatch{Item: item}
		}
		return
	}

	// Run fuzzy matching
	matches := fuzzy.FindFrom(query, itemStrings(a.search.AllItems))

	// Convert to our fuzzyMatch type
	a.search.FuzzyMatches = make([]fuzzyMatch, len(matches))
	for i, m := range matches {
		a.search.FuzzyMatches[i] = fuzzyMatch{
			Item:           a.search.AllItems[m.Index],
			MatchedIndexes: m.MatchedIndexes,
			Score:          m.Score,
		}
	}

	// Reset cursor if out of bounds
	if a.search.FuzzyCursor >= len(a.search.FuzzyMatches) {
		a.search.FuzzyCursor = 0
	}
}

// collectAllTags gathers all unique tags from bookmarks.
func (a *App) collectAllTags() {
	tagSet := make(map[string]bool)
	for _, b := range a.store.Bookmarks {
		for _, tag := range b.Tags {
			tagSet[tag] = true
		}
	}
	a.modal.AllTags = make([]string, 0, len(tagSet))
	for tag := range tagSet {
		a.modal.AllTags = append(a.modal.AllTags, tag)
	}
	sort.Strings(a.modal.AllTags)
}

// updateTagSuggestions filters suggestions based on current input.
func (a *App) updateTagSuggestions() {
	// Get the current word being typed (after the last comma)
	input := a.modal.TagsInput.Value()
	lastComma := strings.LastIndex(input, ",")
	currentWord := input
	if lastComma >= 0 {
		currentWord = input[lastComma+1:]
	}
	currentWord = strings.TrimSpace(strings.ToLower(currentWord))

	if currentWord == "" {
		a.modal.TagSuggestions = nil
		a.modal.TagSuggestionIdx = -1
		return
	}

	// Get already-used tags in current input to avoid suggesting them
	usedTags := make(map[string]bool)
	for _, part := range strings.Split(input, ",") {
		usedTags[strings.TrimSpace(strings.ToLower(part))] = true
	}

	// Filter tags that start with currentWord and aren't already used
	a.modal.TagSuggestions = nil
	for _, tag := range a.modal.AllTags {
		tagLower := strings.ToLower(tag)
		if strings.HasPrefix(tagLower, currentWord) && !usedTags[tagLower] {
			a.modal.TagSuggestions = append(a.modal.TagSuggestions, tag)
		}
	}

	// Reset selection if out of bounds
	if a.modal.TagSuggestionIdx >= len(a.modal.TagSuggestions) {
		a.modal.TagSuggestionIdx = -1
	}
}

// insertTagSuggestion inserts the selected tag suggestion.
func (a *App) insertTagSuggestion() {
	if a.modal.TagSuggestionIdx < 0 || a.modal.TagSuggestionIdx >= len(a.modal.TagSuggestions) {
		return
	}

	tag := a.modal.TagSuggestions[a.modal.TagSuggestionIdx]
	input := a.modal.TagsInput.Value()
	lastComma := strings.LastIndex(input, ",")

	var newValue string
	if lastComma >= 0 {
		// Replace current word with selected tag
		newValue = input[:lastComma+1] + " " + tag
	} else {
		// Replace entire input with selected tag
		newValue = tag
	}

	a.modal.TagsInput.SetValue(newValue)
	a.modal.TagsInput.SetCursor(len(newValue))
	a.modal.TagSuggestions = nil
	a.modal.TagSuggestionIdx = -1
}

// Cursor returns the current cursor position.
func (a App) Cursor() int {
	return a.browser.Cursor
}

// CurrentFolderID returns the ID of the current folder (nil for root).
func (a App) CurrentFolderID() *string {
	return a.browser.CurrentFolderID
}

// Items returns the current list of items.
func (a App) Items() []Item {
	return a.browser.Items
}

// YankedItem returns the first item in the yank buffer, or nil if empty.
func (a App) YankedItem() *Item {
	if len(a.yankedItems) == 0 {
		return nil
	}
	return &a.yankedItems[0]
}

// YankedItems returns all items in the yank buffer.
func (a App) YankedItems() []Item {
	return a.yankedItems
}

// HasYankedItems returns true if there are items in the yank buffer.
func (a App) HasYankedItems() bool {
	return len(a.yankedItems) > 0
}

// Mode returns the current UI mode.
func (a App) Mode() Mode {
	return a.mode
}

// SortMode returns the current sort mode.
func (a App) SortMode() SortMode {
	return a.browser.SortMode
}

// FilterQuery returns the current filter query.
func (a App) FilterQuery() string {
	return a.search.FilterQuery
}

// StatusMessage returns the current status message text.
func (a App) StatusMessage() string {
	return a.messageText
}

// MessageType returns the current message type.
func (a App) MessageType() MessageType {
	return a.messageType
}

// WithDimensions returns a copy of the App with the specified dimensions.
// This is primarily used for testing with fixed terminal sizes.
func (a App) WithDimensions(width, height int) App {
	a.width = width
	a.height = height
	return a
}

// setMessage sets a status message with the given type.
// Returns a command to auto-clear the message after messageDuration.
func (a *App) setMessage(t MessageType, msg string) tea.Cmd {
	a.messageType = t
	a.messageText = msg
	return tea.Tick(messageDuration, func(time.Time) tea.Msg {
		return messageClearMsg{}
	})
}

// clearMessage clears the current message.
func (a *App) clearMessage() {
	a.messageType = MessageNone
	a.messageText = ""
}

// setStatus sets an info message (convenience wrapper for compatibility).
func (a *App) setStatus(msg string) {
	a.messageType = MessageInfo
	a.messageText = msg
}

// SetConfirmDelete sets the confirmDelete flag (for testing).
func (a *App) SetConfirmDelete(confirm bool) {
	a.confirmDelete = confirm
}

// FuzzyMatches returns the current fuzzy match results.
func (a App) FuzzyMatches() []fuzzyMatch {
	return a.search.FuzzyMatches
}

// FuzzyCursor returns the selected index in fuzzy results.
func (a App) FuzzyCursor() int {
	return a.search.FuzzyCursor
}

// Init implements tea.Model.
func (a App) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		return a, nil

	case messageClearMsg:
		// Auto-clear message after timeout
		a.clearMessage()
		return a, nil

	case openURLErrorMsg:
		// Failed to open URL in browser
		cmd := a.setMessage(MessageError, "Failed to open URL: "+msg.err.Error())
		return a, cmd

	case clipboardErrorMsg:
		// Failed to write to clipboard
		cmd := a.setMessage(MessageError, "Failed to copy to clipboard: "+msg.err.Error())
		return a, cmd

	case clipboardSuccessMsg:
		// Successfully copied to clipboard
		cmd := a.setMessage(MessageSuccess, "URL copied to clipboard")
		return a, cmd

	case cullProgressMsg:
		// Update progress during URL checking
		a.cull.Progress = msg.completed
		a.cull.Total = msg.total
		return a, nil

	case cullTickMsg:
		// Periodic tick to update progress display during cull
		if a.mode != ModeCullLoading {
			return a, nil // Stop ticking if not in loading mode
		}
		a.cull.Progress = int(atomic.LoadInt64(&cullProgressCounter))
		// Continue ticking
		return a, cullTickCmd()

	case organizeTickMsg:
		if a.mode != ModeOrganizeLoading {
			return a, nil
		}
		a.organize.Progress = int(atomic.LoadInt64(&organizeProgressCounter))
		if a.organize.Progress < a.organize.Total {
			return a, organizeTickCmd()
		}
		return a, nil

	case organizeResultsMsg:
		a.organize.Suggestions = msg.suggestions
		// Save cache for future use
		if len(msg.suggestions) > 0 {
			_ = a.saveOrganizeCache(msg.suggestions)
			a.organize.HasCache = true
			a.organize.CacheTime = time.Now()
		}
		if len(msg.suggestions) == 0 {
			a.mode = ModeNormal
			a.setStatus("All items already well-organized!")
			return a, nil
		}
		a.organize.Cursor = 0
		a.mode = ModeOrganizeResults
		return a, nil

	case cullCompleteMsg:
		// URL checking is complete - save cache
		_ = a.saveCullCache(msg.results)
		a.cull.HasCache = true
		a.cull.CacheTime = time.Now()

		a.cull.Results = msg.results
		a.cull.Groups = a.groupCullResults(msg.results)
		a.cull.GroupCursor = 0
		a.cull.ItemCursor = 0
		if len(a.cull.Groups) == 0 {
			// No dead/unreachable links found
			a.mode = ModeNormal
			cmd := a.setMessage(MessageSuccess, "All bookmarks healthy!")
			return a, cmd
		}
		a.mode = ModeCullResults
		return a, nil

	case aiResponseMsg:
		// Handle AI response for quick add
		if a.mode == ModeQuickAddLoading {
			if msg.err != nil {
				// AI failed - save to "To Review" with URL as title
				a.quickAdd.Error = msg.err
				url := a.quickAdd.Input.Value()
				folder, _ := a.store.GetOrCreateFolderByPath("To Review")
				var folderID *string
				if folder != nil {
					folderID = &folder.ID
				}
				newBookmark := model.NewBookmark(model.NewBookmarkParams{
					Title:    url,
					URL:      url,
					FolderID: folderID,
					Tags:     []string{},
				})
				a.store.AddBookmark(newBookmark)
				a.saveStore()
				a.refreshItems()
				a.mode = ModeNormal
				a.setStatus("AI failed, saved to 'To Review': " + msg.err.Error())
				return a, nil
			}

			// AI succeeded - show confirmation
			a.quickAdd.Response = msg.response
			a.mode = ModeQuickAddConfirm

			// Pre-fill inputs with AI suggestion
			a.modal.TitleInput.Reset()
			a.modal.TitleInput.SetValue(msg.response.Title)
			a.modal.TagsInput.Reset()
			a.modal.TagsInput.SetValue(strings.Join(msg.response.Tags, ", "))

			// Build folder picker options with smart ordering
			a.quickAdd.Folders = a.buildOrderedFolderPaths(a.browser.CurrentFolderID, msg.response.FolderPath)
			a.quickAdd.FilteredFolders = a.quickAdd.Folders
			a.quickAdd.FilterInput.Reset()

			// Find the AI suggested folder (normalize path)
			aiPath := "/" + strings.TrimPrefix(msg.response.FolderPath, "/")
			a.quickAdd.FolderIdx = a.findFolderIndex(aiPath)

			a.modal.TitleInput.Focus()
			return a, a.modal.TitleInput.Focus()
		}

		// Handle AI response for read later
		if a.mode == ModeReadLaterLoading {
			bookmarkURL := a.readLaterURL
			a.readLaterURL = ""
			a.mode = ModeNormal

			// Get or create the quick add folder
			folder, _ := a.store.GetOrCreateFolderByPath(a.config.QuickAddFolder)
			var folderID *string
			if folder != nil {
				folderID = &folder.ID
			}

			var title string
			var tags []string

			if msg.err != nil {
				// AI failed - use URL as title
				title = bookmarkURL
				tags = []string{}
				a.setStatus("AI unavailable - saved with URL as title")
			} else {
				// AI succeeded
				title = msg.response.Title
				tags = msg.response.Tags
			}

			newBookmark := model.NewBookmark(model.NewBookmarkParams{
				Title:    title,
				URL:      bookmarkURL,
				FolderID: folderID,
				Tags:     tags,
			})
			a.store.AddBookmark(newBookmark)
			a.saveStore()
			a.refreshItems()

			if msg.err == nil {
				cmd := a.setMessage(MessageSuccess, "Added to "+a.config.QuickAddFolder+": "+title)
				return a, cmd
			}
			return a, nil
		}
		return a, nil

	case tea.KeyMsg:
		// Handle q to quit globally (except when in text input mode)
		if key.Matches(msg, a.keys.Quit) && !a.mode.hasTextInput() {
			return a, tea.Quit
		}

		// Handle modal modes
		if a.mode != ModeNormal {
			return a.updateModal(msg)
		}

		// Handle global keys (work in any pane, normal mode only)
		if key.Matches(msg, a.keys.Help) {
			a.mode = ModeHelp
			return a, nil
		}

		// Handle 0 key globally - jump to pinned pane
		if msg.String() == "0" && len(a.pinnedItems) > 0 {
			a.focusedPane = PanePinned
			return a, nil
		}

		switch {
		case key.Matches(msg, a.keys.Search):
			// Open fuzzy finder mode with GLOBAL search
			a.mode = ModeSearch
			a.search.Input.Reset()
			a.search.Input.Focus()
			a.search.FuzzyCursor = 0
			// Gather ALL items from the entire store for fuzzy matching
			a.search.AllItems = []Item{}
			for i := range a.store.Folders {
				a.search.AllItems = append(a.search.AllItems, Item{
					Kind:   ItemFolder,
					Folder: &a.store.Folders[i],
				})
			}
			for i := range a.store.Bookmarks {
				a.search.AllItems = append(a.search.AllItems, Item{
					Kind:     ItemBookmark,
					Bookmark: &a.store.Bookmarks[i],
				})
			}
			a.updateFuzzyMatches()
			return a, a.search.Input.Focus()

		case key.Matches(msg, a.keys.Filter):
			// Open local filter for current folder
			a.mode = ModeFilter
			a.search.FilterInput.Reset()
			a.search.FilterInput.SetValue(a.search.FilterQuery) // Restore previous filter
			a.search.FilterInput.Focus()
			return a, a.search.FilterInput.Focus()

		case key.Matches(msg, a.keys.QuickAdd):
			// AI-powered quick add
			a.mode = ModeQuickAdd
			a.quickAdd.Reset()
			// Pre-fill with clipboard contents
			if clipContent, err := clipboard.ReadAll(); err == nil && clipContent != "" {
				a.quickAdd.Input.SetValue(clipContent)
			}
			a.quickAdd.Input.Focus()
			return a, a.quickAdd.Input.Focus()

		case key.Matches(msg, a.keys.ReadLater):
			// Quick add to Read Later from clipboard
			clipContent, err := clipboard.ReadAll()
			if err != nil {
				cmd := a.setMessage(MessageError, "Failed to read clipboard")
				return a, cmd
			}
			clipContent = strings.TrimSpace(clipContent)
			if clipContent == "" {
				cmd := a.setMessage(MessageError, "No URL in clipboard")
				return a, cmd
			}
			// Validate URL
			parsedURL, err := url.Parse(clipContent)
			if err != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") {
				cmd := a.setMessage(MessageError, "Invalid URL in clipboard")
				return a, cmd
			}
			// Start AI-powered quick add to Read Later
			a.readLaterURL = clipContent
			a.mode = ModeReadLaterLoading
			cmd := a.setMessage(MessageInfo, "Adding to "+a.config.QuickAddFolder+"...")
			return a, tea.Batch(cmd, a.callAICmd(clipContent))

		case key.Matches(msg, a.keys.Cull):
			// Check if cache exists
			a.checkCullCache()
			if a.cull.HasCache {
				// Show menu to choose fresh vs cached
				a.cull.MenuCursor = 0
				a.mode = ModeCullMenu
				return a, nil
			}
			// No cache - go directly to loading
			a.cull.Reset()
			a.cull.Total = len(a.store.Bookmarks)
			a.mode = ModeCullLoading
			return a, a.startCullCmd()

		case key.Matches(msg, a.keys.Organize):
			// Organize: analyze current item or folder contents
			return a.startOrganize()

		case key.Matches(msg, a.keys.ToggleConfirm):
			// Toggle delete confirmation
			a.confirmDelete = !a.confirmDelete
			return a, nil

		case key.Matches(msg, a.keys.AddBookmark):
			a.mode = ModeAddBookmark
			a.modal.TitleInput.Reset()
			a.modal.URLInput.Reset()
			a.modal.TitleInput.Focus()
			return a, a.modal.TitleInput.Focus()

		case key.Matches(msg, a.keys.AddFolder):
			a.mode = ModeAddFolder
			a.modal.TitleInput.Reset()
			a.modal.TitleInput.Focus()
			return a, a.modal.TitleInput.Focus()
		}

		// Handle pinned pane navigation
		if a.focusedPane == PanePinned {
			return a.updatePinnedPane(msg)
		}

		// Browser pane: Handle gg sequence
		if key.Matches(msg, a.keys.Top) {
			if a.lastKeyWasG {
				// This is the second g - go to top
				a.browser.Cursor = 0
				a.lastKeyWasG = false
				return a, nil
			}
			// First g - wait for second
			a.lastKeyWasG = true
			return a, nil
		}

		// Handle y - yank (copy)
		if key.Matches(msg, a.keys.Yank) {
			a.lastKeyWasG = false
			a.yankCurrentItem()
			return a, nil
		}

		// Handle d - delete (without buffer)
		if key.Matches(msg, a.keys.Delete) {
			a.lastKeyWasG = false
			a.deleteCurrentItem()
			return a, nil
		}

		// Handle x - cut (delete + buffer)
		if key.Matches(msg, a.keys.Cut) {
			a.lastKeyWasG = false
			a.cutCurrentItem()
			return a, nil
		}

		// Handle m - toggle pin
		if key.Matches(msg, a.keys.Pin) {
			a.lastKeyWasG = false
			cmd := a.togglePinCurrentItem()
			return a, cmd
		}

		// Handle M - move to folder
		if key.Matches(msg, a.keys.Move) {
			a.lastKeyWasG = false
			displayItems := a.getDisplayItems()
			if len(displayItems) == 0 || a.browser.Cursor >= len(displayItems) {
				return a, nil
			}

			// Capture items to move (selected or current)
			a.move.ItemsToMove = nil
			if a.selection.HasSelection() {
				for _, item := range displayItems {
					if a.selection.IsSelected(item.ID()) {
						a.move.ItemsToMove = append(a.move.ItemsToMove, item)
					}
				}
			} else {
				a.move.ItemsToMove = []Item{displayItems[a.browser.Cursor]}
			}

			a.mode = ModeMove
			a.move.Folders = a.buildFolderPaths()
			a.move.FilteredFolders = a.move.Folders // Start with all folders
			a.move.FilterInput.Reset()
			a.move.FilterInput.Focus()
			// Find current location in folder list
			item := displayItems[a.browser.Cursor]
			currentPath := "/"
			if item.IsFolder() && item.Folder.ParentID != nil {
				currentPath = a.store.GetFolderPath(item.Folder.ParentID)
			} else if !item.IsFolder() && item.Bookmark.FolderID != nil {
				currentPath = a.store.GetFolderPath(item.Bookmark.FolderID)
			}
			a.move.FolderIdx = a.findMoveFolderIndex(currentPath)
			return a, a.move.FilterInput.Focus()
		}

		// Handle v - toggle selection on current item
		if key.Matches(msg, a.keys.Select) {
			a.lastKeyWasG = false
			a.toggleSelectCurrentItem()
			return a, nil
		}

		// Handle V - enter visual line mode
		if key.Matches(msg, a.keys.SelectVisual) {
			a.lastKeyWasG = false
			a.enterVisualMode()
			return a, nil
		}

		// Handle Esc - clear selection, or navigate back in browser pane
		if key.Matches(msg, a.keys.ClearSelect) {
			a.lastKeyWasG = false
			// First priority: clear selection if any
			if a.selection.HasSelection() {
				a.clearSelection()
				return a, nil
			}
			// Second priority: navigate back in browser pane
			if a.focusedPane == PaneBrowser {
				if a.browser.CurrentFolderID == nil {
					// At root: switch to pinned pane
					a.focusedPane = PanePinned
					return a, nil
				}
				// Go back to parent folder
				if len(a.browser.FolderStack) > 0 {
					lastIdx := len(a.browser.FolderStack) - 1
					parentID := a.browser.FolderStack[lastIdx]
					a.browser.FolderStack = a.browser.FolderStack[:lastIdx]
					a.browser.CurrentFolderID = &parentID
				} else {
					a.browser.CurrentFolderID = nil
				}
				a.browser.Cursor = 0
				a.refreshItems()
				return a, nil
			}
			// In pinned pane: do nothing
			return a, nil
		}

		// Reset sequence flags for any other key
		a.lastKeyWasG = false

		switch {
		case key.Matches(msg, a.keys.Down):
			displayItems := a.getDisplayItems()
			if len(displayItems) > 0 && a.browser.Cursor < len(displayItems)-1 {
				a.browser.Cursor++
				a.updateVisualSelection()
			}
			a.clearMessage() // Clear message on navigation

		case key.Matches(msg, a.keys.Up):
			if a.browser.Cursor > 0 {
				a.browser.Cursor--
				a.updateVisualSelection()
			}
			a.clearMessage() // Clear message on navigation

		case key.Matches(msg, a.keys.Bottom):
			displayItems := a.getDisplayItems()
			if len(displayItems) > 0 {
				a.browser.Cursor = len(displayItems) - 1
				a.updateVisualSelection()
			}

		case key.Matches(msg, a.keys.Right):
			// Enter folder or open bookmark
			displayItems := a.getDisplayItems()
			if len(displayItems) > 0 && a.browser.Cursor < len(displayItems) {
				item := displayItems[a.browser.Cursor]
				if item.IsFolder() {
					// Push current folder to stack
					if a.browser.CurrentFolderID != nil {
						a.browser.FolderStack = append(a.browser.FolderStack, *a.browser.CurrentFolderID)
					}
					// Enter the folder
					id := item.Folder.ID
					a.browser.CurrentFolderID = &id
					a.browser.Cursor = 0
					a.refreshItems()
				} else {
					// Open bookmark URL
					return a.openBookmark()
				}
			}

		case key.Matches(msg, a.keys.Left):
			// At root level: switch to pinned pane
			if a.browser.CurrentFolderID == nil {
				a.focusedPane = PanePinned
				return a, nil
			}
			// Go back to parent folder
			if len(a.browser.FolderStack) > 0 {
				// Pop from stack
				lastIdx := len(a.browser.FolderStack) - 1
				parentID := a.browser.FolderStack[lastIdx]
				a.browser.FolderStack = a.browser.FolderStack[:lastIdx]
				a.browser.CurrentFolderID = &parentID
			} else {
				// Back to root
				a.browser.CurrentFolderID = nil
			}
			a.browser.Cursor = 0
			a.refreshItems()

		case key.Matches(msg, a.keys.PasteAfter):
			a.pasteItem(false) // after cursor

		case key.Matches(msg, a.keys.PasteBefore):
			a.pasteItem(true) // before cursor

		case key.Matches(msg, a.keys.Edit):
			// Only edit if there's an item selected
			displayItems := a.getDisplayItems()
			if len(displayItems) == 0 || a.browser.Cursor >= len(displayItems) {
				return a, nil
			}
			item := displayItems[a.browser.Cursor]
			if item.IsFolder() {
				a.mode = ModeEditFolder
				a.modal.EditItemID = item.Folder.ID
				a.modal.TitleInput.Reset()
				a.modal.TitleInput.SetValue(item.Folder.Name)
				a.modal.TitleInput.Focus()
				return a, a.modal.TitleInput.Focus()
			} else {
				a.mode = ModeEditBookmark
				a.modal.EditItemID = item.Bookmark.ID
				a.modal.TitleInput.Reset()
				a.modal.TitleInput.SetValue(item.Bookmark.Title)
				a.modal.URLInput.Reset()
				a.modal.URLInput.SetValue(item.Bookmark.URL)
				a.modal.TitleInput.Focus()
				return a, a.modal.TitleInput.Focus()
			}

		case key.Matches(msg, a.keys.EditTags):
			// Only edit tags on bookmarks
			displayItems := a.getDisplayItems()
			if len(displayItems) == 0 || a.browser.Cursor >= len(displayItems) {
				return a, nil
			}
			item := displayItems[a.browser.Cursor]
			if item.IsFolder() {
				// Folders don't have tags
				return a, nil
			}
			a.mode = ModeEditTags
			a.modal.EditItemID = item.Bookmark.ID
			a.modal.TagsInput.Reset()
			// Convert tags slice to comma-separated string
			a.modal.TagsInput.SetValue(strings.Join(item.Bookmark.Tags, ", "))
			a.modal.TagsInput.Focus()
			// Initialize tag autocompletion
			a.collectAllTags()
			a.modal.TagSuggestions = nil
			a.modal.TagSuggestionIdx = -1
			return a, a.modal.TagsInput.Focus()

		case key.Matches(msg, a.keys.Sort):
			// Cycle through sort modes
			a.browser.SortMode = (a.browser.SortMode + 1) % 4
			a.refreshItems()

		case key.Matches(msg, a.keys.YankURL):
			// Yank URL to clipboard
			return a.yankURLToClipboard()

		}
	}

	return a, nil
}

// updatePinnedPane handles key events when the pinned pane is focused.
func (a App) updatePinnedPane(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle gg sequence
	if key.Matches(msg, a.keys.Top) {
		if a.lastKeyWasG {
			a.pinnedCursor = 0
			a.lastKeyWasG = false
			return a, nil
		}
		a.lastKeyWasG = true
		return a, nil
	}

	// Handle d - unpin (in pinned pane, d just unpins, doesn't delete)
	if key.Matches(msg, a.keys.Delete) {
		a.lastKeyWasG = false
		cmd := a.unpinSelectedItem()
		return a, cmd
	}

	// Handle x - unpin (same as d in pinned pane)
	if key.Matches(msg, a.keys.Cut) {
		a.lastKeyWasG = false
		cmd := a.unpinSelectedItem()
		return a, cmd
	}

	// Handle m - unpin
	if key.Matches(msg, a.keys.Pin) {
		a.lastKeyWasG = false
		cmd := a.unpinSelectedItem()
		return a, cmd
	}

	a.lastKeyWasG = false

	switch {
	case key.Matches(msg, a.keys.Down):
		if len(a.pinnedItems) > 0 && a.pinnedCursor < len(a.pinnedItems)-1 {
			a.pinnedCursor++
		}
		a.clearMessage()

	case key.Matches(msg, a.keys.Up):
		if a.pinnedCursor > 0 {
			a.pinnedCursor--
		}
		a.clearMessage()

	case key.Matches(msg, a.keys.Bottom):
		if len(a.pinnedItems) > 0 {
			a.pinnedCursor = len(a.pinnedItems) - 1
		}

	case key.Matches(msg, a.keys.Right):
		// Switch to browser pane
		a.focusedPane = PaneBrowser

	case key.Matches(msg, a.keys.Left):
		// Already at leftmost pane, do nothing
		return a, nil
	}

	// Handle J/K for reordering pinned items
	if msg.String() == "J" {
		return a.movePinnedItemDown()
	}
	if msg.String() == "K" {
		return a.movePinnedItemUp()
	}

	// Handle 1-9 for quick access to pinned items
	if len(msg.String()) == 1 && msg.String() >= "1" && msg.String() <= "9" {
		idx := int(msg.String()[0] - '1') // "1" -> 0, "2" -> 1, etc.
		if idx < len(a.pinnedItems) {
			a.pinnedCursor = idx
			return a.activatePinnedItem()
		}
		return a, nil
	}

	// Handle Enter key for activating pinned items
	if msg.Type == tea.KeyEnter {
		return a.activatePinnedItem()
	}

	return a, nil
}

// movePinnedItemUp moves the selected pinned item up (lower PinOrder).
func (a *App) movePinnedItemUp() (tea.Model, tea.Cmd) {
	if a.pinnedCursor <= 0 || len(a.pinnedItems) < 2 {
		return a, nil
	}

	// Get current and target items
	current := a.pinnedItems[a.pinnedCursor]
	target := a.pinnedItems[a.pinnedCursor-1]

	// Get their PinOrders
	var currentOrder, targetOrder int
	if current.IsFolder() {
		currentOrder = current.Folder.PinOrder
	} else {
		currentOrder = current.Bookmark.PinOrder
	}
	if target.IsFolder() {
		targetOrder = target.Folder.PinOrder
	} else {
		targetOrder = target.Bookmark.PinOrder
	}

	// Swap orders
	a.store.SwapPinOrders(currentOrder, targetOrder)
	a.refreshPinnedItems()
	a.pinnedCursor--

	return a, nil
}

// movePinnedItemDown moves the selected pinned item down (higher PinOrder).
func (a *App) movePinnedItemDown() (tea.Model, tea.Cmd) {
	if a.pinnedCursor >= len(a.pinnedItems)-1 || len(a.pinnedItems) < 2 {
		return a, nil
	}

	// Get current and target items
	current := a.pinnedItems[a.pinnedCursor]
	target := a.pinnedItems[a.pinnedCursor+1]

	// Get their PinOrders
	var currentOrder, targetOrder int
	if current.IsFolder() {
		currentOrder = current.Folder.PinOrder
	} else {
		currentOrder = current.Bookmark.PinOrder
	}
	if target.IsFolder() {
		targetOrder = target.Folder.PinOrder
	} else {
		targetOrder = target.Bookmark.PinOrder
	}

	// Swap orders
	a.store.SwapPinOrders(currentOrder, targetOrder)
	a.refreshPinnedItems()
	a.pinnedCursor++

	return a, nil
}

// activatePinnedItem opens a bookmark or navigates to a folder from the pinned pane.
func (a *App) activatePinnedItem() (tea.Model, tea.Cmd) {
	item := a.selectedPinnedItem()
	if item == nil {
		return a, nil
	}

	if item.IsFolder() {
		// Navigate browser to this folder
		a.buildFolderStack(item.Folder.ParentID)
		id := item.Folder.ID
		a.browser.CurrentFolderID = &id
		a.browser.Cursor = 0
		a.refreshItems()
		a.focusedPane = PaneBrowser
		return a, nil
	}

	// Open bookmark URL
	if item.Bookmark != nil && item.Bookmark.URL != "" {
		// Update visited time
		now := time.Now()
		if b := a.store.GetBookmarkByID(item.Bookmark.ID); b != nil {
			b.VisitedAt = &now
		}
		a.refreshPinnedItems()
		return a, openURLCmd(item.Bookmark.URL)
	}
	return a, nil
}

// unpinSelectedItem unpins the currently selected item in the pinned pane.
// Returns a command to schedule message auto-clear.
func (a *App) unpinSelectedItem() tea.Cmd {
	item := a.selectedPinnedItem()
	if item == nil {
		return nil
	}

	var cmd tea.Cmd
	if item.IsFolder() {
		if err := a.store.TogglePinFolder(item.Folder.ID); err != nil {
			cmd = a.setMessage(MessageError, "Failed to unpin: "+err.Error())
		} else {
			cmd = a.setMessage(MessageSuccess, "Unpinned: "+item.Folder.Name)
		}
	} else {
		if err := a.store.TogglePinBookmark(item.Bookmark.ID); err != nil {
			cmd = a.setMessage(MessageError, "Failed to unpin: "+err.Error())
		} else {
			cmd = a.setMessage(MessageSuccess, "Unpinned: "+item.Bookmark.Title)
		}
	}

	a.refreshPinnedItems()

	// Adjust cursor if needed
	if a.pinnedCursor >= len(a.pinnedItems) && a.pinnedCursor > 0 {
		a.pinnedCursor--
	}

	return cmd
}

// togglePinCurrentItem toggles pin on the currently selected item (or selected items) in browser pane.
// Returns a command to schedule message auto-clear.
func (a *App) togglePinCurrentItem() tea.Cmd {
	displayItems := a.getDisplayItems()
	if len(displayItems) == 0 || a.browser.Cursor >= len(displayItems) {
		return nil
	}

	var cmd tea.Cmd

	// Handle batch pin toggle
	if a.selection.HasSelection() {
		var pinCount, unpinCount int
		for _, item := range displayItems {
			if !a.selection.IsSelected(item.ID()) {
				continue
			}
			if item.IsFolder() {
				wasPinned := item.Folder.Pinned
				if err := a.store.TogglePinFolder(item.Folder.ID); err == nil {
					if wasPinned {
						unpinCount++
					} else {
						pinCount++
					}
				}
			} else {
				wasPinned := item.Bookmark.Pinned
				if err := a.store.TogglePinBookmark(item.Bookmark.ID); err == nil {
					if wasPinned {
						unpinCount++
					} else {
						pinCount++
					}
				}
			}
		}

		a.saveStore()
		a.clearSelection()
		a.refreshItems()
		a.refreshPinnedItems()

		// Build message
		if pinCount > 0 && unpinCount > 0 {
			cmd = a.setMessage(MessageSuccess, "Pinned "+strconv.Itoa(pinCount)+", unpinned "+strconv.Itoa(unpinCount)+" items")
		} else if pinCount > 0 {
			cmd = a.setMessage(MessageSuccess, "Pinned "+strconv.Itoa(pinCount)+" items")
		} else {
			cmd = a.setMessage(MessageSuccess, "Unpinned "+strconv.Itoa(unpinCount)+" items")
		}
		return cmd
	}

	// Single item toggle
	item := displayItems[a.browser.Cursor]
	if item.IsFolder() {
		wasPinned := item.Folder.Pinned
		if err := a.store.TogglePinFolder(item.Folder.ID); err != nil {
			cmd = a.setMessage(MessageError, "Failed to toggle pin: "+err.Error())
		} else if wasPinned {
			cmd = a.setMessage(MessageSuccess, "Unpinned: "+item.Folder.Name)
		} else {
			cmd = a.setMessage(MessageSuccess, "Pinned: "+item.Folder.Name)
		}
	} else {
		wasPinned := item.Bookmark.Pinned
		if err := a.store.TogglePinBookmark(item.Bookmark.ID); err != nil {
			cmd = a.setMessage(MessageError, "Failed to toggle pin: "+err.Error())
		} else if wasPinned {
			cmd = a.setMessage(MessageSuccess, "Unpinned: "+item.Bookmark.Title)
		} else {
			cmd = a.setMessage(MessageSuccess, "Pinned: "+item.Bookmark.Title)
		}
	}

	a.saveStore()
	a.refreshItems()
	a.refreshPinnedItems()
	return cmd
}

// updateModal handles key events when in a modal mode.
func (a App) updateModal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle help overlay
	if a.mode == ModeHelp {
		switch {
		case msg.Type == tea.KeyEsc:
			a.mode = ModeNormal
			return a, nil
		case key.Matches(msg, a.keys.Help):
			// '?' toggles help off
			a.mode = ModeNormal
			return a, nil
		}
		// q quits (handled globally above)
		return a, nil
	}

	// Handle cull menu mode (fresh vs cached)
	if a.mode == ModeCullMenu {
		switch msg.Type {
		case tea.KeyEsc:
			a.mode = ModeNormal
			return a, nil
		case tea.KeyEnter:
			if a.cull.MenuCursor == 0 {
				// Fresh check
				a.cull.Reset()
				a.cull.Total = len(a.store.Bookmarks)
				a.mode = ModeCullLoading
				return a, a.startCullCmd()
			} else {
				// Use cached results
				results, _, err := a.loadCullCache()
				if err != nil {
					a.setMessage(MessageError, "Cache load failed: "+err.Error())
					a.mode = ModeNormal
					return a, nil
				}
				a.cull.Results = results
				a.cull.Groups = a.groupCullResults(results)
				a.cull.GroupCursor = 0
				a.cull.ItemCursor = 0
				a.mode = ModeCullResults
			}
			return a, nil
		case tea.KeyDown:
			if a.cull.MenuCursor < 1 {
				a.cull.MenuCursor++
			}
			return a, nil
		case tea.KeyUp:
			if a.cull.MenuCursor > 0 {
				a.cull.MenuCursor--
			}
			return a, nil
		}
		// Handle vim-style navigation
		if msg.Type == tea.KeyRunes {
			switch string(msg.Runes) {
			case "j":
				if a.cull.MenuCursor < 1 {
					a.cull.MenuCursor++
				}
				return a, nil
			case "k":
				if a.cull.MenuCursor > 0 {
					a.cull.MenuCursor--
				}
				return a, nil
			}
		}
		return a, nil
	}

	// Handle organize menu mode
	if a.mode == ModeOrganizeMenu {
		switch msg.Type {
		case tea.KeyEsc:
			a.mode = ModeNormal
			return a, nil
		case tea.KeyEnter:
			if a.organize.MenuCursor == 0 {
				// Fresh analysis
				return a.startOrganizeAnalysis()
			} else {
				// Use cached results
				suggestions, _, err := a.loadOrganizeCache()
				if err != nil {
					a.setMessage(MessageError, "Cache load failed: "+err.Error())
					a.mode = ModeNormal
					return a, nil
				}
				a.organize.Suggestions = suggestions
				a.organize.Cursor = 0
				if len(suggestions) == 0 {
					a.mode = ModeNormal
					a.setStatus("All items already well-organized!")
				} else {
					a.mode = ModeOrganizeResults
				}
			}
			return a, nil
		case tea.KeyDown:
			if a.organize.MenuCursor < 1 {
				a.organize.MenuCursor++
			}
			return a, nil
		case tea.KeyUp:
			if a.organize.MenuCursor > 0 {
				a.organize.MenuCursor--
			}
			return a, nil
		}
		// Handle vim-style navigation
		if msg.Type == tea.KeyRunes {
			switch string(msg.Runes) {
			case "j":
				if a.organize.MenuCursor < 1 {
					a.organize.MenuCursor++
				}
				return a, nil
			case "k":
				if a.organize.MenuCursor > 0 {
					a.organize.MenuCursor--
				}
				return a, nil
			case "q":
				a.mode = ModeNormal
				return a, nil
			}
		}
		return a, nil
	}

	// Handle cull loading mode
	if a.mode == ModeCullLoading {
		// Only allow Esc to cancel
		if msg.Type == tea.KeyEsc {
			a.cull.Reset()
			a.mode = ModeNormal
			return a, nil
		}
		return a, nil
	}

	// Handle organize loading mode
	if a.mode == ModeOrganizeLoading {
		// Only allow Esc to cancel
		if msg.Type == tea.KeyEsc {
			a.organize.Reset()
			a.mode = ModeNormal
			return a, nil
		}
		return a, nil
	}

	// Handle cull results mode (group list)
	if a.mode == ModeCullResults {
		switch msg.Type {
		case tea.KeyEsc:
			a.cull.Reset()
			a.mode = ModeNormal
			return a, nil
		case tea.KeyEnter:
			// Enter inspect mode for selected group
			if len(a.cull.Groups) > 0 {
				a.cull.ItemCursor = 0
				a.mode = ModeCullInspect
			}
			return a, nil
		case tea.KeyDown:
			if len(a.cull.Groups) > 0 && a.cull.GroupCursor < len(a.cull.Groups)-1 {
				a.cull.GroupCursor++
			}
			return a, nil
		case tea.KeyUp:
			if a.cull.GroupCursor > 0 {
				a.cull.GroupCursor--
			}
			return a, nil
		}
		// Handle vim-style navigation and commands
		if msg.Type == tea.KeyRunes {
			switch string(msg.Runes) {
			case "j":
				if len(a.cull.Groups) > 0 && a.cull.GroupCursor < len(a.cull.Groups)-1 {
					a.cull.GroupCursor++
				}
				return a, nil
			case "k":
				if a.cull.GroupCursor > 0 {
					a.cull.GroupCursor--
				}
				return a, nil
			case "d":
				// Delete all in selected group
				return a.cullDeleteGroup()
			}
			// q quits (handled globally above)
		}
		return a, nil
	}

	// Handle cull inspect mode (bookmark list within group)
	if a.mode == ModeCullInspect {
		group := a.cull.CurrentGroup()
		if group == nil {
			a.mode = ModeCullResults
			return a, nil
		}

		switch msg.Type {
		case tea.KeyEsc:
			// Back to group list
			a.mode = ModeCullResults
			return a, nil
		case tea.KeyDown:
			if len(group.Results) > 0 && a.cull.ItemCursor < len(group.Results)-1 {
				a.cull.ItemCursor++
			}
			return a, nil
		case tea.KeyUp:
			if a.cull.ItemCursor > 0 {
				a.cull.ItemCursor--
			}
			return a, nil
		}
		// Handle vim-style navigation and commands
		if msg.Type == tea.KeyRunes {
			switch string(msg.Runes) {
			case "j":
				if len(group.Results) > 0 && a.cull.ItemCursor < len(group.Results)-1 {
					a.cull.ItemCursor++
				}
				return a, nil
			case "k":
				if a.cull.ItemCursor > 0 {
					a.cull.ItemCursor--
				}
				return a, nil
			case "d":
				// Delete current item
				return a.cullDeleteItem()
			case "o":
				// Open in browser
				return a.cullOpenItem()
			case "e":
				// Edit bookmark
				return a.cullEditItem()
			case "m":
				// Move bookmark
				return a.cullMoveItem()
			}
		}
		return a, nil
	}

	// Handle organize results mode
	if a.mode == ModeOrganizeResults {
		switch msg.Type {
		case tea.KeyEsc:
			a.mode = ModeNormal
			a.organize.Reset()
			return a, nil
		case tea.KeyEnter:
			return a.organizeAcceptCurrent()
		case tea.KeyDown:
			a.organizeNextUnprocessed()
			return a, nil
		case tea.KeyUp:
			a.organizePrevUnprocessed()
			return a, nil
		}
		// Handle vim-style navigation and commands
		if msg.Type == tea.KeyRunes {
			switch string(msg.Runes) {
			case "j":
				a.organizeNextUnprocessed()
				return a, nil
			case "k":
				a.organizePrevUnprocessed()
				return a, nil
			case "s":
				a.organizeSkipCurrent()
				return a, nil
			case "o":
				return a.organizeOpenCurrent()
			case "m":
				return a.organizeMoveCurrent()
			case "d":
				return a.organizeDeleteCurrent()
			case "q":
				a.mode = ModeNormal
				a.organize.Reset()
				return a, nil
			}
		}
		return a, nil
	}

	// Handle confirm delete modal separately (simple yes/no)
	if a.mode == ModeConfirmDelete {
		switch msg.Type {
		case tea.KeyEsc:
			// Cancel deletion
			a.mode = ModeNormal
			return a, nil
		case tea.KeyEnter:
			// Confirm deletion
			a.confirmDeleteItem()
			a.mode = ModeNormal
			return a, nil
		}
		return a, nil
	}

	// Handle move mode (folder picker with filter)
	if a.mode == ModeMove {
		switch msg.Type {
		case tea.KeyEsc:
			// Cancel move - return to appropriate mode
			if a.move.ReturnMode != 0 {
				a.mode = a.move.ReturnMode
			} else {
				a.mode = ModeNormal
			}
			return a, nil
		case tea.KeyEnter:
			// Execute move if there are filtered results
			if len(a.move.FilteredFolders) > 0 {
				a.executeMoveItem()
			}
			// Mark organize suggestion as processed if coming from organize
			if a.move.OrganizeSuggestion != nil {
				a.move.OrganizeSuggestion.Processed = true
				// Check if there are more suggestions
				if a.organize.UnprocessedCount() == 0 {
					a.mode = ModeNormal
					a.refreshItems()
					a.organize.Reset()
				} else {
					a.mode = ModeOrganizeResults
					a.organizeNextUnprocessed()
				}
			} else if a.move.ReturnMode != 0 {
				a.mode = a.move.ReturnMode
			} else {
				a.mode = ModeNormal
			}
			return a, nil
		case tea.KeyUp, tea.KeyCtrlP:
			if a.move.FolderIdx > 0 {
				a.move.FolderIdx--
			} else if len(a.move.FilteredFolders) > 0 {
				a.move.FolderIdx = len(a.move.FilteredFolders) - 1
			}
			return a, nil
		case tea.KeyDown, tea.KeyCtrlN:
			if len(a.move.FilteredFolders) > 0 {
				a.move.FolderIdx++
				if a.move.FolderIdx >= len(a.move.FilteredFolders) {
					a.move.FolderIdx = 0
				}
			}
			return a, nil
		case tea.KeyTab:
			// Tab also moves down
			if len(a.move.FilteredFolders) > 0 {
				a.move.FolderIdx++
				if a.move.FolderIdx >= len(a.move.FilteredFolders) {
					a.move.FolderIdx = 0
				}
			}
			return a, nil
		}

		// Forward to filter input and update filter
		var cmd tea.Cmd
		a.move.FilterInput, cmd = a.move.FilterInput.Update(msg)
		a.updateMoveFilter()
		return a, cmd
	}

	// Handle search mode (fuzzy finder)
	if a.mode == ModeSearch {
		switch msg.Type {
		case tea.KeyEsc:
			// Cancel search
			a.mode = ModeNormal
			a.search.FuzzyMatches = nil
			a.search.AllItems = nil
			return a, nil

		case tea.KeyEnter:
			// Select highlighted item: navigate to folder or bookmark location
			if len(a.search.FuzzyMatches) > 0 && a.search.FuzzyCursor < len(a.search.FuzzyMatches) {
				selectedItem := a.search.FuzzyMatches[a.search.FuzzyCursor].Item

				if selectedItem.IsFolder() {
					// Navigate into the selected folder
					folderID := selectedItem.Folder.ID
					a.browser.FolderStack = []string{}
					a.buildFolderStack(selectedItem.Folder.ParentID)
					a.browser.CurrentFolderID = &folderID
					a.browser.Cursor = 0
					a.refreshItems()
				} else {
					// Navigate to bookmark's folder and position cursor on it
					bookmark := selectedItem.Bookmark
					a.browser.FolderStack = []string{}

					if bookmark.FolderID != nil {
						// Bookmark is in a folder - navigate there
						folder := a.store.GetFolderByID(*bookmark.FolderID)
						if folder != nil {
							a.buildFolderStack(folder.ParentID)
						}
						a.browser.CurrentFolderID = bookmark.FolderID
					} else {
						// Bookmark is at root
						a.browser.CurrentFolderID = nil
					}

					a.refreshItems()

					// Find and position cursor on the bookmark
					for i, item := range a.browser.Items {
						if !item.IsFolder() && item.Bookmark.ID == bookmark.ID {
							a.browser.Cursor = i
							break
						}
					}
				}
			}
			a.mode = ModeNormal
			a.search.FuzzyMatches = nil
			a.search.AllItems = nil
			return a, nil

		case tea.KeyDown:
			// Navigate down in results
			if len(a.search.FuzzyMatches) > 0 && a.search.FuzzyCursor < len(a.search.FuzzyMatches)-1 {
				a.search.FuzzyCursor++
			}
			return a, nil

		case tea.KeyUp:
			// Navigate up in results
			if a.search.FuzzyCursor > 0 {
				a.search.FuzzyCursor--
			}
			return a, nil
		}

		// Handle j/k for vim-style navigation
		if msg.Type == tea.KeyRunes {
			switch string(msg.Runes) {
			case "j":
				if len(a.search.FuzzyMatches) > 0 && a.search.FuzzyCursor < len(a.search.FuzzyMatches)-1 {
					a.search.FuzzyCursor++
				}
				return a, nil
			case "k":
				if a.search.FuzzyCursor > 0 {
					a.search.FuzzyCursor--
				}
				return a, nil
			}
		}

		// Update search input
		var cmd tea.Cmd
		a.search.Input, cmd = a.search.Input.Update(msg)
		// Update fuzzy matches as user types
		a.updateFuzzyMatches()
		return a, cmd
	}

	// Handle local filter mode (/ key)
	if a.mode == ModeFilter {
		switch msg.Type {
		case tea.KeyEsc:
			// Keep filter active, just close input
			a.search.FilterQuery = a.search.FilterInput.Value()
			a.applyFilter()
			a.mode = ModeNormal
			return a, nil
		case tea.KeyEnter:
			// Apply filter and close
			a.search.FilterQuery = a.search.FilterInput.Value()
			a.applyFilter()
			a.mode = ModeNormal
			return a, nil
		case tea.KeyBackspace:
			// If filter is empty and backspace, clear filter entirely
			if a.search.FilterInput.Value() == "" {
				a.search.FilterQuery = ""
				a.applyFilter()
			}
		}

		// Update filter input
		var cmd tea.Cmd
		a.search.FilterInput, cmd = a.search.FilterInput.Update(msg)
		// Live filter as user types
		a.search.FilterQuery = a.search.FilterInput.Value()
		a.applyFilter()
		return a, cmd
	}

	// Handle quick add URL input mode
	if a.mode == ModeQuickAdd {
		switch msg.Type {
		case tea.KeyEsc:
			a.mode = ModeNormal
			return a, nil
		case tea.KeyEnter:
			url := a.quickAdd.Input.Value()
			if url == "" {
				return a, nil
			}
			// Start AI call
			a.mode = ModeQuickAddLoading
			return a, a.callAICmd(url)
		}
		// Forward to input
		var cmd tea.Cmd
		a.quickAdd.Input, cmd = a.quickAdd.Input.Update(msg)
		return a, cmd
	}

	// Handle quick add loading mode (no user input)
	if a.mode == ModeQuickAddLoading {
		// Only allow Esc to cancel
		if msg.Type == tea.KeyEsc {
			a.mode = ModeNormal
			return a, nil
		}
		return a, nil
	}

	// Handle read later loading mode (no user input)
	if a.mode == ModeReadLaterLoading {
		// Only allow Esc to cancel
		if msg.Type == tea.KeyEsc {
			a.mode = ModeNormal
			a.readLaterURL = ""
			return a, nil
		}
		return a, nil
	}

	// Handle quick add confirmation mode
	if a.mode == ModeQuickAddConfirm {
		// Check if folder filter is focused (not title and not tags)
		folderFilterFocused := a.quickAdd.FilterInput.Focused()

		switch msg.Type {
		case tea.KeyEsc:
			a.mode = ModeNormal
			return a, nil
		case tea.KeyEnter:
			// Check if we should create a new folder (no filtered results and filter has text)
			if folderFilterFocused && len(a.quickAdd.FilteredFolders) == 0 && a.quickAdd.FilterInput.Value() != "" {
				// Enter folder creation mode
				a.quickAddCreateFolder.NewFolderName = a.quickAdd.FilterInput.Value()
				a.quickAddCreateFolder.ParentOptions = a.buildFolderPaths()
				a.quickAddCreateFolder.ParentIdx = 0
				a.mode = ModeQuickAddCreateFolder
				return a, nil
			}
			// Save the bookmark with edited values
			return a.submitQuickAdd()
		case tea.KeyTab:
			// Cycle through: title -> folder filter -> tags -> title
			if a.modal.TitleInput.Focused() {
				a.modal.TitleInput.Blur()
				a.quickAdd.FilterInput.Focus()
				return a, a.quickAdd.FilterInput.Focus()
			} else if folderFilterFocused {
				a.quickAdd.FilterInput.Blur()
				a.modal.TagsInput.Focus()
				return a, a.modal.TagsInput.Focus()
			} else if a.modal.TagsInput.Focused() {
				a.modal.TagsInput.Blur()
				a.modal.TitleInput.Focus()
				return a, a.modal.TitleInput.Focus()
			}
			return a, nil
		case tea.KeyUp, tea.KeyCtrlP:
			// Navigate folder picker up (when folder filter focused or no input focused)
			if folderFilterFocused || (!a.modal.TitleInput.Focused() && !a.modal.TagsInput.Focused()) {
				if a.quickAdd.FolderIdx > 0 {
					a.quickAdd.FolderIdx--
				} else if len(a.quickAdd.FilteredFolders) > 0 {
					a.quickAdd.FolderIdx = len(a.quickAdd.FilteredFolders) - 1
				}
			}
			return a, nil
		case tea.KeyDown, tea.KeyCtrlN:
			// Navigate folder picker down (when folder filter focused or no input focused)
			if folderFilterFocused || (!a.modal.TitleInput.Focused() && !a.modal.TagsInput.Focused()) {
				if len(a.quickAdd.FilteredFolders) > 0 {
					a.quickAdd.FolderIdx++
					if a.quickAdd.FolderIdx >= len(a.quickAdd.FilteredFolders) {
						a.quickAdd.FolderIdx = 0
					}
				}
			}
			return a, nil
		}

		// Handle j/k for folder navigation when folder filter is focused
		if msg.Type == tea.KeyRunes && folderFilterFocused {
			// Don't intercept j/k when filter has focus - let them type
		} else if msg.Type == tea.KeyRunes && !a.modal.TitleInput.Focused() && !a.modal.TagsInput.Focused() {
			switch string(msg.Runes) {
			case "j":
				if len(a.quickAdd.FilteredFolders) > 0 {
					a.quickAdd.FolderIdx++
					if a.quickAdd.FolderIdx >= len(a.quickAdd.FilteredFolders) {
						a.quickAdd.FolderIdx = 0
					}
				}
				return a, nil
			case "k":
				if a.quickAdd.FolderIdx > 0 {
					a.quickAdd.FolderIdx--
				} else if len(a.quickAdd.FilteredFolders) > 0 {
					a.quickAdd.FolderIdx = len(a.quickAdd.FilteredFolders) - 1
				}
				return a, nil
			}
		}

		// Forward to active text input
		var cmd tea.Cmd
		if a.modal.TitleInput.Focused() {
			a.modal.TitleInput, cmd = a.modal.TitleInput.Update(msg)
		} else if a.modal.TagsInput.Focused() {
			a.modal.TagsInput, cmd = a.modal.TagsInput.Update(msg)
		} else if folderFilterFocused {
			a.quickAdd.FilterInput, cmd = a.quickAdd.FilterInput.Update(msg)
			a.updateQuickAddFilter()
		}
		return a, cmd
	}

	// Handle quick add create folder mode (parent picker)
	if a.mode == ModeQuickAddCreateFolder {
		switch msg.Type {
		case tea.KeyEsc:
			// Cancel and return to confirm mode
			a.mode = ModeQuickAddConfirm
			return a, nil
		case tea.KeyEnter:
			// Create folder and return to confirm mode
			if len(a.quickAddCreateFolder.ParentOptions) > 0 {
				parentPath := a.quickAddCreateFolder.ParentOptions[a.quickAddCreateFolder.ParentIdx]
				var newFolderPath string
				if parentPath == "/" {
					newFolderPath = a.quickAddCreateFolder.NewFolderName
				} else {
					newFolderPath = parentPath + "/" + a.quickAddCreateFolder.NewFolderName
				}

				// Create the folder
				a.store.GetOrCreateFolderByPath(newFolderPath)
				a.saveStore()

				// Rebuild folder paths and select the new folder
				a.quickAdd.Folders = a.buildOrderedFolderPaths(a.browser.CurrentFolderID, "")
				a.quickAdd.FilteredFolders = a.quickAdd.Folders
				a.quickAdd.FilterInput.Reset()
				a.quickAdd.FolderIdx = a.findFolderIndex("/" + newFolderPath)

				a.mode = ModeQuickAddConfirm
				a.setStatus("Created folder: " + newFolderPath)
			}
			return a, nil
		case tea.KeyUp, tea.KeyCtrlP:
			if a.quickAddCreateFolder.ParentIdx > 0 {
				a.quickAddCreateFolder.ParentIdx--
			} else if len(a.quickAddCreateFolder.ParentOptions) > 0 {
				a.quickAddCreateFolder.ParentIdx = len(a.quickAddCreateFolder.ParentOptions) - 1
			}
			return a, nil
		case tea.KeyDown, tea.KeyCtrlN:
			if len(a.quickAddCreateFolder.ParentOptions) > 0 {
				a.quickAddCreateFolder.ParentIdx++
				if a.quickAddCreateFolder.ParentIdx >= len(a.quickAddCreateFolder.ParentOptions) {
					a.quickAddCreateFolder.ParentIdx = 0
				}
			}
			return a, nil
		}

		// Handle j/k for navigation
		if msg.Type == tea.KeyRunes {
			switch string(msg.Runes) {
			case "j":
				if len(a.quickAddCreateFolder.ParentOptions) > 0 {
					a.quickAddCreateFolder.ParentIdx++
					if a.quickAddCreateFolder.ParentIdx >= len(a.quickAddCreateFolder.ParentOptions) {
						a.quickAddCreateFolder.ParentIdx = 0
					}
				}
				return a, nil
			case "k":
				if a.quickAddCreateFolder.ParentIdx > 0 {
					a.quickAddCreateFolder.ParentIdx--
				} else if len(a.quickAddCreateFolder.ParentOptions) > 0 {
					a.quickAddCreateFolder.ParentIdx = len(a.quickAddCreateFolder.ParentOptions) - 1
				}
				return a, nil
			}
		}
		return a, nil
	}

	switch msg.Type {
	case tea.KeyEsc:
		// Cancel modal
		a.mode = ModeNormal
		return a, nil

	case tea.KeyEnter:
		// Submit modal
		return a.submitModal()
	}

	// Forward to text inputs
	var cmd tea.Cmd
	switch a.mode {
	case ModeAddBookmark, ModeEditBookmark:
		// Handle Tab to switch between inputs
		if msg.Type == tea.KeyTab {
			if a.modal.TitleInput.Focused() {
				a.modal.TitleInput.Blur()
				a.modal.URLInput.Focus()
				return a, a.modal.URLInput.Focus()
			} else {
				a.modal.URLInput.Blur()
				a.modal.TitleInput.Focus()
				return a, a.modal.TitleInput.Focus()
			}
		}

		if a.modal.TitleInput.Focused() {
			a.modal.TitleInput, cmd = a.modal.TitleInput.Update(msg)
		} else {
			a.modal.URLInput, cmd = a.modal.URLInput.Update(msg)
		}
	case ModeAddFolder, ModeEditFolder:
		a.modal.TitleInput, cmd = a.modal.TitleInput.Update(msg)
	case ModeEditTags:
		// Handle Tab for tag autocompletion
		if msg.Type == tea.KeyTab {
			if len(a.modal.TagSuggestions) > 0 {
				// Cycle through suggestions
				a.modal.TagSuggestionIdx++
				if a.modal.TagSuggestionIdx >= len(a.modal.TagSuggestions) {
					a.modal.TagSuggestionIdx = 0
				}
				// Insert selected suggestion
				a.insertTagSuggestion()
				return a, nil
			}
		}

		// Handle up/down for suggestion navigation
		if msg.Type == tea.KeyUp && len(a.modal.TagSuggestions) > 0 {
			if a.modal.TagSuggestionIdx > 0 {
				a.modal.TagSuggestionIdx--
			} else {
				a.modal.TagSuggestionIdx = len(a.modal.TagSuggestions) - 1
			}
			return a, nil
		}
		if msg.Type == tea.KeyDown && len(a.modal.TagSuggestions) > 0 {
			a.modal.TagSuggestionIdx++
			if a.modal.TagSuggestionIdx >= len(a.modal.TagSuggestions) {
				a.modal.TagSuggestionIdx = 0
			}
			return a, nil
		}

		// Update input and then update suggestions
		a.modal.TagsInput, cmd = a.modal.TagsInput.Update(msg)
		a.updateTagSuggestions()
	}

	return a, cmd
}

// submitModal handles submission of the current modal.
func (a App) submitModal() (tea.Model, tea.Cmd) {
	switch a.mode {
	case ModeAddFolder:
		name := a.modal.TitleInput.Value()
		if name == "" {
			// Don't submit with empty name
			return a, nil
		}

		// Create and add the folder
		newFolder := model.NewFolder(model.NewFolderParams{
			Name:     name,
			ParentID: a.browser.CurrentFolderID,
		})
		a.store.AddFolder(newFolder)
		a.saveStore()
		a.refreshItems()
		a.mode = ModeNormal
		a.setStatus("Folder added: " + name)
		return a, nil

	case ModeAddBookmark:
		title := a.modal.TitleInput.Value()
		url := a.modal.URLInput.Value()
		if title == "" || url == "" {
			// Don't submit with empty fields
			return a, nil
		}

		// Create and add the bookmark
		newBookmark := model.NewBookmark(model.NewBookmarkParams{
			Title:    title,
			URL:      url,
			FolderID: a.browser.CurrentFolderID,
			Tags:     []string{},
		})
		a.store.AddBookmark(newBookmark)
		a.saveStore()
		a.refreshItems()
		a.mode = ModeNormal
		a.setStatus("Bookmark added: " + title)
		return a, nil

	case ModeEditFolder:
		name := a.modal.TitleInput.Value()
		if name == "" {
			// Don't submit with empty name
			return a, nil
		}

		// Find and update the folder
		folder := a.store.GetFolderByID(a.modal.EditItemID)
		if folder != nil {
			folder.Name = name
		}
		a.saveStore()
		a.refreshItems()
		a.mode = ModeNormal
		return a, nil

	case ModeEditBookmark:
		title := a.modal.TitleInput.Value()
		url := a.modal.URLInput.Value()
		if title == "" || url == "" {
			// Don't submit with empty fields
			return a, nil
		}

		// Find and update the bookmark
		bookmark := a.store.GetBookmarkByID(a.modal.EditItemID)
		if bookmark != nil {
			bookmark.Title = title
			bookmark.URL = url
		}
		a.saveStore()
		a.refreshItems()
		a.mode = ModeNormal
		return a, nil

	case ModeEditTags:
		// Parse comma-separated tags
		tagsStr := a.modal.TagsInput.Value()
		var tags []string
		if tagsStr != "" {
			for _, tag := range strings.Split(tagsStr, ",") {
				tag = strings.TrimSpace(tag)
				if tag != "" {
					tags = append(tags, tag)
				}
			}
		}

		// Find and update the bookmark
		bookmark := a.store.GetBookmarkByID(a.modal.EditItemID)
		if bookmark != nil {
			bookmark.Tags = tags
		}
		a.saveStore()
		a.refreshItems()
		a.mode = ModeNormal
		return a, nil
	}

	return a, nil
}

// submitQuickAdd saves the bookmark from AI quick add confirmation.
func (a App) submitQuickAdd() (tea.Model, tea.Cmd) {
	title := a.modal.TitleInput.Value()
	url := a.quickAdd.Input.Value()

	if title == "" || url == "" {
		return a, nil
	}

	// Parse tags
	var tags []string
	tagsStr := a.modal.TagsInput.Value()
	if tagsStr != "" {
		for _, tag := range strings.Split(tagsStr, ",") {
			tag = strings.TrimSpace(tag)
			if tag != "" {
				tags = append(tags, tag)
			}
		}
	}

	// Get or create the selected folder
	var folderID *string
	if a.quickAdd.FolderIdx >= 0 && a.quickAdd.FolderIdx < len(a.quickAdd.FilteredFolders) {
		folderPath := a.quickAdd.FilteredFolders[a.quickAdd.FolderIdx]
		if folderPath != "/" {
			folder, _ := a.store.GetOrCreateFolderByPath(folderPath)
			if folder != nil {
				folderID = &folder.ID
			}
		}
	}

	// Create and add the bookmark
	newBookmark := model.NewBookmark(model.NewBookmarkParams{
		Title:    title,
		URL:      url,
		FolderID: folderID,
		Tags:     tags,
	})
	a.store.AddBookmark(newBookmark)
	a.saveStore()
	a.refreshItems()
	a.mode = ModeNormal
	a.setStatus("Bookmark added: " + title)
	return a, nil
}

// toggleSelectCurrentItem toggles selection on the current item.
func (a *App) toggleSelectCurrentItem() {
	displayItems := a.getDisplayItems()
	if len(displayItems) == 0 || a.browser.Cursor >= len(displayItems) {
		return
	}
	item := displayItems[a.browser.Cursor]
	a.selection.Toggle(item.ID())

	// Exit visual mode when toggling with v
	if a.selection.VisualMode {
		a.selection.VisualMode = false
		a.selection.AnchorIndex = -1
	}
}

// enterVisualMode starts visual line mode at current cursor.
func (a *App) enterVisualMode() {
	displayItems := a.getDisplayItems()
	if len(displayItems) == 0 || a.browser.Cursor >= len(displayItems) {
		return
	}

	a.selection.VisualMode = true
	a.selection.AnchorIndex = a.browser.Cursor

	// Select the current item
	item := displayItems[a.browser.Cursor]
	a.selection.Selected[item.ID()] = true
}

// updateVisualSelection updates selection when cursor moves in visual mode.
func (a *App) updateVisualSelection() {
	if !a.selection.VisualMode {
		return
	}

	displayItems := a.getDisplayItems()
	if len(displayItems) == 0 {
		return
	}

	// Clear existing selection and re-select range
	a.selection.Selected = make(map[string]bool)

	start := a.selection.AnchorIndex
	end := a.browser.Cursor
	if start > end {
		start, end = end, start
	}

	for i := start; i <= end && i < len(displayItems); i++ {
		item := displayItems[i]
		a.selection.Selected[item.ID()] = true
	}
}

// clearSelection clears all selection state.
func (a *App) clearSelection() {
	a.selection.Reset()
}

// yankCurrentItem copies the current item (or selected items) to the yank buffer.
func (a *App) yankCurrentItem() {
	displayItems := a.getDisplayItems()
	if len(displayItems) == 0 || a.browser.Cursor >= len(displayItems) {
		return
	}

	// If selection exists, yank all selected items
	if a.selection.HasSelection() {
		a.yankedItems = nil
		for _, item := range displayItems {
			if a.selection.IsSelected(item.ID()) {
				a.yankedItems = append(a.yankedItems, item)
			}
		}
		a.setStatus("Yanked " + strconv.Itoa(len(a.yankedItems)) + " items")
		a.clearSelection()
		return
	}

	// Single item yank
	item := displayItems[a.browser.Cursor]
	a.yankedItems = []Item{item}
	a.setStatus("Yanked: " + item.Title())
}

// cutCurrentItem copies the current item (or selected items) to yank buffer and deletes it.
// Shows confirmation dialog if confirmDelete is enabled or if batch cutting.
func (a *App) cutCurrentItem() {
	displayItems := a.getDisplayItems()
	if len(displayItems) == 0 || a.browser.Cursor >= len(displayItems) {
		return
	}

	a.modal.CutMode = true

	// If selection exists, batch cut
	if a.selection.HasSelection() {
		var itemsToCut []Item
		for _, item := range displayItems {
			if a.selection.IsSelected(item.ID()) {
				itemsToCut = append(itemsToCut, item)
			}
		}
		// Always confirm batch operations
		a.modal.DeleteItems = itemsToCut
		a.mode = ModeConfirmDelete
		return
	}

	// Single item cut
	item := displayItems[a.browser.Cursor]

	// Show confirmation if enabled
	if a.confirmDelete {
		if item.IsFolder() {
			a.modal.EditItemID = item.Folder.ID
		} else {
			a.modal.EditItemID = item.Bookmark.ID
		}
		a.mode = ModeConfirmDelete
		return
	}

	// No confirmation - cut immediately
	a.yankedItems = []Item{item}
	if item.IsFolder() {
		a.store.RemoveFolderByID(item.Folder.ID)
		a.setStatus("Cut: " + item.Folder.Name)
	} else {
		a.store.RemoveBookmarkByID(item.Bookmark.ID)
		a.setStatus("Cut: " + item.Bookmark.Title)
	}
	a.saveStore()

	a.refreshItems()
	if a.browser.Cursor >= len(a.browser.Items) && a.browser.Cursor > 0 {
		a.browser.Cursor--
	}
}

// deleteCurrentItem deletes the current item (or selected items) without copying to yank buffer.
// Shows confirmation dialog if confirmDelete is enabled or if batch deleting.
func (a *App) deleteCurrentItem() {
	displayItems := a.getDisplayItems()
	if len(displayItems) == 0 || a.browser.Cursor >= len(displayItems) {
		return
	}

	a.modal.CutMode = false

	// If selection exists, batch delete
	if a.selection.HasSelection() {
		var itemsToDelete []Item
		for _, item := range displayItems {
			if a.selection.IsSelected(item.ID()) {
				itemsToDelete = append(itemsToDelete, item)
			}
		}
		// Always confirm batch operations
		a.modal.DeleteItems = itemsToDelete
		a.mode = ModeConfirmDelete
		return
	}

	// Single item delete
	item := displayItems[a.browser.Cursor]

	// Show confirmation if enabled
	if a.confirmDelete {
		if item.IsFolder() {
			a.modal.EditItemID = item.Folder.ID
		} else {
			a.modal.EditItemID = item.Bookmark.ID
		}
		a.mode = ModeConfirmDelete
		return
	}

	// No confirmation - delete immediately
	if item.IsFolder() {
		a.store.RemoveFolderByID(item.Folder.ID)
		a.setStatus("Deleted: " + item.Folder.Name)
	} else {
		a.store.RemoveBookmarkByID(item.Bookmark.ID)
		a.setStatus("Deleted: " + item.Bookmark.Title)
	}
	a.saveStore()

	a.refreshItems()
	if a.browser.Cursor >= len(a.browser.Items) && a.browser.Cursor > 0 {
		a.browser.Cursor--
	}
}

// confirmDeleteItem performs the actual deletion after confirmation.
// Handles both single items and batch operations.
func (a *App) confirmDeleteItem() {
	// Handle batch delete/cut
	if len(a.modal.DeleteItems) > 0 {
		if a.modal.CutMode {
			a.yankedItems = make([]Item, len(a.modal.DeleteItems))
			copy(a.yankedItems, a.modal.DeleteItems)
		}

		for _, item := range a.modal.DeleteItems {
			if item.IsFolder() {
				a.store.RemoveFolderByID(item.Folder.ID)
			} else {
				a.store.RemoveBookmarkByID(item.Bookmark.ID)
			}
		}
		a.saveStore()

		count := len(a.modal.DeleteItems)
		a.modal.DeleteItems = nil
		a.clearSelection()

		a.refreshItems()
		if a.browser.Cursor >= len(a.browser.Items) && a.browser.Cursor > 0 {
			a.browser.Cursor = len(a.browser.Items) - 1
		}
		if a.browser.Cursor < 0 {
			a.browser.Cursor = 0
		}

		if a.modal.CutMode {
			a.setStatus("Cut " + strconv.Itoa(count) + " items")
		} else {
			a.setStatus("Deleted " + strconv.Itoa(count) + " items")
		}
		return
	}

	// Single item delete/cut
	var title string

	// Try as folder first
	folder := a.store.GetFolderByID(a.modal.EditItemID)
	if folder != nil {
		title = folder.Name
		if a.modal.CutMode {
			// Make a copy before deleting
			folderCopy := *folder
			item := Item{Kind: ItemFolder, Folder: &folderCopy}
			a.yankedItems = []Item{item}
		}
		a.store.RemoveFolderByID(a.modal.EditItemID)
	} else {
		// Try as bookmark
		bookmark := a.store.GetBookmarkByID(a.modal.EditItemID)
		if bookmark == nil {
			return
		}
		title = bookmark.Title
		if a.modal.CutMode {
			// Make a copy before deleting
			bookmarkCopy := *bookmark
			item := Item{Kind: ItemBookmark, Bookmark: &bookmarkCopy}
			a.yankedItems = []Item{item}
		}
		a.store.RemoveBookmarkByID(a.modal.EditItemID)
	}
	a.saveStore()

	// Refresh items and adjust cursor
	a.refreshItems()
	if a.browser.Cursor >= len(a.browser.Items) && a.browser.Cursor > 0 {
		a.browser.Cursor--
	}

	if a.modal.CutMode {
		a.setStatus("Cut: " + title)
	} else {
		a.setStatus("Deleted: " + title)
	}
}

// executeMoveItem moves the item(s) to the selected folder.
func (a *App) executeMoveItem() {
	if a.move.FolderIdx < 0 || a.move.FolderIdx >= len(a.move.FilteredFolders) {
		return
	}

	if len(a.move.ItemsToMove) == 0 {
		return
	}

	targetPath := a.move.FilteredFolders[a.move.FolderIdx]

	// Resolve target folder ID (nil for root "/")
	var targetFolderID *string
	if targetPath != "/" {
		targetFolder := a.store.GetFolderByPath(targetPath)
		if targetFolder != nil {
			targetFolderID = &targetFolder.ID
		}
	}

	movedCount := 0
	for _, item := range a.move.ItemsToMove {
		if item.IsFolder() {
			folder := a.store.GetFolderByID(item.Folder.ID)
			if folder == nil {
				continue
			}

			// Prevent moving folder into itself or its descendants
			if targetFolderID != nil && a.isFolderDescendant(item.Folder.ID, *targetFolderID) {
				continue // Skip this one, don't abort the whole operation
			}

			folder.ParentID = targetFolderID
			movedCount++
		} else {
			bookmark := a.store.GetBookmarkByID(item.Bookmark.ID)
			if bookmark == nil {
				continue
			}

			bookmark.FolderID = targetFolderID
			movedCount++
		}
	}

	a.saveStore()
	a.clearSelection()
	a.move.ItemsToMove = nil
	a.refreshItems()
	a.refreshPinnedItems()

	// Adjust cursor if needed
	if a.browser.Cursor >= len(a.browser.Items) && a.browser.Cursor > 0 {
		a.browser.Cursor = len(a.browser.Items) - 1
	}
	if a.browser.Cursor < 0 {
		a.browser.Cursor = 0
	}

	if movedCount == 1 {
		a.setStatus("Moved item → " + targetPath)
	} else {
		a.setStatus("Moved " + strconv.Itoa(movedCount) + " items → " + targetPath)
	}
}

// isFolderDescendant checks if targetID is the same as or a descendant of folderID.
func (a *App) isFolderDescendant(folderID, targetID string) bool {
	if folderID == targetID {
		return true
	}

	// Check all descendants of folderID
	for _, f := range a.store.Folders {
		if f.ParentID != nil && *f.ParentID == folderID {
			if a.isFolderDescendant(f.ID, targetID) {
				return true
			}
		}
	}

	return false
}

// pasteItem pastes the yanked item(s) before or after the cursor.
func (a *App) pasteItem(before bool) {
	if len(a.yankedItems) == 0 {
		a.setStatus("Nothing to paste")
		return
	}

	// Calculate insert position
	insertIdx := a.browser.Cursor
	if !before && len(a.browser.Items) > 0 {
		insertIdx = a.browser.Cursor + 1
	}

	// Count folders in current view
	folderCount := 0
	for _, item := range a.browser.Items {
		if item.IsFolder() {
			folderCount++
		}
	}

	// Paste all yanked items
	pastedCount := 0
	for _, yankedItem := range a.yankedItems {
		if yankedItem.IsFolder() {
			// Create a copy with new ID
			newFolder := model.NewFolder(model.NewFolderParams{
				Name:     yankedItem.Folder.Name,
				ParentID: a.browser.CurrentFolderID,
			})

			// If pasting among folders
			if insertIdx <= folderCount {
				a.store.InsertFolderAt(newFolder, insertIdx)
				insertIdx++ // next item goes after this one
				folderCount++
			} else {
				a.store.AddFolder(newFolder)
			}
		} else {
			// Create a copy with new ID
			newBookmark := model.NewBookmark(model.NewBookmarkParams{
				Title:    yankedItem.Bookmark.Title,
				URL:      yankedItem.Bookmark.URL,
				FolderID: a.browser.CurrentFolderID,
				Tags:     yankedItem.Bookmark.Tags,
			})

			// Bookmarks come after folders in the view
			bookmarkIdx := insertIdx - folderCount
			if bookmarkIdx < 0 {
				bookmarkIdx = 0
			}

			a.store.InsertBookmarkAt(newBookmark, bookmarkIdx)
			insertIdx++ // next item goes after this one
		}
		pastedCount++
	}

	a.saveStore()
	a.refreshItems()
	if pastedCount == 1 {
		a.setStatus("Pasted: " + a.yankedItems[0].Title())
	} else {
		a.setStatus("Pasted " + strconv.Itoa(pastedCount) + " items")
	}
}

// openURLCmd returns a tea.Cmd that opens a URL in the default browser.
func openURLCmd(url string) tea.Cmd {
	return func() tea.Msg {
		var openCmd *exec.Cmd
		switch runtime.GOOS {
		case "darwin":
			openCmd = exec.Command("open", url)
		case "linux":
			openCmd = exec.Command("xdg-open", url)
		case "windows":
			openCmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
		}
		if openCmd != nil {
			if err := openCmd.Start(); err != nil {
				return openURLErrorMsg{err: err}
			}
		}
		return nil
	}
}

// openBookmark opens the selected bookmark URL in default browser.
func (a App) openBookmark() (tea.Model, tea.Cmd) {
	displayItems := a.getDisplayItems()
	if len(displayItems) == 0 || a.browser.Cursor >= len(displayItems) {
		return a, nil
	}

	item := displayItems[a.browser.Cursor]
	if item.IsFolder() {
		return a, nil
	}

	// Update visitedAt timestamp
	bookmark := a.store.GetBookmarkByID(item.Bookmark.ID)
	if bookmark != nil {
		now := time.Now()
		bookmark.VisitedAt = &now
		a.refreshItems()
	}

	return a, openURLCmd(item.Bookmark.URL)
}

// clipboardSuccessMsg is sent when clipboard write succeeds.
type clipboardSuccessMsg struct{}

// yankURLToClipboard copies the selected bookmark URL to system clipboard.
func (a App) yankURLToClipboard() (tea.Model, tea.Cmd) {
	displayItems := a.getDisplayItems()
	if len(displayItems) == 0 || a.browser.Cursor >= len(displayItems) {
		return a, nil
	}

	item := displayItems[a.browser.Cursor]
	if item.IsFolder() {
		return a, nil
	}

	url := item.Bookmark.URL
	cmd := func() tea.Msg {
		if err := clipboard.WriteAll(url); err != nil {
			return clipboardErrorMsg{err: err}
		}
		return clipboardSuccessMsg{}
	}

	return a, cmd
}

// View implements tea.Model.
func (a App) View() string {
	return a.renderView()
}

// applyFilter filters current items based on filterQuery using fuzzy matching.
func (a *App) applyFilter() {
	if a.search.FilterQuery == "" {
		a.search.FilteredItems = nil
		return
	}

	// Use fuzzy matching on current folder items
	matches := fuzzy.FindFrom(a.search.FilterQuery, itemStrings(a.browser.Items))
	a.search.FilteredItems = make([]Item, len(matches))
	for i, m := range matches {
		a.search.FilteredItems[i] = a.browser.Items[m.Index]
	}

	// Reset cursor if out of bounds
	displayItems := a.getDisplayItems()
	if a.browser.Cursor >= len(displayItems) {
		a.browser.Cursor = 0
	}
}

// getDisplayItems returns filtered items if filter is active, otherwise all items.
func (a *App) getDisplayItems() []Item {
	if a.search.FilterQuery != "" && a.search.FilteredItems != nil {
		return a.search.FilteredItems
	}
	return a.browser.Items
}

// buildFolderPaths returns all folder paths in the store.
func (a *App) buildFolderPaths() []string {
	paths := []string{"/"}
	a.collectFolderPaths(&paths, nil, "")
	return paths
}

// collectFolderPaths recursively collects all folder paths.
func (a *App) collectFolderPaths(paths *[]string, parentID *string, prefix string) {
	folders := a.store.GetFoldersInFolder(parentID)
	for _, folder := range folders {
		path := prefix + "/" + folder.Name
		*paths = append(*paths, path)
		a.collectFolderPaths(paths, &folder.ID, path)
	}
}

// buildOrderedFolderPaths returns folder paths with smart ordering:
// 1. Current folder (if in one)
// 2. AI suggestion (if different from current)
// 3. Root
// 4. All other folders alphabetically
func (a *App) buildOrderedFolderPaths(currentFolderID *string, aiSuggestion string) []string {
	paths := []string{}

	// 1. Current folder (if in one)
	if currentFolderID != nil {
		currentPath := a.store.GetFolderPath(currentFolderID)
		paths = append(paths, currentPath)
	}

	// 2. AI suggestion (if different from current)
	if aiSuggestion != "" {
		aiPath := "/" + strings.TrimPrefix(aiSuggestion, "/")
		alreadyIncluded := len(paths) > 0 && paths[0] == aiPath
		if !alreadyIncluded {
			paths = append(paths, aiPath)
		}
	}

	// 3. Root (if not already included)
	hasRoot := false
	for _, p := range paths {
		if p == "/" {
			hasRoot = true
			break
		}
	}
	if !hasRoot {
		paths = append(paths, "/")
	}

	// 4. All other folders alphabetically
	allFolders := a.buildFolderPaths()
	for _, folder := range allFolders {
		included := false
		for _, p := range paths {
			if p == folder {
				included = true
				break
			}
		}
		if !included {
			paths = append(paths, folder)
		}
	}

	return paths
}

// findFolderIndex finds the index of a folder path in quickAdd FilteredFolders.
func (a *App) findFolderIndex(path string) int {
	for i, p := range a.quickAdd.FilteredFolders {
		if p == path {
			return i
		}
	}
	// Default to first item if not found
	return 0
}

// findMoveFolderIndex finds the index of a folder path in moveFilteredFolders.
func (a *App) findMoveFolderIndex(path string) int {
	for i, p := range a.move.FilteredFolders {
		if p == path {
			return i
		}
	}
	// Default to first item if not found
	return 0
}

// updateMoveFilter filters moveFolders based on the filter input.
func (a *App) updateMoveFilter() {
	query := strings.ToLower(a.move.FilterInput.Value())
	if query == "" {
		a.move.FilteredFolders = a.move.Folders
	} else {
		a.move.FilteredFolders = nil
		for _, folder := range a.move.Folders {
			if strings.Contains(strings.ToLower(folder), query) {
				a.move.FilteredFolders = append(a.move.FilteredFolders, folder)
			}
		}
	}
	// Reset index if out of bounds
	if a.move.FolderIdx >= len(a.move.FilteredFolders) {
		a.move.FolderIdx = 0
	}
}

// updateQuickAddFilter filters quickAdd folders based on the filter input.
func (a *App) updateQuickAddFilter() {
	query := strings.ToLower(a.quickAdd.FilterInput.Value())
	if query == "" {
		a.quickAdd.FilteredFolders = a.quickAdd.Folders
	} else {
		a.quickAdd.FilteredFolders = nil
		for _, folder := range a.quickAdd.Folders {
			if strings.Contains(strings.ToLower(folder), query) {
				a.quickAdd.FilteredFolders = append(a.quickAdd.FilteredFolders, folder)
			}
		}
	}
	// Reset index if out of bounds
	if a.quickAdd.FolderIdx >= len(a.quickAdd.FilteredFolders) {
		a.quickAdd.FolderIdx = 0
	}
}

// callAICmd returns a tea.Cmd that calls the AI API.
func (a *App) callAICmd(url string) tea.Cmd {
	return func() tea.Msg {
		client, err := ai.NewClient()
		if err != nil {
			return aiResponseMsg{err: err}
		}

		context := ai.BuildContext(a.store)
		response, err := client.SuggestBookmark(url, context)
		return aiResponseMsg{response: response, err: err}
	}
}

// startCullCmd returns a tea.Cmd that starts the URL cull check.
func (a *App) startCullCmd() tea.Cmd {
	bookmarks := make([]model.Bookmark, len(a.store.Bookmarks))
	copy(bookmarks, a.store.Bookmarks)

	excludeDomains := a.config.CullExcludeDomains

	// Reset the atomic progress counter
	atomic.StoreInt64(&cullProgressCounter, 0)

	// Start both the cull operation and the ticker
	return tea.Batch(
		// Cull operation
		func() tea.Msg {
			onProgress := func(completed, total int) {
				atomic.StoreInt64(&cullProgressCounter, int64(completed))
			}
			results := culler.CheckURLs(bookmarks, 10, 10*time.Second, excludeDomains, onProgress)
			return cullCompleteMsg{results: results}
		},
		// Start the ticker to update UI
		cullTickCmd(),
	)
}

// cullTickCmd returns a command that ticks every 100ms to update progress.
func cullTickCmd() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return cullTickMsg{}
	})
}

// organizeTickCmd returns a command that ticks every 100ms to update progress.
func organizeTickCmd() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return organizeTickMsg{}
	})
}

// cullCachePath returns the path to the cull cache file.
func cullCachePath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".config", "bm", "cull-cache.json"), nil
}

// saveCullCache saves cull results to disk.
func (a *App) saveCullCache(results []culler.Result) error {
	path, err := cullCachePath()
	if err != nil {
		return err
	}

	// Convert to serializable format
	cacheResults := make([]CullCacheResult, 0, len(results))
	for _, r := range results {
		if r.Status == culler.Healthy {
			continue // Only cache problematic results
		}
		cacheResults = append(cacheResults, CullCacheResult{
			BookmarkID: r.Bookmark.ID,
			Status:     int(r.Status),
			StatusCode: r.StatusCode,
			Error:      r.Error,
		})
	}

	cache := CullCache{
		Timestamp: time.Now(),
		Results:   cacheResults,
	}

	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// loadCullCache loads cull results from disk and matches with current bookmarks.
func (a *App) loadCullCache() ([]culler.Result, time.Time, error) {
	path, err := cullCachePath()
	if err != nil {
		return nil, time.Time{}, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, time.Time{}, err
	}

	var cache CullCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, time.Time{}, err
	}

	// Build bookmark ID map for fast lookup
	bookmarkMap := make(map[string]*model.Bookmark)
	for i := range a.store.Bookmarks {
		bookmarkMap[a.store.Bookmarks[i].ID] = &a.store.Bookmarks[i]
	}

	// Convert back to culler.Result, skipping missing bookmarks
	results := make([]culler.Result, 0, len(cache.Results))
	for _, cr := range cache.Results {
		bookmark, exists := bookmarkMap[cr.BookmarkID]
		if !exists {
			continue // Bookmark was deleted
		}
		results = append(results, culler.Result{
			Bookmark:   bookmark,
			Status:     culler.Status(cr.Status),
			StatusCode: cr.StatusCode,
			Error:      cr.Error,
		})
	}

	return results, cache.Timestamp, nil
}

// checkCullCache checks if a cache file exists and updates state.
func (a *App) checkCullCache() {
	path, err := cullCachePath()
	if err != nil {
		a.cull.HasCache = false
		return
	}

	info, err := os.Stat(path)
	if err != nil {
		a.cull.HasCache = false
		return
	}

	a.cull.HasCache = true
	a.cull.CacheTime = info.ModTime()
}

// countCachedProblems returns the count of problematic bookmarks in cache.
func (a *App) countCachedProblems() int {
	results, _, err := a.loadCullCache()
	if err != nil {
		return 0
	}
	return len(results)
}

// organizeCachePath returns the path to the organize cache file.
func organizeCachePath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".config", "bm", "organize-cache.json"), nil
}

// saveOrganizeCache saves organize suggestions to disk.
func (a *App) saveOrganizeCache(suggestions []OrganizeSuggestion) error {
	path, err := organizeCachePath()
	if err != nil {
		return err
	}

	// Convert to serializable format
	cacheResults := make([]OrganizeCacheResult, 0, len(suggestions))
	for _, s := range suggestions {
		var itemID string
		var isFolder bool
		if s.Item.IsFolder() {
			itemID = s.Item.Folder.ID
			isFolder = true
		} else {
			itemID = s.Item.Bookmark.ID
			isFolder = false
		}
		cacheResults = append(cacheResults, OrganizeCacheResult{
			ItemID:        itemID,
			IsFolder:      isFolder,
			CurrentPath:   s.CurrentPath,
			SuggestedPath: s.SuggestedPath,
			IsNewFolder:   s.IsNewFolder,
			CurrentTags:   s.CurrentTags,
			SuggestedTags: s.SuggestedTags,
		})
	}

	cache := OrganizeCache{
		Timestamp: time.Now(),
		Results:   cacheResults,
	}

	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// loadOrganizeCache loads organize suggestions from disk and matches with current items.
func (a *App) loadOrganizeCache() ([]OrganizeSuggestion, time.Time, error) {
	path, err := organizeCachePath()
	if err != nil {
		return nil, time.Time{}, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, time.Time{}, err
	}

	var cache OrganizeCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, time.Time{}, err
	}

	// Build maps for fast lookup
	bookmarkMap := make(map[string]*model.Bookmark)
	for i := range a.store.Bookmarks {
		bookmarkMap[a.store.Bookmarks[i].ID] = &a.store.Bookmarks[i]
	}
	folderMap := make(map[string]*model.Folder)
	for i := range a.store.Folders {
		folderMap[a.store.Folders[i].ID] = &a.store.Folders[i]
	}

	// Convert back to OrganizeSuggestion, skipping missing items
	suggestions := make([]OrganizeSuggestion, 0, len(cache.Results))
	for _, cr := range cache.Results {
		var item Item
		if cr.IsFolder {
			folder, exists := folderMap[cr.ItemID]
			if !exists {
				continue // Folder was deleted
			}
			item = Item{Kind: ItemFolder, Folder: folder}
		} else {
			bookmark, exists := bookmarkMap[cr.ItemID]
			if !exists {
				continue // Bookmark was deleted
			}
			item = Item{Kind: ItemBookmark, Bookmark: bookmark}
		}
		suggestions = append(suggestions, OrganizeSuggestion{
			Item:          item,
			CurrentPath:   cr.CurrentPath,
			SuggestedPath: cr.SuggestedPath,
			IsNewFolder:   cr.IsNewFolder,
			CurrentTags:   cr.CurrentTags,
			SuggestedTags: cr.SuggestedTags,
			Processed:     false,
		})
	}

	return suggestions, cache.Timestamp, nil
}

// checkOrganizeCache checks if a cache file exists and updates state.
func (a *App) checkOrganizeCache() {
	path, err := organizeCachePath()
	if err != nil {
		a.organize.HasCache = false
		return
	}

	info, err := os.Stat(path)
	if err != nil {
		a.organize.HasCache = false
		return
	}

	a.organize.HasCache = true
	a.organize.CacheTime = info.ModTime()
}

// countCachedOrganizeSuggestions returns the count of suggestions in cache.
func (a *App) countCachedOrganizeSuggestions() int {
	suggestions, _, err := a.loadOrganizeCache()
	if err != nil {
		return 0
	}
	return len(suggestions)
}

// groupCullResults groups cull results by status and error type.
func (a *App) groupCullResults(results []culler.Result) []CullGroup {
	// Map to collect results by group key
	groupMap := make(map[string]*CullGroup)

	for _, r := range results {
		if r.Status == culler.Healthy {
			continue // Skip healthy items
		}

		var key, label, desc string

		switch r.Status {
		case culler.Dead:
			key = "dead"
			label = "DEAD"
			desc = "404/410 responses"
		case culler.Unreachable:
			// Group by error type
			key = "unreachable:" + r.Error
			label = r.Error
			desc = "Could not reach"
		}

		group, exists := groupMap[key]
		if !exists {
			group = &CullGroup{
				Label:       label,
				Description: desc,
				Status:      r.Status,
				Error:       r.Error,
				Results:     []culler.Result{},
			}
			groupMap[key] = group
		}
		group.Results = append(group.Results, r)
	}

	// Convert map to slice and sort (DEAD first, then by count)
	groups := make([]CullGroup, 0, len(groupMap))
	for _, g := range groupMap {
		groups = append(groups, *g)
	}

	sort.Slice(groups, func(i, j int) bool {
		// DEAD first
		if groups[i].Status == culler.Dead && groups[j].Status != culler.Dead {
			return true
		}
		if groups[i].Status != culler.Dead && groups[j].Status == culler.Dead {
			return false
		}
		// Then by count (descending)
		return len(groups[i].Results) > len(groups[j].Results)
	})

	return groups
}

// cullDeleteGroup deletes all bookmarks in the current cull group.
func (a *App) cullDeleteGroup() (tea.Model, tea.Cmd) {
	group := a.cull.CurrentGroup()
	if group == nil {
		return a, nil
	}

	count := 0
	for _, r := range group.Results {
		a.store.RemoveBookmarkByID(r.Bookmark.ID)
		count++
	}

	a.saveStore()
	a.refreshItems()
	a.refreshPinnedItems()

	// Remove the group from the list
	if len(a.cull.Groups) > 0 {
		idx := a.cull.GroupCursor
		a.cull.Groups = append(a.cull.Groups[:idx], a.cull.Groups[idx+1:]...)
	}

	// Adjust cursor if needed
	if a.cull.GroupCursor >= len(a.cull.Groups) && a.cull.GroupCursor > 0 {
		a.cull.GroupCursor--
	}

	// If no more groups, return to normal mode
	if len(a.cull.Groups) == 0 {
		a.cull.Reset()
		a.mode = ModeNormal
		cmd := a.setMessage(MessageSuccess, "Deleted "+strconv.Itoa(count)+" bookmarks. Cull complete!")
		return a, cmd
	}

	cmd := a.setMessage(MessageSuccess, "Deleted "+strconv.Itoa(count)+" bookmarks")
	return a, cmd
}

// cullDeleteItem deletes the current bookmark in cull inspect mode.
func (a *App) cullDeleteItem() (tea.Model, tea.Cmd) {
	group := a.cull.CurrentGroup()
	result := a.cull.CurrentItem()
	if group == nil || result == nil {
		return a, nil
	}

	title := result.Bookmark.Title
	a.store.RemoveBookmarkByID(result.Bookmark.ID)
	a.saveStore()
	a.refreshItems()
	a.refreshPinnedItems()

	// Remove from group results
	idx := a.cull.ItemCursor
	group.Results = append(group.Results[:idx], group.Results[idx+1:]...)

	// Adjust cursor
	if a.cull.ItemCursor >= len(group.Results) && a.cull.ItemCursor > 0 {
		a.cull.ItemCursor--
	}

	// If group is empty, remove it and go back to results
	if len(group.Results) == 0 {
		groupIdx := a.cull.GroupCursor
		a.cull.Groups = append(a.cull.Groups[:groupIdx], a.cull.Groups[groupIdx+1:]...)

		if a.cull.GroupCursor >= len(a.cull.Groups) && a.cull.GroupCursor > 0 {
			a.cull.GroupCursor--
		}

		if len(a.cull.Groups) == 0 {
			a.cull.Reset()
			a.mode = ModeNormal
			cmd := a.setMessage(MessageSuccess, "Deleted: "+title+". Cull complete!")
			return a, cmd
		}

		a.mode = ModeCullResults
	}

	cmd := a.setMessage(MessageSuccess, "Deleted: "+title)
	return a, cmd
}

// cullOpenItem opens the current bookmark URL in the browser.
func (a *App) cullOpenItem() (tea.Model, tea.Cmd) {
	result := a.cull.CurrentItem()
	if result == nil {
		return a, nil
	}
	return a, openURLCmd(result.Bookmark.URL)
}

// cullEditItem switches to edit mode for the current cull item.
func (a *App) cullEditItem() (tea.Model, tea.Cmd) {
	result := a.cull.CurrentItem()
	if result == nil {
		return a, nil
	}

	a.mode = ModeEditBookmark
	a.modal.EditItemID = result.Bookmark.ID
	a.modal.TitleInput.Reset()
	a.modal.TitleInput.SetValue(result.Bookmark.Title)
	a.modal.URLInput.Reset()
	a.modal.URLInput.SetValue(result.Bookmark.URL)
	a.modal.TitleInput.Focus()
	return a, a.modal.TitleInput.Focus()
}

// cullMoveItem switches to move mode for the current cull item.
func (a *App) cullMoveItem() (tea.Model, tea.Cmd) {
	result := a.cull.CurrentItem()
	if result == nil {
		return a, nil
	}

	// Create item wrapper for the bookmark
	item := Item{
		Kind:     ItemBookmark,
		Bookmark: result.Bookmark,
	}

	a.move.ItemsToMove = []Item{item}
	a.mode = ModeMove
	a.move.Folders = a.buildFolderPaths()
	a.move.FilteredFolders = a.move.Folders
	a.move.FilterInput.Reset()
	a.move.FilterInput.Focus()

	// Find current location
	currentPath := "/"
	if result.Bookmark.FolderID != nil {
		currentPath = a.store.GetFolderPath(result.Bookmark.FolderID)
	}
	a.move.FolderIdx = a.findMoveFolderIndex(currentPath)

	return a, a.move.FilterInput.Focus()
}

// startOrganize initiates the AI-powered organize analysis.
func (a *App) startOrganize() (tea.Model, tea.Cmd) {
	// Check for API key first
	client, err := ai.NewClient()
	if err != nil {
		cmd := a.setMessage(MessageError, "No API key: set ANTHROPIC_API_KEY")
		return a, cmd
	}
	_ = client // Will be used in the command

	// Check for cache
	a.checkOrganizeCache()
	if a.organize.HasCache {
		a.organize.MenuCursor = 0
		a.mode = ModeOrganizeMenu
		return a, nil
	}

	// No cache - start fresh analysis
	return a.startOrganizeAnalysis()
}

// startOrganizeAnalysis begins the AI-powered organize analysis.
func (a *App) startOrganizeAnalysis() (tea.Model, tea.Cmd) {
	// Get the current item
	displayItems := a.getDisplayItems()
	if len(displayItems) == 0 || a.browser.Cursor >= len(displayItems) {
		cmd := a.setMessage(MessageError, "No item selected")
		return a, cmd
	}
	item := displayItems[a.browser.Cursor]

	// Reset organize state
	a.organize.Reset()

	// Collect items to analyze
	var itemsToAnalyze []Item
	if item.IsFolder() {
		// Recursively collect all items in folder
		itemsToAnalyze = a.collectFolderItemsRecursive(item.Folder.ID)
		a.organize.SourceFolderID = &item.Folder.ID
	} else {
		itemsToAnalyze = []Item{item}
		a.organize.SourceItem = &item
	}

	if len(itemsToAnalyze) == 0 {
		cmd := a.setMessage(MessageInfo, "No items to organize")
		return a, cmd
	}

	a.organize.Total = len(itemsToAnalyze)
	a.mode = ModeOrganizeLoading

	// Reset progress counter
	atomic.StoreInt64(&organizeProgressCounter, 0)

	// Start analysis
	return a, tea.Batch(
		a.analyzeOrganizeItems(itemsToAnalyze),
		organizeTickCmd(),
	)
}

// collectFolderItemsRecursive collects all bookmarks and folders recursively.
func (a *App) collectFolderItemsRecursive(folderID string) []Item {
	var items []Item

	// Get direct children
	bookmarks := a.store.GetBookmarksInFolder(&folderID)
	for i := range bookmarks {
		items = append(items, Item{Kind: ItemBookmark, Bookmark: &bookmarks[i]})
	}

	folders := a.store.GetFoldersInFolder(&folderID)
	for i := range folders {
		items = append(items, Item{Kind: ItemFolder, Folder: &folders[i]})
		// Recurse into subfolders
		items = append(items, a.collectFolderItemsRecursive(folders[i].ID)...)
	}

	return items
}

// tagsEqual returns true if two tag slices contain the same tags (order-independent).
func tagsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	aSet := make(map[string]bool)
	for _, tag := range a {
		aSet[tag] = true
	}
	for _, tag := range b {
		if !aSet[tag] {
			return false
		}
	}
	return true
}

// analyzeOrganizeItems starts the AI analysis for all items.
func (a *App) analyzeOrganizeItems(items []Item) tea.Cmd {
	return func() tea.Msg {
		client, err := ai.NewClient()
		if err != nil {
			return organizeCompleteMsg{}
		}

		context := ai.BuildContext(a.store)
		var suggestions []OrganizeSuggestion

		for i, item := range items {
			atomic.StoreInt64(&organizeProgressCounter, int64(i+1))

			var title, url, currentPath string
			var tags []string
			var isFolder bool

			if item.IsFolder() {
				title = item.Folder.Name
				currentPath = a.store.GetFolderPath(item.Folder.ParentID)
				isFolder = true
			} else {
				title = item.Bookmark.Title
				url = item.Bookmark.URL
				currentPath = a.store.GetFolderPath(item.Bookmark.FolderID)
				tags = item.Bookmark.Tags
				isFolder = false
			}

			resp, err := client.SuggestOrganize(title, url, currentPath, tags, isFolder, context)
			if err != nil {
				continue // Skip items that fail
			}

			// Filter out low confidence suggestions
			if resp.Confidence == "low" {
				continue
			}

			// Check if there are any changes (folder OR tags)
			folderDiffers := resp.FolderPath != currentPath
			tagsDiffer := !tagsEqual(tags, resp.SuggestedTags)

			// Skip if neither folder nor tags changed
			if !folderDiffers && !tagsDiffer {
				continue
			}

			suggestions = append(suggestions, OrganizeSuggestion{
				Item:          item,
				CurrentPath:   currentPath,
				SuggestedPath: resp.FolderPath,
				IsNewFolder:   resp.IsNewFolder,
				CurrentTags:   tags,
				SuggestedTags: resp.SuggestedTags,
				Processed:     false,
			})
		}

		// Store suggestions and complete
		return organizeResultsMsg{suggestions: suggestions}
	}
}

// organizeNextUnprocessed moves cursor to next unprocessed suggestion.
func (a *App) organizeNextUnprocessed() {
	for i := a.organize.Cursor + 1; i < len(a.organize.Suggestions); i++ {
		if !a.organize.Suggestions[i].Processed {
			a.organize.Cursor = i
			return
		}
	}
}

// organizePrevUnprocessed moves cursor to previous unprocessed suggestion.
func (a *App) organizePrevUnprocessed() {
	for i := a.organize.Cursor - 1; i >= 0; i-- {
		if !a.organize.Suggestions[i].Processed {
			a.organize.Cursor = i
			return
		}
	}
}

// organizeAcceptCurrent applies the suggested organization changes.
func (a *App) organizeAcceptCurrent() (tea.Model, tea.Cmd) {
	sug := a.organize.CurrentSuggestion()
	if sug == nil || sug.Processed {
		return a, nil
	}

	var moved, tagged, created bool

	// Apply folder move if different
	if sug.HasFolderChanges() {
		targetFolder, wasCreated := a.store.GetOrCreateFolderByPath(sug.SuggestedPath)
		created = wasCreated
		var targetFolderID *string
		if targetFolder != nil {
			targetFolderID = &targetFolder.ID
		}

		if sug.Item.IsFolder() {
			folder := a.store.GetFolderByID(sug.Item.Folder.ID)
			if folder != nil {
				folder.ParentID = targetFolderID
				moved = true
			}
		} else {
			bookmark := a.store.GetBookmarkByID(sug.Item.Bookmark.ID)
			if bookmark != nil {
				bookmark.FolderID = targetFolderID
				moved = true
			}
		}
	}

	// Apply tag changes for bookmarks
	if !sug.Item.IsFolder() && sug.HasTagChanges() {
		bookmark := a.store.GetBookmarkByID(sug.Item.Bookmark.ID)
		if bookmark != nil {
			bookmark.Tags = sug.SuggestedTags
			tagged = true
		}
	}

	// Save after applying changes
	if moved || tagged {
		a.saveStore()
	}

	sug.Processed = true

	// Build action message
	var action string
	switch {
	case moved && tagged && created:
		action = "Moved (new folder) + tagged"
	case moved && tagged:
		action = "Moved + tagged"
	case moved && created:
		action = "Moved (new folder)"
	case moved:
		action = "Moved"
	case tagged:
		action = "Tagged"
	default:
		action = "Organized"
	}
	cmd := a.setMessage(MessageInfo, action+": "+sug.Item.Title())

	// Move to next or exit if done
	if a.organize.UnprocessedCount() == 0 {
		a.mode = ModeNormal
		a.refreshItems()
		a.organize.Reset()
		return a, cmd
	}
	a.organizeNextUnprocessed()
	return a, cmd
}

// organizeSkipCurrent marks the current suggestion as processed without moving.
func (a *App) organizeSkipCurrent() {
	sug := a.organize.CurrentSuggestion()
	if sug == nil || sug.Processed {
		return
	}

	sug.Processed = true

	if a.organize.UnprocessedCount() == 0 {
		a.mode = ModeNormal
		a.organize.Reset()
		return
	}
	a.organizeNextUnprocessed()
}

// organizeOpenCurrent opens the current item's URL in browser.
func (a *App) organizeOpenCurrent() (tea.Model, tea.Cmd) {
	sug := a.organize.CurrentSuggestion()
	if sug == nil || sug.Item.IsFolder() {
		return a, nil
	}

	return a, openURLCmd(sug.Item.Bookmark.URL)
}

// organizeMoveCurrent switches to move mode for manual folder selection.
func (a *App) organizeMoveCurrent() (tea.Model, tea.Cmd) {
	sug := a.organize.CurrentSuggestion()
	if sug == nil || sug.Processed {
		return a, nil
	}

	// Set up move state with this item
	a.move.Reset()
	a.move.ItemsToMove = []Item{sug.Item}
	a.move.Folders = a.buildFolderPaths()
	a.move.FilteredFolders = a.move.Folders
	a.move.FolderIdx = a.findMoveFolderIndex(sug.SuggestedPath)
	a.move.FilterInput.Focus()
	a.move.ReturnMode = ModeOrganizeResults
	a.move.OrganizeSuggestion = sug

	a.mode = ModeMove
	return a, nil
}

// organizeDeleteCurrent deletes the current item.
func (a *App) organizeDeleteCurrent() (tea.Model, tea.Cmd) {
	sug := a.organize.CurrentSuggestion()
	if sug == nil || sug.Processed {
		return a, nil
	}

	// Delete the item
	if sug.Item.IsFolder() {
		a.store.RemoveFolderByID(sug.Item.Folder.ID)
	} else {
		a.store.RemoveBookmarkByID(sug.Item.Bookmark.ID)
	}
	a.saveStore()

	sug.Processed = true

	cmd := a.setMessage(MessageInfo, "Deleted: "+sug.Item.Title())

	// Move to next or exit if done
	if a.organize.UnprocessedCount() == 0 {
		a.mode = ModeNormal
		a.refreshItems()
		a.organize.Reset()
		return a, cmd
	}
	a.organizeNextUnprocessed()
	return a, cmd
}
