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
	"github.com/nikbrunner/bm/internal/model"
	"github.com/sahilm/fuzzy"
)

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
	ModeHelp
)

// SortMode represents the current sort mode.
type SortMode int

const (
	SortManual  SortMode = iota // preserve insertion order
	SortAlpha                   // alphabetical
	SortCreated                 // by creation date (newest first)
	SortVisited                 // by visit date (most recent first)
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

	// Navigation state
	currentFolderID *string  // nil = root
	folderStack     []string // breadcrumb trail of folder IDs
	cursor          int      // selected item index
	items           []Item   // current list items

	// Sort mode
	sortMode SortMode

	// Filter/search
	filterQuery   string
	searchInput   textinput.Model
	fuzzyMatches  []fuzzyMatch    // Current fuzzy match results
	fuzzyCursor   int             // Selected index in fuzzy results
	allItems      []Item          // All items in current folder (unfiltered)

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
	searchInput.Placeholder = "Filter..."
	searchInput.CharLimit = 100
	searchInput.Width = 40

	app := App{
		store:           params.Store,
		keys:            keys,
		styles:          styles,
		currentFolderID: nil,
		folderStack:     []string{},
		cursor:          0,
		mode:            ModeNormal,
		titleInput:      titleInput,
		urlInput:        urlInput,
		tagsInput:       tagsInput,
		searchInput:     searchInput,
		width:           80,
		height:          24,
	}

	app.refreshItems()
	return app
}

// refreshItems rebuilds the items slice based on current folder and sort mode.
func (a *App) refreshItems() {
	a.items = []Item{}

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

	case tea.KeyMsg:
		// Handle modal modes first
		if a.mode != ModeNormal {
			return a.updateModal(msg)
		}

		// Handle gg sequence
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

		// Reset sequence flags for any other key
		a.lastKeyWasG = false

		switch {
		case key.Matches(msg, a.keys.Quit):
			return a, tea.Quit

		case key.Matches(msg, a.keys.Down):
			if len(a.items) > 0 && a.cursor < len(a.items)-1 {
				a.cursor++
			}
			a.statusMessage = "" // Clear status on navigation

		case key.Matches(msg, a.keys.Up):
			if a.cursor > 0 {
				a.cursor--
			}
			a.statusMessage = "" // Clear status on navigation

		case key.Matches(msg, a.keys.Bottom):
			if len(a.items) > 0 {
				a.cursor = len(a.items) - 1
			}

		case key.Matches(msg, a.keys.Right):
			// Enter folder or open bookmark
			if len(a.items) > 0 && a.cursor < len(a.items) {
				item := a.items[a.cursor]
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
			// Go back to parent folder
			if a.currentFolderID != nil {
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
			}

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

		case key.Matches(msg, a.keys.Edit):
			// Only edit if there's an item selected
			if len(a.items) == 0 || a.cursor >= len(a.items) {
				return a, nil
			}
			item := a.items[a.cursor]
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
			if len(a.items) == 0 || a.cursor >= len(a.items) {
				return a, nil
			}
			item := a.items[a.cursor]
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

		case key.Matches(msg, a.keys.Open):
			// Open bookmark URL in browser
			return a.openBookmark()

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
			a.confirmDeleteFolder()
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

// yankCurrentItem copies the current item to the yank buffer.
func (a *App) yankCurrentItem() {
	if len(a.items) == 0 || a.cursor >= len(a.items) {
		return
	}
	item := a.items[a.cursor]
	a.yankedItem = &item
	a.setStatus("Yanked: " + item.Title())
}

// cutCurrentItem copies the current item to yank buffer and deletes it.
// For folders, it shows a confirmation dialog instead of deleting immediately.
func (a *App) cutCurrentItem() {
	if len(a.items) == 0 || a.cursor >= len(a.items) {
		return
	}

	item := a.items[a.cursor]

	// For folders, show confirmation dialog
	if item.IsFolder() {
		a.editItemID = item.Folder.ID
		a.cutMode = true
		a.mode = ModeConfirmDelete
		return
	}

	// For bookmarks, cut immediately (delete + buffer)
	title := item.Bookmark.Title
	a.yankedItem = &item
	a.store.RemoveBookmarkByID(item.Bookmark.ID)

	// Refresh items and adjust cursor
	a.refreshItems()
	if a.cursor >= len(a.items) && a.cursor > 0 {
		a.cursor--
	}
	a.setStatus("Cut: " + title)
}

// deleteCurrentItem deletes the current item without copying to yank buffer.
// For folders, it shows a confirmation dialog instead of deleting immediately.
func (a *App) deleteCurrentItem() {
	if len(a.items) == 0 || a.cursor >= len(a.items) {
		return
	}

	item := a.items[a.cursor]

	// For folders, show confirmation dialog
	if item.IsFolder() {
		a.editItemID = item.Folder.ID
		a.cutMode = false
		a.mode = ModeConfirmDelete
		return
	}

	// For bookmarks, delete immediately (no buffer)
	title := item.Bookmark.Title
	a.store.RemoveBookmarkByID(item.Bookmark.ID)

	// Refresh items and adjust cursor
	a.refreshItems()
	if a.cursor >= len(a.items) && a.cursor > 0 {
		a.cursor--
	}
	a.setStatus("Deleted: " + title)
}

// confirmDeleteFolder performs the actual folder deletion after confirmation.
func (a *App) confirmDeleteFolder() {
	folder := a.store.GetFolderByID(a.editItemID)
	if folder == nil {
		return
	}

	title := folder.Name

	// Only store in yank buffer if this was a cut operation
	if a.cutMode {
		item := Item{Kind: ItemFolder, Folder: folder}
		a.yankedItem = &item
	}

	// Delete the folder
	a.store.RemoveFolderByID(a.editItemID)

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

// openBookmark opens the selected bookmark URL in default browser.
func (a App) openBookmark() (tea.Model, tea.Cmd) {
	if len(a.items) == 0 || a.cursor >= len(a.items) {
		return a, nil
	}

	item := a.items[a.cursor]
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

	// Open URL in default browser
	url := item.Bookmark.URL
	cmd := func() tea.Msg {
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

	return a, cmd
}

// yankURLToClipboard copies the selected bookmark URL to system clipboard.
func (a App) yankURLToClipboard() (tea.Model, tea.Cmd) {
	if len(a.items) == 0 || a.cursor >= len(a.items) {
		return a, nil
	}

	item := a.items[a.cursor]
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
