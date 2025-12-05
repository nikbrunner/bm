package tui

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/nikbrunner/bm/internal/model"
)

// Mode represents the current UI mode.
type Mode int

const (
	ModeNormal Mode = iota
	ModeAddBookmark
	ModeAddFolder
)

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

	// For gg command
	lastKeyWasG bool

	// For yy command
	lastKeyWasY bool

	// For dd command
	lastKeyWasD bool

	// Yank buffer
	yankedItem *Item

	// Modal state
	mode       Mode
	titleInput textinput.Model
	urlInput   textinput.Model

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
		width:           80,
		height:          24,
	}

	app.refreshItems()
	return app
}

// refreshItems rebuilds the items slice based on current folder.
func (a *App) refreshItems() {
	a.items = []Item{}

	// Add folders first (sorted before bookmarks)
	folders := a.store.GetFoldersInFolder(a.currentFolderID)
	for i := range folders {
		a.items = append(a.items, Item{
			Kind:   ItemFolder,
			Folder: &folders[i],
		})
	}

	// Add bookmarks
	bookmarks := a.store.GetBookmarksInFolder(a.currentFolderID)
	for i := range bookmarks {
		a.items = append(a.items, Item{
			Kind:     ItemBookmark,
			Bookmark: &bookmarks[i],
		})
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
			a.lastKeyWasY = false
			a.lastKeyWasD = false
			return a, nil
		}

		// Handle yy sequence
		if key.Matches(msg, a.keys.Yank) {
			if a.lastKeyWasY {
				// This is the second y - yank current item
				a.yankCurrentItem()
				a.lastKeyWasY = false
				return a, nil
			}
			// First y - wait for second
			a.lastKeyWasY = true
			a.lastKeyWasG = false
			a.lastKeyWasD = false
			return a, nil
		}

		// Handle dd sequence
		if key.Matches(msg, a.keys.Cut) {
			if a.lastKeyWasD {
				// This is the second d - cut current item
				a.cutCurrentItem()
				a.lastKeyWasD = false
				return a, nil
			}
			// First d - wait for second
			a.lastKeyWasD = true
			a.lastKeyWasG = false
			a.lastKeyWasY = false
			return a, nil
		}

		// Reset sequence flags for any other key
		a.lastKeyWasG = false
		a.lastKeyWasY = false
		a.lastKeyWasD = false

		switch {
		case key.Matches(msg, a.keys.Quit):
			return a, tea.Quit

		case key.Matches(msg, a.keys.Down):
			if len(a.items) > 0 && a.cursor < len(a.items)-1 {
				a.cursor++
			}

		case key.Matches(msg, a.keys.Up):
			if a.cursor > 0 {
				a.cursor--
			}

		case key.Matches(msg, a.keys.Bottom):
			if len(a.items) > 0 {
				a.cursor = len(a.items) - 1
			}

		case key.Matches(msg, a.keys.Right):
			// Enter folder if current item is a folder
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
		}
	}

	return a, nil
}

// updateModal handles key events when in a modal mode.
func (a App) updateModal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
	if a.mode == ModeAddBookmark {
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
	} else if a.mode == ModeAddFolder {
		a.titleInput, cmd = a.titleInput.Update(msg)
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
}

// cutCurrentItem copies the current item to yank buffer and deletes it.
func (a *App) cutCurrentItem() {
	if len(a.items) == 0 || a.cursor >= len(a.items) {
		return
	}

	item := a.items[a.cursor]
	a.yankedItem = &item

	// Delete from store
	if item.IsFolder() {
		a.store.RemoveFolderByID(item.Folder.ID)
	} else {
		a.store.RemoveBookmarkByID(item.Bookmark.ID)
	}

	// Refresh items and adjust cursor
	a.refreshItems()
	if a.cursor >= len(a.items) && a.cursor > 0 {
		a.cursor--
	}
}

// pasteItem pastes the yanked item before or after the cursor.
func (a *App) pasteItem(before bool) {
	if a.yankedItem == nil {
		return
	}

	// Calculate insert position
	insertIdx := a.cursor
	if !before && len(a.items) > 0 {
		insertIdx = a.cursor + 1
	}

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
}

// View implements tea.Model.
func (a App) View() string {
	return a.renderView()
}
