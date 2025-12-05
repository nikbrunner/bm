package tui

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/nikbrunner/bm/internal/model"
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

	app := App{
		store:           params.Store,
		keys:            keys,
		styles:          styles,
		currentFolderID: nil,
		folderStack:     []string{},
		cursor:          0,
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

		// Reset g flag for any other key
		a.lastKeyWasG = false

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
		}
	}

	return a, nil
}

// View implements tea.Model.
func (a App) View() string {
	return a.renderView()
}
