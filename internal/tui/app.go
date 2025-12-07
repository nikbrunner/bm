package tui

import (
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/nikbrunner/bm/internal/ai"
	"github.com/nikbrunner/bm/internal/model"
	"github.com/nikbrunner/bm/internal/tui/layout"
	"github.com/sahilm/fuzzy"
)

// aiResponseMsg is sent when the AI API call completes.
type aiResponseMsg struct {
	response *ai.Response
	err      error
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
	ModeQuickAdd        // URL input for AI quick add
	ModeQuickAddLoading // Waiting for AI response
	ModeQuickAddConfirm // Review/edit AI suggestion
	ModeMove            // Move item to different folder
)

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

	// Yank buffer
	yankedItem *Item

	// UI mode and modal state
	mode  Mode
	modal ModalState

	// Quick add (AI-powered) state
	quickAdd QuickAddState

	// Move state
	move MoveState

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
	Keys         *KeyMap              // optional, uses default if nil
	Styles       *Styles              // optional, uses default if nil
	LayoutConfig *layout.LayoutConfig // optional, uses default if nil
}

// NewApp creates a new App with the given parameters.
func NewApp(params AppParams) App {
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

// refreshPinnedItems rebuilds the pinnedItems slice from the store.
func (a *App) refreshPinnedItems() {
	a.pinnedItems = []Item{}

	// Get pinned folders and bookmarks
	folders := a.store.GetPinnedFolders()
	bookmarks := a.store.GetPinnedBookmarks()

	// Add folders first (same convention as browser)
	for i := range folders {
		a.pinnedItems = append(a.pinnedItems, Item{
			Kind:   ItemFolder,
			Folder: &folders[i],
		})
	}

	// Add bookmarks
	for i := range bookmarks {
		a.pinnedItems = append(a.pinnedItems, Item{
			Kind:     ItemBookmark,
			Bookmark: &bookmarks[i],
		})
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

// YankedItem returns the item in the yank buffer, or nil if empty.
func (a App) YankedItem() *Item {
	return a.yankedItem
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

			// Build folder picker options
			a.quickAdd.Folders = a.buildFolderPaths()
			a.quickAdd.FolderIdx = a.findFolderIndex(msg.response.FolderPath)

			a.modal.TitleInput.Focus()
			return a, a.modal.TitleInput.Focus()
		}
		return a, nil

	case tea.KeyMsg:
		// Handle modal modes first
		if a.mode != ModeNormal {
			return a.updateModal(msg)
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

		// Reset sequence flags for any other key
		a.lastKeyWasG = false

		switch {
		case key.Matches(msg, a.keys.Quit):
			return a, tea.Quit

		case key.Matches(msg, a.keys.Down):
			displayItems := a.getDisplayItems()
			if len(displayItems) > 0 && a.browser.Cursor < len(displayItems)-1 {
				a.browser.Cursor++
			}
			a.clearMessage() // Clear message on navigation

		case key.Matches(msg, a.keys.Up):
			if a.browser.Cursor > 0 {
				a.browser.Cursor--
			}
			a.clearMessage() // Clear message on navigation

		case key.Matches(msg, a.keys.Bottom):
			displayItems := a.getDisplayItems()
			if len(displayItems) > 0 {
				a.browser.Cursor = len(displayItems) - 1
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

		case key.Matches(msg, a.keys.ToggleConfirm):
			// Toggle delete confirmation
			a.confirmDelete = !a.confirmDelete

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

		case key.Matches(msg, a.keys.YankURL):
			// Yank URL to clipboard
			return a.yankURLToClipboard()

		case key.Matches(msg, a.keys.Help):
			// Toggle help overlay
			a.mode = ModeHelp
			return a, nil
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
	case key.Matches(msg, a.keys.Quit):
		return a, tea.Quit

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

	case key.Matches(msg, a.keys.Help):
		a.mode = ModeHelp
		return a, nil
	}

	// Handle Enter key for activating pinned items
	if msg.Type == tea.KeyEnter {
		return a.activatePinnedItem()
	}

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

// togglePinCurrentItem toggles pin on the currently selected item in browser pane.
// Returns a command to schedule message auto-clear.
func (a *App) togglePinCurrentItem() tea.Cmd {
	displayItems := a.getDisplayItems()
	if len(displayItems) == 0 || a.browser.Cursor >= len(displayItems) {
		return nil
	}

	var cmd tea.Cmd
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
		case key.Matches(msg, a.keys.Quit):
			// 'q' closes help, doesn't quit
			a.mode = ModeNormal
			return a, nil
		case key.Matches(msg, a.keys.Help):
			// '?' toggles help off
			a.mode = ModeNormal
			return a, nil
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
			// Cancel move
			a.mode = ModeNormal
			return a, nil
		case tea.KeyEnter:
			// Execute move if there are filtered results
			if len(a.move.FilteredFolders) > 0 {
				a.executeMoveItem()
			}
			a.mode = ModeNormal
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

	// Handle quick add confirmation mode
	if a.mode == ModeQuickAddConfirm {
		switch msg.Type {
		case tea.KeyEsc:
			a.mode = ModeNormal
			return a, nil
		case tea.KeyEnter:
			// Save the bookmark with edited values
			return a.submitQuickAdd()
		case tea.KeyTab:
			// Cycle through: title -> folder -> tags -> title
			if a.modal.TitleInput.Focused() {
				a.modal.TitleInput.Blur()
				// Focus is now on folder picker (no input to focus)
			} else if a.modal.TagsInput.Focused() {
				a.modal.TagsInput.Blur()
				a.modal.TitleInput.Focus()
				return a, a.modal.TitleInput.Focus()
			} else {
				// Was on folder picker, move to tags
				a.modal.TagsInput.Focus()
				return a, a.modal.TagsInput.Focus()
			}
			return a, nil
		case tea.KeyUp:
			// Navigate folder picker up (when not in text input)
			if !a.modal.TitleInput.Focused() && !a.modal.TagsInput.Focused() {
				if a.quickAdd.FolderIdx > 0 {
					a.quickAdd.FolderIdx--
				}
			}
			return a, nil
		case tea.KeyDown:
			// Navigate folder picker down (when not in text input)
			if !a.modal.TitleInput.Focused() && !a.modal.TagsInput.Focused() {
				if a.quickAdd.FolderIdx < len(a.quickAdd.Folders)-1 {
					a.quickAdd.FolderIdx++
				}
			}
			return a, nil
		}

		// Handle j/k for folder navigation when not in text input
		if msg.Type == tea.KeyRunes && !a.modal.TitleInput.Focused() && !a.modal.TagsInput.Focused() {
			switch string(msg.Runes) {
			case "j":
				if a.quickAdd.FolderIdx < len(a.quickAdd.Folders)-1 {
					a.quickAdd.FolderIdx++
				}
				return a, nil
			case "k":
				if a.quickAdd.FolderIdx > 0 {
					a.quickAdd.FolderIdx--
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
		}
		return a, cmd
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
	if a.quickAdd.FolderIdx > 0 && a.quickAdd.FolderIdx < len(a.quickAdd.Folders) {
		folderPath := a.quickAdd.Folders[a.quickAdd.FolderIdx]
		folder, _ := a.store.GetOrCreateFolderByPath(folderPath)
		if folder != nil {
			folderID = &folder.ID
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
	a.refreshItems()
	a.mode = ModeNormal
	a.setStatus("Bookmark added: " + title)
	return a, nil
}

// yankCurrentItem copies the current item to the yank buffer.
func (a *App) yankCurrentItem() {
	displayItems := a.getDisplayItems()
	if len(displayItems) == 0 || a.browser.Cursor >= len(displayItems) {
		return
	}
	item := displayItems[a.browser.Cursor]
	a.yankedItem = &item
	a.setStatus("Yanked: " + item.Title())
}

// cutCurrentItem copies the current item to yank buffer and deletes it.
// Shows confirmation dialog if confirmDelete is enabled.
func (a *App) cutCurrentItem() {
	displayItems := a.getDisplayItems()
	if len(displayItems) == 0 || a.browser.Cursor >= len(displayItems) {
		return
	}

	item := displayItems[a.browser.Cursor]
	a.modal.CutMode = true

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
	if item.IsFolder() {
		a.yankedItem = &item
		a.store.RemoveFolderByID(item.Folder.ID)
		a.setStatus("Cut: " + item.Folder.Name)
	} else {
		a.yankedItem = &item
		a.store.RemoveBookmarkByID(item.Bookmark.ID)
		a.setStatus("Cut: " + item.Bookmark.Title)
	}

	a.refreshItems()
	if a.browser.Cursor >= len(a.browser.Items) && a.browser.Cursor > 0 {
		a.browser.Cursor--
	}
}

// deleteCurrentItem deletes the current item without copying to yank buffer.
// Shows confirmation dialog if confirmDelete is enabled.
func (a *App) deleteCurrentItem() {
	displayItems := a.getDisplayItems()
	if len(displayItems) == 0 || a.browser.Cursor >= len(displayItems) {
		return
	}

	item := displayItems[a.browser.Cursor]
	a.modal.CutMode = false

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

	a.refreshItems()
	if a.browser.Cursor >= len(a.browser.Items) && a.browser.Cursor > 0 {
		a.browser.Cursor--
	}
}

// confirmDeleteItem performs the actual deletion after confirmation.
// Handles both folders and bookmarks.
func (a *App) confirmDeleteItem() {
	var title string

	// Try as folder first
	folder := a.store.GetFolderByID(a.modal.EditItemID)
	if folder != nil {
		title = folder.Name
		if a.modal.CutMode {
			// Make a copy before deleting
			folderCopy := *folder
			item := Item{Kind: ItemFolder, Folder: &folderCopy}
			a.yankedItem = &item
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
			a.yankedItem = &item
		}
		a.store.RemoveBookmarkByID(a.modal.EditItemID)
	}

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

// executeMoveItem moves the current item to the selected folder.
func (a *App) executeMoveItem() {
	if a.move.FolderIdx < 0 || a.move.FolderIdx >= len(a.move.FilteredFolders) {
		return
	}

	displayItems := a.getDisplayItems()
	if len(displayItems) == 0 || a.browser.Cursor >= len(displayItems) {
		return
	}

	item := displayItems[a.browser.Cursor]
	targetPath := a.move.FilteredFolders[a.move.FolderIdx]

	// Resolve target folder ID (nil for root "/")
	var targetFolderID *string
	if targetPath != "/" {
		targetFolder := a.store.GetFolderByPath(targetPath)
		if targetFolder != nil {
			targetFolderID = &targetFolder.ID
		}
	}

	if item.IsFolder() {
		folder := a.store.GetFolderByID(item.Folder.ID)
		if folder == nil {
			return
		}

		// Prevent moving folder into itself or its descendants
		if targetFolderID != nil && a.isFolderDescendant(item.Folder.ID, *targetFolderID) {
			a.setStatus("Cannot move folder into itself")
			return
		}

		folder.ParentID = targetFolderID
		a.setStatus("Moved: " + folder.Name + " → " + targetPath)
	} else {
		bookmark := a.store.GetBookmarkByID(item.Bookmark.ID)
		if bookmark == nil {
			return
		}

		bookmark.FolderID = targetFolderID
		a.setStatus("Moved: " + bookmark.Title + " → " + targetPath)
	}

	a.refreshItems()
	a.refreshPinnedItems()

	// Adjust cursor if needed
	if a.browser.Cursor >= len(a.browser.Items) && a.browser.Cursor > 0 {
		a.browser.Cursor--
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

// pasteItem pastes the yanked item before or after the cursor.
func (a *App) pasteItem(before bool) {
	if a.yankedItem == nil {
		a.setStatus("Nothing to paste")
		return
	}

	// Calculate insert position
	insertIdx := a.browser.Cursor
	if !before && len(a.browser.Items) > 0 {
		insertIdx = a.browser.Cursor + 1
	}

	title := a.yankedItem.Title()

	if a.yankedItem.IsFolder() {
		// Create a copy with new ID
		newFolder := model.NewFolder(model.NewFolderParams{
			Name:     a.yankedItem.Folder.Name,
			ParentID: a.browser.CurrentFolderID,
		})

		// Count folders in current view to find insert position
		folderCount := 0
		for _, item := range a.browser.Items {
			if item.IsFolder() {
				folderCount++
			}
		}

		// If pasting among folders
		if insertIdx <= folderCount {
			a.store.InsertFolderAt(newFolder, insertIdx)
		} else {
			a.store.AddFolder(newFolder)
		}
	} else {
		// Create a copy with new ID
		newBookmark := model.NewBookmark(model.NewBookmarkParams{
			Title:    a.yankedItem.Bookmark.Title,
			URL:      a.yankedItem.Bookmark.URL,
			FolderID: a.browser.CurrentFolderID,
			Tags:     a.yankedItem.Bookmark.Tags,
		})

		// Count folders to calculate bookmark insert position
		folderCount := 0
		for _, item := range a.browser.Items {
			if item.IsFolder() {
				folderCount++
			}
		}

		// Bookmarks come after folders in the view
		bookmarkIdx := insertIdx - folderCount
		if bookmarkIdx < 0 {
			bookmarkIdx = 0
		}

		a.store.InsertBookmarkAt(newBookmark, bookmarkIdx)
	}

	a.refreshItems()
	a.setStatus("Pasted: " + title)
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

// findFolderIndex finds the index of a folder path in quickAddFolders.
func (a *App) findFolderIndex(path string) int {
	for i, p := range a.quickAdd.Folders {
		if p == path {
			return i
		}
	}
	// Default to root if not found
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
