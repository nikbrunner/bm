package tui

import (
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/nikbrunner/bm/internal/ai"
	"github.com/nikbrunner/bm/internal/model"
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
	store  *model.Store
	keys   KeyMap
	styles Styles

	// Focus state
	focusedPane  FocusedPane // which pane has focus
	pinnedCursor int         // cursor in pinned pane
	pinnedItems  []Item      // cached pinned items (folders first, then bookmarks)

	// Navigation state
	currentFolderID *string  // nil = root
	folderStack     []string // breadcrumb trail of folder IDs
	cursor          int      // selected item index
	items           []Item   // current list items

	// Sort mode
	sortMode SortMode

	// Global search (s key)
	searchInput  textinput.Model
	fuzzyMatches []fuzzyMatch // Current fuzzy match results
	fuzzyCursor  int          // Selected index in fuzzy results
	allItems     []Item       // All items for global search

	// Local filter (/ key)
	filterInput   textinput.Model
	filterQuery   string // Active filter query (persists after closing filter)
	filteredItems []Item // Items matching filter in current folder

	// For gg command
	lastKeyWasG bool

	// Yank buffer
	yankedItem *Item

	// Modal state
	mode       Mode
	titleInput textinput.Model
	urlInput   textinput.Model
	tagsInput  textinput.Model
	editItemID string // ID of item being edited (folder or bookmark)
	cutMode    bool   // true = cut (buffer), false = delete (no buffer)

	// Quick add (AI-powered) state
	quickAddInput     textinput.Model // URL input
	quickAddResponse  *ai.Response    // AI suggestion
	quickAddError     error           // AI error (if any)
	quickAddFolders   []string        // Available folder paths for picker
	quickAddFolderIdx int             // Selected folder index in picker

	// Settings
	confirmDelete bool // true = ask confirmation before delete (default true)

	// Status message (for user feedback)
	statusMessage string

	// Window dimensions
	width  int
	height int
}

// AppParams holds parameters for creating a new App.
type AppParams struct {
	Store  *model.Store
	Keys   *KeyMap // optional, uses default if nil
	Styles *Styles // optional, uses default if nil
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

	// Initialize text inputs
	titleInput := textinput.New()
	titleInput.Placeholder = "Title"
	titleInput.CharLimit = 100
	titleInput.Width = 40

	urlInput := textinput.New()
	urlInput.Placeholder = "URL"
	urlInput.CharLimit = 500
	urlInput.Width = 40

	tagsInput := textinput.New()
	tagsInput.Placeholder = "tag1, tag2, tag3"
	tagsInput.CharLimit = 200
	tagsInput.Width = 40

	searchInput := textinput.New()
	searchInput.Placeholder = "Search all..."
	searchInput.CharLimit = 100
	searchInput.Width = 40

	filterInput := textinput.New()
	filterInput.Placeholder = "Filter..."
	filterInput.CharLimit = 50
	filterInput.Width = 30

	quickAddInput := textinput.New()
	quickAddInput.Placeholder = "https://..."
	quickAddInput.CharLimit = 500
	quickAddInput.Width = 50

	app := App{
		store:           params.Store,
		keys:            keys,
		styles:          styles,
		focusedPane:     PaneBrowser, // will be updated after refreshPinnedItems
		pinnedCursor:    0,
		currentFolderID: nil,
		folderStack:     []string{},
		cursor:          0,
		mode:            ModeNormal,
		titleInput:      titleInput,
		urlInput:        urlInput,
		tagsInput:       tagsInput,
		searchInput:     searchInput,
		filterInput:     filterInput,
		quickAddInput:   quickAddInput,
		confirmDelete:   true,
		width:           80,
		height:          24,
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
	a.items = []Item{}
	// Clear filter when refreshing (folder changed)
	a.filterQuery = ""
	a.filteredItems = nil

	// Get folders and bookmarks
	folders := a.store.GetFoldersInFolder(a.currentFolderID)
	bookmarks := a.store.GetBookmarksInFolder(a.currentFolderID)

	// Apply sorting based on current mode
	switch a.sortMode {
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
		a.items = append(a.items, Item{
			Kind:   ItemFolder,
			Folder: &folders[i],
		})
	}

	// Add bookmarks
	for i := range bookmarks {
		a.items = append(a.items, Item{
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
	a.folderStack = path
}

// updateFuzzyMatches performs fuzzy matching on allItems with the current query.
func (a *App) updateFuzzyMatches() {
	query := a.searchInput.Value()

	if query == "" {
		// No query - show all items
		a.fuzzyMatches = make([]fuzzyMatch, len(a.allItems))
		for i, item := range a.allItems {
			a.fuzzyMatches[i] = fuzzyMatch{Item: item}
		}
		return
	}

	// Run fuzzy matching
	matches := fuzzy.FindFrom(query, itemStrings(a.allItems))

	// Convert to our fuzzyMatch type
	a.fuzzyMatches = make([]fuzzyMatch, len(matches))
	for i, m := range matches {
		a.fuzzyMatches[i] = fuzzyMatch{
			Item:           a.allItems[m.Index],
			MatchedIndexes: m.MatchedIndexes,
			Score:          m.Score,
		}
	}

	// Reset cursor if out of bounds
	if a.fuzzyCursor >= len(a.fuzzyMatches) {
		a.fuzzyCursor = 0
	}
}

// Cursor returns the current cursor position.
func (a App) Cursor() int {
	return a.cursor
}

// CurrentFolderID returns the ID of the current folder (nil for root).
func (a App) CurrentFolderID() *string {
	return a.currentFolderID
}

// Items returns the current list of items.
func (a App) Items() []Item {
	return a.items
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
	return a.sortMode
}

// FilterQuery returns the current filter query.
func (a App) FilterQuery() string {
	return a.filterQuery
}

// StatusMessage returns the current status message.
func (a App) StatusMessage() string {
	return a.statusMessage
}

// setStatus sets the status message.
func (a *App) setStatus(msg string) {
	a.statusMessage = msg
}

// SetConfirmDelete sets the confirmDelete flag (for testing).
func (a *App) SetConfirmDelete(confirm bool) {
	a.confirmDelete = confirm
}

// FuzzyMatches returns the current fuzzy match results.
func (a App) FuzzyMatches() []fuzzyMatch {
	return a.fuzzyMatches
}

// FuzzyCursor returns the selected index in fuzzy results.
func (a App) FuzzyCursor() int {
	return a.fuzzyCursor
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

	case aiResponseMsg:
		// Handle AI response for quick add
		if a.mode == ModeQuickAddLoading {
			if msg.err != nil {
				// AI failed - save to "To Review" with URL as title
				a.quickAddError = msg.err
				url := a.quickAddInput.Value()
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
			a.quickAddResponse = msg.response
			a.mode = ModeQuickAddConfirm

			// Pre-fill inputs with AI suggestion
			a.titleInput.Reset()
			a.titleInput.SetValue(msg.response.Title)
			a.tagsInput.Reset()
			a.tagsInput.SetValue(strings.Join(msg.response.Tags, ", "))

			// Build folder picker options
			a.quickAddFolders = a.buildFolderPaths()
			a.quickAddFolderIdx = a.findFolderIndex(msg.response.FolderPath)

			a.titleInput.Focus()
			return a, a.titleInput.Focus()
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
				a.cursor = 0
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
			a.togglePinCurrentItem()
			return a, nil
		}

		// Reset sequence flags for any other key
		a.lastKeyWasG = false

		switch {
		case key.Matches(msg, a.keys.Quit):
			return a, tea.Quit

		case key.Matches(msg, a.keys.Down):
			displayItems := a.getDisplayItems()
			if len(displayItems) > 0 && a.cursor < len(displayItems)-1 {
				a.cursor++
			}
			a.statusMessage = "" // Clear status on navigation

		case key.Matches(msg, a.keys.Up):
			if a.cursor > 0 {
				a.cursor--
			}
			a.statusMessage = "" // Clear status on navigation

		case key.Matches(msg, a.keys.Bottom):
			displayItems := a.getDisplayItems()
			if len(displayItems) > 0 {
				a.cursor = len(displayItems) - 1
			}

		case key.Matches(msg, a.keys.Right):
			// Enter folder or open bookmark
			displayItems := a.getDisplayItems()
			if len(displayItems) > 0 && a.cursor < len(displayItems) {
				item := displayItems[a.cursor]
				if item.IsFolder() {
					// Push current folder to stack
					if a.currentFolderID != nil {
						a.folderStack = append(a.folderStack, *a.currentFolderID)
					}
					// Enter the folder
					id := item.Folder.ID
					a.currentFolderID = &id
					a.cursor = 0
					a.refreshItems()
				} else {
					// Open bookmark URL
					return a.openBookmark()
				}
			}

		case key.Matches(msg, a.keys.Left):
			// At root level: switch to pinned pane
			if a.currentFolderID == nil {
				a.focusedPane = PanePinned
				return a, nil
			}
			// Go back to parent folder
			if len(a.folderStack) > 0 {
				// Pop from stack
				lastIdx := len(a.folderStack) - 1
				parentID := a.folderStack[lastIdx]
				a.folderStack = a.folderStack[:lastIdx]
				a.currentFolderID = &parentID
			} else {
				// Back to root
				a.currentFolderID = nil
			}
			a.cursor = 0
			a.refreshItems()

		case key.Matches(msg, a.keys.PasteAfter):
			a.pasteItem(false) // after cursor

		case key.Matches(msg, a.keys.PasteBefore):
			a.pasteItem(true) // before cursor

		case key.Matches(msg, a.keys.AddBookmark):
			a.mode = ModeAddBookmark
			a.titleInput.Reset()
			a.urlInput.Reset()
			a.titleInput.Focus()
			return a, a.titleInput.Focus()

		case key.Matches(msg, a.keys.AddFolder):
			a.mode = ModeAddFolder
			a.titleInput.Reset()
			a.titleInput.Focus()
			return a, a.titleInput.Focus()

		case key.Matches(msg, a.keys.QuickAdd):
			// AI-powered quick add
			a.mode = ModeQuickAdd
			a.quickAddInput.Reset()
			a.quickAddResponse = nil
			a.quickAddError = nil
			// Pre-fill with clipboard contents
			if clipContent, err := clipboard.ReadAll(); err == nil && clipContent != "" {
				a.quickAddInput.SetValue(clipContent)
			}
			a.quickAddInput.Focus()
			return a, a.quickAddInput.Focus()

		case key.Matches(msg, a.keys.Edit):
			// Only edit if there's an item selected
			displayItems := a.getDisplayItems()
			if len(displayItems) == 0 || a.cursor >= len(displayItems) {
				return a, nil
			}
			item := displayItems[a.cursor]
			if item.IsFolder() {
				a.mode = ModeEditFolder
				a.editItemID = item.Folder.ID
				a.titleInput.Reset()
				a.titleInput.SetValue(item.Folder.Name)
				a.titleInput.Focus()
				return a, a.titleInput.Focus()
			} else {
				a.mode = ModeEditBookmark
				a.editItemID = item.Bookmark.ID
				a.titleInput.Reset()
				a.titleInput.SetValue(item.Bookmark.Title)
				a.urlInput.Reset()
				a.urlInput.SetValue(item.Bookmark.URL)
				a.titleInput.Focus()
				return a, a.titleInput.Focus()
			}

		case key.Matches(msg, a.keys.EditTags):
			// Only edit tags on bookmarks
			displayItems := a.getDisplayItems()
			if len(displayItems) == 0 || a.cursor >= len(displayItems) {
				return a, nil
			}
			item := displayItems[a.cursor]
			if item.IsFolder() {
				// Folders don't have tags
				return a, nil
			}
			a.mode = ModeEditTags
			a.editItemID = item.Bookmark.ID
			a.tagsInput.Reset()
			// Convert tags slice to comma-separated string
			a.tagsInput.SetValue(strings.Join(item.Bookmark.Tags, ", "))
			a.tagsInput.Focus()
			return a, a.tagsInput.Focus()

		case key.Matches(msg, a.keys.Sort):
			// Cycle through sort modes
			a.sortMode = (a.sortMode + 1) % 4
			a.refreshItems()

		case key.Matches(msg, a.keys.ToggleConfirm):
			// Toggle delete confirmation
			a.confirmDelete = !a.confirmDelete

		case key.Matches(msg, a.keys.Search):
			// Open fuzzy finder mode with GLOBAL search
			a.mode = ModeSearch
			a.searchInput.Reset()
			a.searchInput.Focus()
			a.fuzzyCursor = 0
			// Gather ALL items from the entire store for fuzzy matching
			a.allItems = []Item{}
			for i := range a.store.Folders {
				a.allItems = append(a.allItems, Item{
					Kind:   ItemFolder,
					Folder: &a.store.Folders[i],
				})
			}
			for i := range a.store.Bookmarks {
				a.allItems = append(a.allItems, Item{
					Kind:     ItemBookmark,
					Bookmark: &a.store.Bookmarks[i],
				})
			}
			a.updateFuzzyMatches()
			return a, a.searchInput.Focus()

		case key.Matches(msg, a.keys.Filter):
			// Open local filter for current folder
			a.mode = ModeFilter
			a.filterInput.Reset()
			a.filterInput.SetValue(a.filterQuery) // Restore previous filter
			a.filterInput.Focus()
			return a, a.filterInput.Focus()

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
		a.unpinSelectedItem()
		return a, nil
	}

	// Handle x - unpin (same as d in pinned pane)
	if key.Matches(msg, a.keys.Cut) {
		a.lastKeyWasG = false
		a.unpinSelectedItem()
		return a, nil
	}

	// Handle m - unpin
	if key.Matches(msg, a.keys.Pin) {
		a.lastKeyWasG = false
		a.unpinSelectedItem()
		return a, nil
	}

	a.lastKeyWasG = false

	switch {
	case key.Matches(msg, a.keys.Quit):
		return a, tea.Quit

	case key.Matches(msg, a.keys.Down):
		if len(a.pinnedItems) > 0 && a.pinnedCursor < len(a.pinnedItems)-1 {
			a.pinnedCursor++
		}
		a.statusMessage = ""

	case key.Matches(msg, a.keys.Up):
		if a.pinnedCursor > 0 {
			a.pinnedCursor--
		}
		a.statusMessage = ""

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
		a.currentFolderID = &id
		a.cursor = 0
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
func (a *App) unpinSelectedItem() {
	item := a.selectedPinnedItem()
	if item == nil {
		return
	}

	if item.IsFolder() {
		_ = a.store.TogglePinFolder(item.Folder.ID)
		a.statusMessage = "Unpinned: " + item.Folder.Name
	} else {
		_ = a.store.TogglePinBookmark(item.Bookmark.ID)
		a.statusMessage = "Unpinned: " + item.Bookmark.Title
	}

	a.refreshPinnedItems()

	// Adjust cursor if needed
	if a.pinnedCursor >= len(a.pinnedItems) && a.pinnedCursor > 0 {
		a.pinnedCursor--
	}
}

// togglePinCurrentItem toggles pin on the currently selected item in browser pane.
func (a *App) togglePinCurrentItem() {
	displayItems := a.getDisplayItems()
	if len(displayItems) == 0 || a.cursor >= len(displayItems) {
		return
	}

	item := displayItems[a.cursor]
	if item.IsFolder() {
		wasPinned := item.Folder.Pinned
		_ = a.store.TogglePinFolder(item.Folder.ID)
		if wasPinned {
			a.statusMessage = "Unpinned: " + item.Folder.Name
		} else {
			a.statusMessage = "Pinned: " + item.Folder.Name
		}
	} else {
		wasPinned := item.Bookmark.Pinned
		_ = a.store.TogglePinBookmark(item.Bookmark.ID)
		if wasPinned {
			a.statusMessage = "Unpinned: " + item.Bookmark.Title
		} else {
			a.statusMessage = "Pinned: " + item.Bookmark.Title
		}
	}

	a.refreshItems()
	a.refreshPinnedItems()
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

	// Handle search mode (fuzzy finder)
	if a.mode == ModeSearch {
		switch msg.Type {
		case tea.KeyEsc:
			// Cancel search
			a.mode = ModeNormal
			a.fuzzyMatches = nil
			a.allItems = nil
			return a, nil

		case tea.KeyEnter:
			// Select highlighted item and navigate to it
			if len(a.fuzzyMatches) > 0 && a.fuzzyCursor < len(a.fuzzyMatches) {
				selectedItem := a.fuzzyMatches[a.fuzzyCursor].Item

				if selectedItem.IsFolder() {
					// Navigate into the selected folder
					folderID := selectedItem.Folder.ID
					// Build the folder stack by finding parent chain
					a.folderStack = []string{}
					a.buildFolderStack(selectedItem.Folder.ParentID)
					a.currentFolderID = &folderID
					a.cursor = 0
					a.refreshItems()
				} else {
					// Navigate to the folder containing this bookmark
					a.folderStack = []string{}
					if selectedItem.Bookmark.FolderID != nil {
						// Find the folder and build stack
						folder := a.store.GetFolderByID(*selectedItem.Bookmark.FolderID)
						if folder != nil {
							a.buildFolderStack(folder.ParentID)
						}
					}
					a.currentFolderID = selectedItem.Bookmark.FolderID
					a.refreshItems()
					// Set cursor to the bookmark
					for i, item := range a.items {
						if !item.IsFolder() && item.Bookmark.ID == selectedItem.Bookmark.ID {
							a.cursor = i
							break
						}
					}
				}
			}
			a.mode = ModeNormal
			a.fuzzyMatches = nil
			a.allItems = nil
			return a, nil

		case tea.KeyDown:
			// Navigate down in results
			if len(a.fuzzyMatches) > 0 && a.fuzzyCursor < len(a.fuzzyMatches)-1 {
				a.fuzzyCursor++
			}
			return a, nil

		case tea.KeyUp:
			// Navigate up in results
			if a.fuzzyCursor > 0 {
				a.fuzzyCursor--
			}
			return a, nil
		}

		// Handle j/k for vim-style navigation
		if msg.Type == tea.KeyRunes {
			switch string(msg.Runes) {
			case "j":
				if len(a.fuzzyMatches) > 0 && a.fuzzyCursor < len(a.fuzzyMatches)-1 {
					a.fuzzyCursor++
				}
				return a, nil
			case "k":
				if a.fuzzyCursor > 0 {
					a.fuzzyCursor--
				}
				return a, nil
			}
		}

		// Update search input
		var cmd tea.Cmd
		a.searchInput, cmd = a.searchInput.Update(msg)
		// Update fuzzy matches as user types
		a.updateFuzzyMatches()
		return a, cmd
	}

	// Handle local filter mode (/ key)
	if a.mode == ModeFilter {
		switch msg.Type {
		case tea.KeyEsc:
			// Keep filter active, just close input
			a.filterQuery = a.filterInput.Value()
			a.applyFilter()
			a.mode = ModeNormal
			return a, nil
		case tea.KeyEnter:
			// Apply filter and close
			a.filterQuery = a.filterInput.Value()
			a.applyFilter()
			a.mode = ModeNormal
			return a, nil
		case tea.KeyBackspace:
			// If filter is empty and backspace, clear filter entirely
			if a.filterInput.Value() == "" {
				a.filterQuery = ""
				a.applyFilter()
			}
		}

		// Update filter input
		var cmd tea.Cmd
		a.filterInput, cmd = a.filterInput.Update(msg)
		// Live filter as user types
		a.filterQuery = a.filterInput.Value()
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
			url := a.quickAddInput.Value()
			if url == "" {
				return a, nil
			}
			// Start AI call
			a.mode = ModeQuickAddLoading
			return a, a.callAICmd(url)
		}
		// Forward to input
		var cmd tea.Cmd
		a.quickAddInput, cmd = a.quickAddInput.Update(msg)
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
			if a.titleInput.Focused() {
				a.titleInput.Blur()
				// Focus is now on folder picker (no input to focus)
			} else if a.tagsInput.Focused() {
				a.tagsInput.Blur()
				a.titleInput.Focus()
				return a, a.titleInput.Focus()
			} else {
				// Was on folder picker, move to tags
				a.tagsInput.Focus()
				return a, a.tagsInput.Focus()
			}
			return a, nil
		case tea.KeyUp:
			// Navigate folder picker up (when not in text input)
			if !a.titleInput.Focused() && !a.tagsInput.Focused() {
				if a.quickAddFolderIdx > 0 {
					a.quickAddFolderIdx--
				}
			}
			return a, nil
		case tea.KeyDown:
			// Navigate folder picker down (when not in text input)
			if !a.titleInput.Focused() && !a.tagsInput.Focused() {
				if a.quickAddFolderIdx < len(a.quickAddFolders)-1 {
					a.quickAddFolderIdx++
				}
			}
			return a, nil
		}

		// Handle j/k for folder navigation when not in text input
		if msg.Type == tea.KeyRunes && !a.titleInput.Focused() && !a.tagsInput.Focused() {
			switch string(msg.Runes) {
			case "j":
				if a.quickAddFolderIdx < len(a.quickAddFolders)-1 {
					a.quickAddFolderIdx++
				}
				return a, nil
			case "k":
				if a.quickAddFolderIdx > 0 {
					a.quickAddFolderIdx--
				}
				return a, nil
			}
		}

		// Forward to active text input
		var cmd tea.Cmd
		if a.titleInput.Focused() {
			a.titleInput, cmd = a.titleInput.Update(msg)
		} else if a.tagsInput.Focused() {
			a.tagsInput, cmd = a.tagsInput.Update(msg)
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
	if a.mode == ModeAddBookmark || a.mode == ModeEditBookmark {
		// Handle Tab to switch between inputs
		if msg.Type == tea.KeyTab {
			if a.titleInput.Focused() {
				a.titleInput.Blur()
				a.urlInput.Focus()
				return a, a.urlInput.Focus()
			} else {
				a.urlInput.Blur()
				a.titleInput.Focus()
				return a, a.titleInput.Focus()
			}
		}

		if a.titleInput.Focused() {
			a.titleInput, cmd = a.titleInput.Update(msg)
		} else {
			a.urlInput, cmd = a.urlInput.Update(msg)
		}
	} else if a.mode == ModeAddFolder || a.mode == ModeEditFolder {
		a.titleInput, cmd = a.titleInput.Update(msg)
	} else if a.mode == ModeEditTags {
		a.tagsInput, cmd = a.tagsInput.Update(msg)
	}

	return a, cmd
}

// submitModal handles submission of the current modal.
func (a App) submitModal() (tea.Model, tea.Cmd) {
	switch a.mode {
	case ModeAddFolder:
		name := a.titleInput.Value()
		if name == "" {
			// Don't submit with empty name
			return a, nil
		}

		// Create and add the folder
		newFolder := model.NewFolder(model.NewFolderParams{
			Name:     name,
			ParentID: a.currentFolderID,
		})
		a.store.AddFolder(newFolder)
		a.refreshItems()
		a.mode = ModeNormal
		a.setStatus("Folder added: " + name)
		return a, nil

	case ModeAddBookmark:
		title := a.titleInput.Value()
		url := a.urlInput.Value()
		if title == "" || url == "" {
			// Don't submit with empty fields
			return a, nil
		}

		// Create and add the bookmark
		newBookmark := model.NewBookmark(model.NewBookmarkParams{
			Title:    title,
			URL:      url,
			FolderID: a.currentFolderID,
			Tags:     []string{},
		})
		a.store.AddBookmark(newBookmark)
		a.refreshItems()
		a.mode = ModeNormal
		a.setStatus("Bookmark added: " + title)
		return a, nil

	case ModeEditFolder:
		name := a.titleInput.Value()
		if name == "" {
			// Don't submit with empty name
			return a, nil
		}

		// Find and update the folder
		folder := a.store.GetFolderByID(a.editItemID)
		if folder != nil {
			folder.Name = name
		}
		a.refreshItems()
		a.mode = ModeNormal
		return a, nil

	case ModeEditBookmark:
		title := a.titleInput.Value()
		url := a.urlInput.Value()
		if title == "" || url == "" {
			// Don't submit with empty fields
			return a, nil
		}

		// Find and update the bookmark
		bookmark := a.store.GetBookmarkByID(a.editItemID)
		if bookmark != nil {
			bookmark.Title = title
			bookmark.URL = url
		}
		a.refreshItems()
		a.mode = ModeNormal
		return a, nil

	case ModeEditTags:
		// Parse comma-separated tags
		tagsStr := a.tagsInput.Value()
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
		bookmark := a.store.GetBookmarkByID(a.editItemID)
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
	title := a.titleInput.Value()
	url := a.quickAddInput.Value()

	if title == "" || url == "" {
		return a, nil
	}

	// Parse tags
	var tags []string
	tagsStr := a.tagsInput.Value()
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
	if a.quickAddFolderIdx > 0 && a.quickAddFolderIdx < len(a.quickAddFolders) {
		folderPath := a.quickAddFolders[a.quickAddFolderIdx]
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
	if len(displayItems) == 0 || a.cursor >= len(displayItems) {
		return
	}
	item := displayItems[a.cursor]
	a.yankedItem = &item
	a.setStatus("Yanked: " + item.Title())
}

// cutCurrentItem copies the current item to yank buffer and deletes it.
// Shows confirmation dialog if confirmDelete is enabled.
func (a *App) cutCurrentItem() {
	displayItems := a.getDisplayItems()
	if len(displayItems) == 0 || a.cursor >= len(displayItems) {
		return
	}

	item := displayItems[a.cursor]
	a.cutMode = true

	// Show confirmation if enabled
	if a.confirmDelete {
		if item.IsFolder() {
			a.editItemID = item.Folder.ID
		} else {
			a.editItemID = item.Bookmark.ID
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
	if a.cursor >= len(a.items) && a.cursor > 0 {
		a.cursor--
	}
}

// deleteCurrentItem deletes the current item without copying to yank buffer.
// Shows confirmation dialog if confirmDelete is enabled.
func (a *App) deleteCurrentItem() {
	displayItems := a.getDisplayItems()
	if len(displayItems) == 0 || a.cursor >= len(displayItems) {
		return
	}

	item := displayItems[a.cursor]
	a.cutMode = false

	// Show confirmation if enabled
	if a.confirmDelete {
		if item.IsFolder() {
			a.editItemID = item.Folder.ID
		} else {
			a.editItemID = item.Bookmark.ID
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
	if a.cursor >= len(a.items) && a.cursor > 0 {
		a.cursor--
	}
}

// confirmDeleteItem performs the actual deletion after confirmation.
// Handles both folders and bookmarks.
func (a *App) confirmDeleteItem() {
	var title string

	// Try as folder first
	folder := a.store.GetFolderByID(a.editItemID)
	if folder != nil {
		title = folder.Name
		if a.cutMode {
			// Make a copy before deleting
			folderCopy := *folder
			item := Item{Kind: ItemFolder, Folder: &folderCopy}
			a.yankedItem = &item
		}
		a.store.RemoveFolderByID(a.editItemID)
	} else {
		// Try as bookmark
		bookmark := a.store.GetBookmarkByID(a.editItemID)
		if bookmark == nil {
			return
		}
		title = bookmark.Title
		if a.cutMode {
			// Make a copy before deleting
			bookmarkCopy := *bookmark
			item := Item{Kind: ItemBookmark, Bookmark: &bookmarkCopy}
			a.yankedItem = &item
		}
		a.store.RemoveBookmarkByID(a.editItemID)
	}

	// Refresh items and adjust cursor
	a.refreshItems()
	if a.cursor >= len(a.items) && a.cursor > 0 {
		a.cursor--
	}

	if a.cutMode {
		a.setStatus("Cut: " + title)
	} else {
		a.setStatus("Deleted: " + title)
	}
}

// pasteItem pastes the yanked item before or after the cursor.
func (a *App) pasteItem(before bool) {
	if a.yankedItem == nil {
		a.setStatus("Nothing to paste")
		return
	}

	// Calculate insert position
	insertIdx := a.cursor
	if !before && len(a.items) > 0 {
		insertIdx = a.cursor + 1
	}

	title := a.yankedItem.Title()

	if a.yankedItem.IsFolder() {
		// Create a copy with new ID
		newFolder := model.NewFolder(model.NewFolderParams{
			Name:     a.yankedItem.Folder.Name,
			ParentID: a.currentFolderID,
		})

		// Count folders in current view to find insert position
		folderCount := 0
		for _, item := range a.items {
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
			FolderID: a.currentFolderID,
			Tags:     a.yankedItem.Bookmark.Tags,
		})

		// Count folders to calculate bookmark insert position
		folderCount := 0
		for _, item := range a.items {
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
			_ = openCmd.Start()
		}
		return nil
	}
}

// openBookmark opens the selected bookmark URL in default browser.
func (a App) openBookmark() (tea.Model, tea.Cmd) {
	displayItems := a.getDisplayItems()
	if len(displayItems) == 0 || a.cursor >= len(displayItems) {
		return a, nil
	}

	item := displayItems[a.cursor]
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

// yankURLToClipboard copies the selected bookmark URL to system clipboard.
func (a App) yankURLToClipboard() (tea.Model, tea.Cmd) {
	displayItems := a.getDisplayItems()
	if len(displayItems) == 0 || a.cursor >= len(displayItems) {
		return a, nil
	}

	item := displayItems[a.cursor]
	if item.IsFolder() {
		return a, nil
	}

	url := item.Bookmark.URL
	a.setStatus("URL copied to clipboard")
	cmd := func() tea.Msg {
		_ = clipboard.WriteAll(url)
		return nil
	}

	return a, cmd
}

// View implements tea.Model.
func (a App) View() string {
	return a.renderView()
}

// applyFilter filters current items based on filterQuery using fuzzy matching.
func (a *App) applyFilter() {
	if a.filterQuery == "" {
		a.filteredItems = nil
		return
	}

	// Use fuzzy matching on current folder items
	matches := fuzzy.FindFrom(a.filterQuery, itemStrings(a.items))
	a.filteredItems = make([]Item, len(matches))
	for i, m := range matches {
		a.filteredItems[i] = a.items[m.Index]
	}

	// Reset cursor if out of bounds
	displayItems := a.getDisplayItems()
	if a.cursor >= len(displayItems) {
		a.cursor = 0
	}
}

// getDisplayItems returns filtered items if filter is active, otherwise all items.
func (a *App) getDisplayItems() []Item {
	if a.filterQuery != "" && a.filteredItems != nil {
		return a.filteredItems
	}
	return a.items
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
	for i, p := range a.quickAddFolders {
		if p == path {
			return i
		}
	}
	// Default to root if not found
	return 0
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
